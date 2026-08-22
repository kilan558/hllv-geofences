package maprules_test

import (
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func TestQuangNgaiUsesCarentanForVietnamTeams(t *testing.T) {
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
			// NVA/Axis starts on J and receives the classic Carentan Axis side.
			axisAllowed: []rconv2.Grid{{X: "J", Y: 3}, {X: "J", Y: 8}, {X: "E", Y: 5}, {X: "A", Y: 3}, {X: "G", Y: 2}},
			axisBlocked: []rconv2.Grid{{X: "D", Y: 3}, {X: "B", Y: 8}, {X: "J", Y: 10}},
			// USA/Allies starts on A and receives the classic Carentan Allies side.
			alliesAllowed: []rconv2.Grid{{X: "A", Y: 3}, {X: "A", Y: 8}, {X: "F", Y: 5}, {X: "J", Y: 3}, {X: "D", Y: 9}},
			alliesBlocked: []rconv2.Grid{{X: "G", Y: 3}, {X: "I", Y: 8}, {X: "A", Y: 1}},
		},
		{
			name:          "advanced midcap below 100 players",
			variant:       maprules.VariantMidcapAdvanced,
			playerCount:   99,
			axisCount:     62,
			alliesCount:   62,
			axisAllowed:   []rconv2.Grid{{X: "J", Y: 3}, {X: "E", Y: 8}, {X: "A", Y: 5}},
			axisBlocked:   []rconv2.Grid{{X: "D", Y: 3}},
			alliesAllowed: []rconv2.Grid{{X: "A", Y: 3}, {X: "F", Y: 8}, {X: "J", Y: 5}},
			alliesBlocked: []rconv2.Grid{{X: "G", Y: 3}},
		},
		{
			name:          "lastcap through 90 players",
			variant:       maprules.VariantLastcap,
			playerCount:   90,
			axisCount:     64,
			alliesCount:   48,
			axisAllowed:   []rconv2.Grid{{X: "J", Y: 3}, {X: "J", Y: 8}, {X: "C", Y: 5}, {X: "G", Y: 2}},
			axisBlocked:   []rconv2.Grid{{X: "B", Y: 3}, {X: "A", Y: 8}, {X: "J", Y: 10}},
			alliesAllowed: []rconv2.Grid{{X: "A", Y: 3}, {X: "A", Y: 8}, {X: "H", Y: 5}},
			alliesBlocked: []rconv2.Grid{{X: "I", Y: 3}, {X: "J", Y: 8}, {X: "A", Y: 2}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := data.Config{Servers: []data.Server{{
				Host:        "192.0.2.1",
				Port:        7779,
				Password:    "unchanged-secret",
				AxisFence:   []data.Fence{quangOtherMapFence()},
				AlliesFence: []data.Fence{quangOtherMapFence()},
			}}}

			if err := maprules.ApplyQuangNgai(&config, tc.variant); err != nil {
				t.Fatalf("ApplyQuangNgai() error = %v", err)
			}
			server := config.Servers[0]
			if server.Host != "192.0.2.1" || server.Port != 7779 || server.Password != "unchanged-secret" {
				t.Fatalf("ApplyQuangNgai() changed server credentials: %+v", server)
			}
			if got := quangMatchingCount(server.AxisFence, quangNgaiSession(tc.playerCount)); got != tc.axisCount {
				t.Fatalf("matching Axis/NVA fence count = %d, want %d", got, tc.axisCount)
			}
			if got := quangMatchingCount(server.AlliesFence, quangNgaiSession(tc.playerCount)); got != tc.alliesCount {
				t.Fatalf("matching Allies/USA fence count = %d, want %d", got, tc.alliesCount)
			}

			assertQuangAllowed(t, server.AxisFence, tc.playerCount, tc.axisAllowed, true)
			assertQuangAllowed(t, server.AxisFence, tc.playerCount, tc.axisBlocked, false)
			assertQuangAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesAllowed, true)
			assertQuangAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesBlocked, false)
		})
	}
}

