package maprules_test

import (
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func TestHueOutskirtsUsesSMDMForVietnamTeams(t *testing.T) {
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
			name:          "midcap through 60 players",
			variant:       maprules.VariantMidcap,
			playerCount:   60,
			axisCount:     62,
			alliesCount:   62,
			axisAllowed:   []rconv2.Grid{{X: "B", Y: 1}, {X: "I", Y: 10}, {X: "C", Y: 1}, {X: "H", Y: 6}, {X: "F", Y: 10}},
			axisBlocked:   []rconv2.Grid{{X: "C", Y: 7}, {X: "H", Y: 9}, {X: "A", Y: 5}, {X: "J", Y: 5}},
			alliesAllowed: []rconv2.Grid{{X: "B", Y: 1}, {X: "I", Y: 10}, {X: "C", Y: 1}, {X: "H", Y: 5}, {X: "F", Y: 10}},
			alliesBlocked: []rconv2.Grid{{X: "C", Y: 2}, {X: "H", Y: 4}, {X: "A", Y: 5}, {X: "J", Y: 5}},
		},
		{
			name:          "advanced midcap below 100 players",
			variant:       maprules.VariantMidcapAdvanced,
			playerCount:   99,
			axisCount:     62,
			alliesCount:   62,
			axisAllowed:   []rconv2.Grid{{X: "C", Y: 1}, {X: "G", Y: 6}, {X: "H", Y: 10}},
			axisBlocked:   []rconv2.Grid{{X: "C", Y: 7}, {X: "G", Y: 9}},
			alliesAllowed: []rconv2.Grid{{X: "C", Y: 1}, {X: "G", Y: 5}, {X: "H", Y: 10}},
			alliesBlocked: []rconv2.Grid{{X: "C", Y: 2}, {X: "G", Y: 4}},
		},
		{
			name:          "lastcap through 90 players",
			variant:       maprules.VariantLastcap,
			playerCount:   90,
			axisCount:     68,
			alliesCount:   68,
			axisAllowed:   []rconv2.Grid{{X: "B", Y: 10}, {X: "C", Y: 1}, {X: "H", Y: 8}},
			axisBlocked:   []rconv2.Grid{{X: "C", Y: 9}, {X: "H", Y: 10}},
			alliesAllowed: []rconv2.Grid{{X: "I", Y: 1}, {X: "C", Y: 3}, {X: "H", Y: 10}},
			alliesBlocked: []rconv2.Grid{{X: "C", Y: 1}, {X: "H", Y: 2}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := data.Config{Servers: []data.Server{{
				Host:        "192.0.2.1",
				Port:        7779,
				Password:    "unchanged-secret",
				AxisFence:   []data.Fence{hueOtherMapFence()},
				AlliesFence: []data.Fence{hueOtherMapFence()},
			}}}

			if err := maprules.ApplyHueOutskirts(&config, tc.variant); err != nil {
				t.Fatalf("ApplyHueOutskirts() error = %v", err)
			}
			server := config.Servers[0]
			if server.Host != "192.0.2.1" || server.Port != 7779 || server.Password != "unchanged-secret" {
				t.Fatalf("ApplyHueOutskirts() changed server credentials: %+v", server)
			}
			if got := hueMatchingCount(server.AxisFence, hueOutskirtsSession(tc.playerCount)); got != tc.axisCount {
				t.Fatalf("matching Axis/NVA fence count = %d, want %d", got, tc.axisCount)
			}
			if got := hueMatchingCount(server.AlliesFence, hueOutskirtsSession(tc.playerCount)); got != tc.alliesCount {
				t.Fatalf("matching Allies/USA fence count = %d, want %d", got, tc.alliesCount)
			}

			assertHueAllowed(t, server.AxisFence, tc.playerCount, tc.axisAllowed, true)
			assertHueAllowed(t, server.AxisFence, tc.playerCount, tc.axisBlocked, false)
			assertHueAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesAllowed, true)
			assertHueAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesBlocked, false)
		})
	}
}

