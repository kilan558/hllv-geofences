package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
)

func TestSessionStatusLogIsBoundedAndContainsDiscordPanelFields(t *testing.T) {
	var output bytes.Buffer
	w := &Worker{
		l: slog.New(slog.NewTextHandler(&output, nil)),
		current: &rconv2.GetSessionInfoResponse{
			ServerName:     "Vietnam Test Server",
			MapName:        "wdeva_warfare_day",
			GameMode:       rconv2.GetSessionInfoGameModeWarfare,
			PlayerCount:    42,
			MaxPlayerCount: 100,
			QueueCount:     2,
			MaxQueueCount:  6,
		},
	}
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	w.logSessionStatus(now, false)
	w.logSessionStatus(now.Add(time.Second), false)

	line := output.String()
	if got := strings.Count(line, "msg=session-status"); got != 1 {
		t.Fatalf("session status log count = %d, want 1; output=%q", got, line)
	}
	for _, want := range []string{
		`server_name="Vietnam Test Server"`,
		"map=wdeva_warfare_day",
		"mode=Warfare",
		"player_count=42",
		"max_player_count=100",
		"queue_count=2",
		"max_queue_count=6",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("session log %q does not contain %q", line, want)
		}
	}

	w.logSessionStatus(now.Add(time.Second), true)
	if got := strings.Count(output.String(), "msg=session-status"); got != 2 {
		t.Fatalf("session status log count after forced refresh = %d, want 2", got)
	}
}

func TestGeofenceEventIncludesPlayerAndSessionContext(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	w := &Worker{
		l: logger,
		current: &rconv2.GetSessionInfoResponse{
			MapName:  "Cam Ranh Port",
			GameMode: rconv2.GetSessionInfoGameModeWarfare,
		},
	}

	w.logPlayerEvent(
		context.Background(),
		slog.LevelInfo,
		"player-outside-fence",
		"NinjaimMüsli",
		"player-123",
		rconv2.Grid{X: "J", Y: 8},
		nil,
	)

	line := output.String()
	for _, want := range []string{
		"level=INFO",
		"msg=player-outside-fence",
		`player=NinjaimMüsli`,
		"player_id=player-123",
		`grid="J8 Numpad 0"`,
		`map="Cam Ranh Port"`,
		"mode=Warfare",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("warning log %q does not contain %q", line, want)
		}
	}
}

func TestHandlePollErrorRestartsOnExhaustedReconnects(t *testing.T) {
	var output bytes.Buffer
	w := &Worker{
		l:         slog.New(slog.NewTextHandler(&output, nil)),
		restartCh: make(chan struct{}, 1),
	}

	if !w.handlePollError("poll-session", rconv2.ReconnectTriesExceeded) {
		t.Fatal("handlePollError() stop = false, want true")
	}
	select {
	case <-w.RestartSignal():
	default:
		t.Fatal("expected a restart signal after reconnects are exhausted")
	}
	if !strings.Contains(output.String(), "request-restart") {
		t.Fatalf("log %q does not request a restart", output.String())
	}
}

func TestHandlePollErrorDoesNotRestartOnFirstTimeout(t *testing.T) {
	w := &Worker{
		l:         slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		restartCh: make(chan struct{}, 1),
	}
	if w.handlePollError("poll-session", errors.New("i/o timeout")) {
		t.Fatal("first timeout should not stop the poller")
	}
	select {
	case <-w.RestartSignal():
		t.Fatal("first timeout should not request a restart")
	default:
	}
}

func TestGeofenceEventKeepsErrorsAtErrorLevel(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	w := &Worker{l: logger}

	w.logPlayerEvent(
		context.Background(),
		slog.LevelError,
		"punish-player",
		"Test Player",
		"player-456",
		rconv2.Grid{X: "A", Y: 3},
		errors.New("rcon unavailable"),
	)

	line := output.String()
	for _, want := range []string{
		"level=ERROR",
		"msg=punish-player",
		`player="Test Player"`,
		`error="rcon unavailable"`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("error log %q does not contain %q", line, want)
		}
	}
}

func TestHLLVPlayerListDecodesFractionalWorldCoordinates(t *testing.T) {
	payload := []byte(`{
		"players": [{
			"iD": "test-player",
			"platform": "steam",
			"name": "NinjaimMüsli",
			"team": 1,
			"worldPosition": {
				"x": 69257.23159032181,
				"y": 9122.282278125398,
				"z": -836.5281383312487
			}
		}]
	}`)

	var response rconv2.GetPlayersResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("decoding HLL:V GetPlayers response failed: %v", err)
	}
	if len(response.Players) != 1 {
		t.Fatalf("decoded players = %d, want 1", len(response.Players))
	}
	position := response.Players[0].Position
	if math.Abs(float64(position.X)-69257.23159032181) > 0.000001 ||
		math.Abs(float64(position.Y)-9122.282278125398) > 0.000001 ||
		math.Abs(float64(position.Z)-(-836.5281383312487)) > 0.000001 {
		t.Fatalf("decoded position = (%v,%v,%v), want the fractional RCON coordinates", position.X, position.Y, position.Z)
	}
}

func TestHLLVTeamSide(t *testing.T) {
	tests := []struct {
		name string
		team rconv2.GetPlayersTeam
		want teamSide
	}{
		{name: "United States", team: 1, want: teamAllies},
		{name: "NVA", team: 6, want: teamAxis},
		{name: "unknown", team: 99, want: teamUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hllvTeamSide(tc.team); got != tc.want {
				t.Fatalf("hllvTeamSide(%d) = %d, want %d", tc.team, got, tc.want)
			}
		})
	}
}

func TestGridForPositionSupportsEveryHLLVMode(t *testing.T) {
	modes := []rconv2.GetSessionInfoGameMode{
		rconv2.GetSessionInfoGameModeWarfare,
		rconv2.GetSessionInfoGameModeOffensive,
		rconv2.GetSessionInfoGameModeConquest,
		rconv2.GetSessionInfoGameMode("Domination"),
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			got, err := gridForPosition(
				rconv2.GetPlayerPosition{X: 1, Y: 1, Z: 1},
				data.GridGeometry{},
			)
			if err != nil {
				t.Fatalf("gridForPosition(%s) returned error: %v", mode, err)
			}
			if got.X != "F" || got.Y != 6 || got.Numpad != 7 {
				t.Fatalf("gridForPosition(%s) = %+v, want F6 Numpad 7", mode, got)
			}
		})
	}
}

func TestGridForPositionRejectsCoordinatesOutsideTenByTenGrid(t *testing.T) {
	_, err := gridForPosition(
		rconv2.GetPlayerPosition{X: 200000, Y: 0, Z: 1},
		data.GridGeometry{},
	)
	if err == nil {
		t.Fatal("gridForPosition() error = nil, want out-of-bounds error")
	}
}