func TestQuangNgaiRulesOnlyMatchWarfareAliasesAndConfiguredPlayerRange(t *testing.T) {
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyQuangNgai(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("ApplyQuangNgai() error = %v", err)
	}
	fences := config.Servers[0].AxisFence

	for _, mapName := range []string{"wdevb_warfare_day", "wdevb", "Quảng Ngãi", "Quang Ngai", "QUẢNG NGÃI"} {
		session := quangNgaiSession(59)
		session.MapName = mapName
		if got := quangMatchingCount(fences, session); got != 62 {
			t.Fatalf("map alias %q matched %d Quảng Ngãi fences, want 62", mapName, got)
		}
	}

	fullServer := quangNgaiSession(61)
	if got := quangMatchingCount(fences, fullServer); got != 0 {
		t.Fatalf("fences at the 61-player cutoff = %d, want 0", got)
	}
	wrongMode := quangNgaiSession(59)
	wrongMode.GameMode = rconv2.GetSessionInfoGameModeConquest
	if got := quangMatchingCount(fences, wrongMode); got != 0 {
		t.Fatalf("fences in Conquest = %d, want 0", got)
	}
	wrongMap := quangNgaiSession(59)
	wrongMap.MapName = "wdeva_warfare_day"
	if got := quangMatchingCount(fences, wrongMap); got != 0 {
		t.Fatalf("fences on another map = %d, want 0", got)
	}
}

func TestApplyQuangNgaiIsIdempotentAndPreservesOtherMapRules(t *testing.T) {
	config := data.Config{Servers: []data.Server{{
		Host:        "192.0.2.1",
		Port:        7779,
		Password:    "secret",
		AxisFence:   []data.Fence{quangOtherMapFence()},
		AlliesFence: []data.Fence{quangOtherMapFence()},
	}}}

	if err := maprules.ApplyQuangNgai(&config, maprules.VariantLastcap); err != nil {
		t.Fatalf("first ApplyQuangNgai() error = %v", err)
	}
	if err := maprules.ApplyQuangNgai(&config, maprules.VariantLastcap); err != nil {
		t.Fatalf("second ApplyQuangNgai() error = %v", err)
	}

	server := config.Servers[0]
	if got := quangMatchingCount(server.AxisFence, quangNgaiSession(90)); got != 64 {
		t.Fatalf("idempotent Axis/NVA fence count = %d, want 64", got)
	}
	other := quangNgaiSession(90)
	other.MapName = "wdeva_warfare_day"
	if got := quangMatchingCount(server.AxisFence, other); got != 1 {
		t.Fatalf("preserved other-map Axis fence count = %d, want 1", got)
	}
	if got := quangMatchingCount(server.AlliesFence, other); got != 1 {
		t.Fatalf("preserved other-map Allies fence count = %d, want 1", got)
	}
}

func TestApplyQuangNgaiRejectsInvalidInput(t *testing.T) {
	if err := maprules.ApplyQuangNgai(nil, maprules.VariantMidcap); err == nil {
		t.Fatal("ApplyQuangNgai(nil) error = nil, want validation error")
	}
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyQuangNgai(&config, maprules.Variant("invalid")); err == nil {
		t.Fatal("ApplyQuangNgai() error = nil, want unknown variant error")
	}
}

func quangNgaiSession(playerCount int) *rconv2.GetSessionInfoResponse {
	return &rconv2.GetSessionInfoResponse{
		MapName:     "wdevb_warfare_day",
		GameMode:    rconv2.GetSessionInfoGameModeWarfare,
		PlayerCount: playerCount,
	}
}

func quangOtherMapFence() data.Fence {
	x, y := "A", 3
	return data.Fence{
		X: &x,
		Y: &y,
		Condition: &data.Condition{Equals: map[string][]string{
			"map_name":  {"wdeva_warfare_day"},
			"game_mode": {"Warfare"},
		}},
	}
}

func quangMatchingCount(fences []data.Fence, session *rconv2.GetSessionInfoResponse) int {
	count := 0
	for _, fence := range fences {
		if fence.Matches(session) {
			count++
		}
	}
	return count
}

func assertQuangAllowed(t *testing.T, fences []data.Fence, playerCount int, grids []rconv2.Grid, want bool) {
	t.Helper()
	for _, grid := range grids {
		got := false
		for _, fence := range fences {
			if fence.Matches(quangNgaiSession(playerCount)) && fence.Includes(grid) {
				got = true
				break
			}
		}
		if got != want {
			t.Errorf("grid %s%d allowed = %t, want %t", grid.X, grid.Y, got, want)
		}
	}
}
