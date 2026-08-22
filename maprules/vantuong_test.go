package maprules_test

import (
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func TestVanTuongMirrorsCarentanForVietnamTeams(t *testing.T) {
	tests := []struct {
		name          string
		variant       maprules.Variant
		playerCount   int
		axisCount     int
		alliesCount   int
		axisAllowed   []rconv2.Grid
		axisBlocked   []rconv2.Grid
		alliesAllowed []rconv2.Grid
		alliesBlocked []rconv2.Grid
	}{
		{
			name:        "midcap through 60 players",
			variant:     maprules.VariantMidcap,
			playerCount: 60,
			axisCount:   62,
			alliesCount: 62,
			// NVA/Axis starts on A and therefore receives the old Carentan Allies side.
			axisAllowed: []rconv2.Grid{{X: "A", Y: 3}, {X: "A", Y: 8}, {X: "F", Y: 5}, {X: "J", Y: 3}, {X: "G", Y: 2}},
			axisBlocked: []rconv2.Grid{{X: "G", Y: 3}, {X: "I", Y: 8}, {X: "A", Y: 1}},
			// US/Allies starts on J and therefore receives the old Carentan Axis side.
			alliesAllowed: []rconv2.Grid{{X: "J", Y: 3}, {X: "J", Y: 8}, {X: "E", Y: 5}, {X: "A", Y: 3}, {X: "D", Y: 9}},
			alliesBlocked: []rconv2.Grid{{X: "D", Y: 3}, {X: "B", Y: 8}, {X: "J", Y: 10}},
		},
		{
			name:          "advanced midcap below 100 players",
			variant:       maprules.VariantMidcapAdvanced,
			playerCount:   99,
			axisCount:     62,
			alliesCount:   62,
			axisAllowed:   []rconv2.Grid{{X: "A", Y: 3}, {X: "F", Y: 8}, {X: "J", Y: 5}},
			axisBlocked:   []rconv2.Grid{{X: "G", Y: 3}},
			alliesAllowed: []rconv2.Grid{{X: "J", Y: 3}, {X: "E", Y: 8}, {X: "A", Y: 5}},
			alliesBlocked: []rconv2.Grid{{X: "D", Y: 3}},
		},
		{
			name:          "lastcap through 90 players",
			variant:       maprules.VariantLastcap,
			playerCount:   90,
			axisCount:     48,
			alliesCount:   64,
			axisAllowed:   []rconv2.Grid{{X: "A", Y: 3}, {X: "A", Y: 8}, {X: "H", Y: 5}},
			axisBlocked:   []rconv2.Grid{{X: "I", Y: 3}, {X: "J", Y: 8}, {X: "A", Y: 2}},
			alliesAllowed: []rconv2.Grid{{X: "J", Y: 3}, {X: "J", Y: 8}, {X: "C", Y: 5}, {X: "G", Y: 2}},
			alliesBlocked: []rconv2.Grid{{X: "B", Y: 3}, {X: "A", Y: 8}, {X: "J", Y: 10}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := data.Config{Servers: []data.Server{{
				Host:        "192.0.2.1",
				Port:        7779,
				Password:    "unchanged-secret",
				AxisFence:   []data.Fence{otherMapFence()},
				AlliesFence: []data.Fence{otherMapFence()},
			}}}

			if err := maprules.ApplyVanTuong(&config, tc.variant); err != nil {
				t.Fatalf("ApplyVanTuong() error = %v", err)
			}
			server := config.Servers[0]
			if server.Host != "192.0.2.1" || server.Port != 7779 || server.Password != "unchanged-secret" {
				t.Fatalf("ApplyVanTuong() changed server credentials: %+v", server)
			}
			if got := matchingCount(server.AxisFence, vanTuongSession(tc.playerCount)); got != tc.axisCount {
				t.Fatalf("matching Axis/NVA fence count = %d, want %d", got, tc.axisCount)
			}
			if got := matchingCount(server.AlliesFence, vanTuongSession(tc.playerCount)); got != tc.alliesCount {
				t.Fatalf("matching Allies/US fence count = %d, want %d", got, tc.alliesCount)
			}

			assertAllowed(t, server.AxisFence, tc.playerCount, tc.axisAllowed, true)
			assertAllowed(t, server.AxisFence, tc.playerCount, tc.axisBlocked, false)
			assertAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesAllowed, true)
			assertAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesBlocked, false)
		})
	}
}