func TestHueOutskirtsRulesOnlyMatchWarfareAliasesAndConfiguredPlayerRange(t *testing.T) {
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyHueOutskirts(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("ApplyHueOutskirts() error = %v", err)
	}
	fences := config.Servers[0].AxisFence

	for _, mapName := range []string{"wdevc_warfare_day", "wdevc", "Huế Outskirts", "Hue Outskirts", "HUẾ OUTSKIRTS"} {
		session := hueOutskirtsSession(59)
		session.MapName = mapName
		if got := hueMatchingCount(fences, session); got != 62 {
			t.Fatalf("map alias %q matching fences = %d, want 62", mapName, got)
		}
	}

	fullServer := hueOutskirtsSession(61)
	if got := hueMatchingCount(fences, fullServer); got != 0 {
		t.Fatalf("fences at the 61-player cutoff = %d, want 0", got)
	}
	wrongMode := hueOutskirtsSession(59)
	wrongMode.GameMode = rconv2.GetSessionInfoGameModeConquest
	if got := hueMatchingCount(fences, wrongMode); got != 0 {
		t.Fatalf("fences in Conquest = %d, want 0", got)
	}
	wrongMap := hueOutskirtsSession(59)
	wrongMap.MapName = "wdevf_warfare_day"
	if got := hueMatchingCount(fences, wrongMap); got != 0 {
		t.Fatalf("fences on another map = %d, want 0", got)
	}
}

func TestApplyHueOutskirtsIsIdempotentAndPreservesOtherMapRules(t *testing.T) {
	config := data.Config{Servers: []data.Server{{
		Host:        "192.0.2.1",
		Port:        7779,
		Password:    "secret",
		AxisFence:   []data.Fence{hueOtherMapFence()},
		AlliesFence: []data.Fence{hueOtherMapFence()},
	}}}

	if err := maprules.ApplyHueOutskirts(&config, maprules.VariantLastcap); err != nil {
		t.Fatalf("first ApplyHueOutskirts() error = %v", err)
	}
	if err := maprules.ApplyHueOutskirts(&config, maprules.VariantLastcap); err != nil {
		t.Fatalf("second ApplyHueOutskirts() error = %v", err)
	}

	server := config.Servers[0]
	if got := hueMatchingCount(server.AxisFence, hueOutskirtsSession(90)); got != 68 {
		t.Fatalf("idempotent Axis/NVA fence count = %d, want 68", got)
	}
	other := hueOutskirtsSession(90)
	other.MapName = "wdeva_warfare_day"
	if got := hueMatchingCount(server.AxisFence, other); got != 1 {
		t.Fatalf("preserved other-map Axis fence count = %d, want 1", got)
	}
	if got := hueMatchingCount(server.AlliesFence, other); got != 1 {
		t.Fatalf("preserved other-map Allies fence count = %d, want 1", got)
	}
}

func TestApplyHueOutskirtsRejectsInvalidInput(t *testing.T) {
	if err := maprules.ApplyHueOutskirts(nil, maprules.VariantMidcap); err == nil {
		t.Fatal("ApplyHueOutskirts(nil) error = nil, want validation error")
	}
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyHueOutskirts(&config, maprules.Variant("invalid")); err == nil {
		t.Fatal("ApplyHueOutskirts() error = nil, want unknown variant error")
	}
}

func hueOutskirtsSession(playerCount int) *rconv2.GetSessionInfoResponse {
	return &rconv2.GetSessionInfoResponse{
		MapName:     "wdevc_warfare_day",
		GameMode:    rconv2.GetSessionInfoGameModeWarfare,
		PlayerCount: playerCount,
	}
}

func hueOtherMapFence() data.Fence {
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

func hueMatchingCount(fences []data.Fence, session *rconv2.GetSessionInfoResponse) int {
	count := 0
	for _, fence := range fences {
		if fence.Matches(session) {
			count++
		}
	}
	return count
}

func assertHueAllowed(t *testing.T, fences []data.Fence, playerCount int, grids []rconv2.Grid, want bool) {
	t.Helper()
	for _, grid := range grids {
		got := false
		for _, fence := range fences {
			if fence.Matches(hueOutskirtsSession(playerCount)) && fence.Includes(grid) {
				got = true
				break
			}
		}
		if got != want {
			t.Errorf("grid %s%d allowed = %t, want %t", grid.X, grid.Y, got, want)
		}
	}
}