func TestVanTuongRulesOnlyMatchTheWarfareLayerAndConfiguredPlayerRange(t *testing.T) {
	config := data.Config{Servers: []data.Server{{
		Host: "192.0.2.1", Port: 7779, Password: "secret",
	}}}
	if err := maprules.ApplyVanTuong(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("ApplyVanTuong() error = %v", err)
	}
	fences := config.Servers[0].AlliesFence

	for _, mapName := range []string{"wdeva_warfare_day", "wdeva", "Vạn Tường"} {
		session := vanTuongSession(59)
		session.MapName = mapName
		if got := matchingCount(fences, session); got == 0 {
			t.Fatalf("map alias %q did not activate Vạn Tường fences", mapName)
		}
	}

	fullServer := vanTuongSession(61)
	if got := matchingCount(fences, fullServer); got != 0 {
		t.Fatalf("fences at the 61-player cutoff = %d, want 0", got)
	}
	wrongMode := vanTuongSession(59)
	wrongMode.GameMode = rconv2.GetSessionInfoGameModeConquest
	if got := matchingCount(fences, wrongMode); got != 0 {
		t.Fatalf("fences in Conquest = %d, want 0", got)
	}
	wrongMap := vanTuongSession(59)
	wrongMap.MapName = "wdevb_warfare_day"
	if got := matchingCount(fences, wrongMap); got != 0 {
		t.Fatalf("fences on another map = %d, want 0", got)
	}
}

func TestApplyVanTuongIsIdempotentAndPreservesOtherMapRules(t *testing.T) {
	config := data.Config{Servers: []data.Server{{
		Host:        "192.0.2.1",
		Port:        7779,
		Password:    "secret",
		AxisFence:   []data.Fence{otherMapFence()},
		AlliesFence: []data.Fence{otherMapFence()},
	}}}

	if err := maprules.ApplyVanTuong(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("first ApplyVanTuong() error = %v", err)
	}
	if err := maprules.ApplyVanTuong(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("second ApplyVanTuong() error = %v", err)
	}

	server := config.Servers[0]
	if got := matchingCount(server.AxisFence, vanTuongSession(59)); got != 62 {
		t.Fatalf("idempotent Axis/NVA fence count = %d, want 62", got)
	}
	other := vanTuongSession(59)
	other.MapName = "wdevb_warfare_day"
	if got := matchingCount(server.AxisFence, other); got != 1 {
		t.Fatalf("preserved other-map Axis fence count = %d, want 1", got)
	}
	if got := matchingCount(server.AlliesFence, other); got != 1 {
		t.Fatalf("preserved other-map Allies fence count = %d, want 1", got)
	}
}

func TestApplyVanTuongRejectsUnknownVariant(t *testing.T) {
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyVanTuong(&config, maprules.Variant("invalid")); err == nil {
		t.Fatal("ApplyVanTuong() error = nil, want unknown variant error")
	}
}

func vanTuongSession(playerCount int) *rconv2.GetSessionInfoResponse {
	return &rconv2.GetSessionInfoResponse{
		MapName:     "wdeva_warfare_day",
		GameMode:    rconv2.GetSessionInfoGameModeWarfare,
		PlayerCount: playerCount,
	}
}

func otherMapFence() data.Fence {
	x, y := "A", 3
	return data.Fence{
		X: &x,
		Y: &y,
		Condition: &data.Condition{Equals: map[string][]string{
			"map_name":  {"wdevb_warfare_day"},
			"game_mode": {"Warfare"},
		}},
	}
}

func matchingCount(fences []data.Fence, session *rconv2.GetSessionInfoResponse) int {
	count := 0
	for _, fence := range fences {
		if fence.Matches(session) {
			count++
		}
	}
	return count
}

func assertAllowed(t *testing.T, fences []data.Fence, playerCount int, grids []rconv2.Grid, want bool) {
	t.Helper()
	for _, grid := range grids {
		got := false
		for _, fence := range fences {
			if fence.Matches(vanTuongSession(playerCount)) && fence.Includes(grid) {
				got = true
				break
			}
		}
		if got != want {
			t.Errorf("grid %s%d allowed = %t, want %t", grid.X, grid.Y, got, want)
		}
	}
}
