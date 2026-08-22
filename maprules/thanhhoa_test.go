package maprules_test

import (
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func TestThanhHoaMirrorsSMDMForVietnamTeams(t *testing.T) {
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
				AxisFence:   []data.Fence{thanhOtherMapFence()},
				AlliesFence: []data.Fence{thanhOtherMapFence()},
			}}}

			if err := maprules.ApplyThanhHoa(&config, tc.variant); err != nil {
				t.Fatalf("ApplyThanhHoa() error = %v", err)
			}
			server := config.Servers[0]
			if server.Host != "192.0.2.1" || server.Port != 7779 || server.Password != "unchanged-secret" {
				t.Fatalf("ApplyThanhHoa() changed server credentials: %+v", server)
			}
			if got := thanhMatchingCount(server.AxisFence, thanhHoaSession(tc.playerCount)); got != tc.axisCount {
				t.Fatalf("matching Axis/NVA fence count = %d, want %d", got, tc.axisCount)
			}
			if got := thanhMatchingCount(server.AlliesFence, thanhHoaSession(tc.playerCount)); got != tc.alliesCount {
				t.Fatalf("matching Allies/US fence count = %d, want %d", got, tc.alliesCount)
			}

			assertThanhAllowed(t, server.AxisFence, tc.playerCount, tc.axisAllowed, true)
			assertThanhAllowed(t, server.AxisFence, tc.playerCount, tc.axisBlocked, false)
			assertThanhAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesAllowed, true)
			assertThanhAllowed(t, server.AlliesFence, tc.playerCount, tc.alliesBlocked, false)
		})
	}
}

func TestThanhHoaRulesOnlyMatchWarfareAliasesAndConfiguredPlayerRange(t *testing.T) {
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyThanhHoa(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("ApplyThanhHoa() error = %v", err)
	}
	fences := config.Servers[0].AxisFence

	for _, mapName := range []string{"wdevf_warfare_day", "wdevf", "Thanh Hòa Bridge", "THANH HÒA BRIDGE", "Thanh Hoa Bridge"} {
		session := thanhHoaSession(59)
		session.MapName = mapName
		if got := thanhMatchingCount(fences, session); got != 62 {
			t.Fatalf("map alias %q matching fences = %d, want 62", mapName, got)
		}
	}

	fullServer := thanhHoaSession(61)
	if got := thanhMatchingCount(fences, fullServer); got != 0 {
		t.Fatalf("fences at the 61-player cutoff = %d, want 0", got)
	}
	wrongMode := thanhHoaSession(59)
	wrongMode.GameMode = rconv2.GetSessionInfoGameModeConquest
	if got := thanhMatchingCount(fences, wrongMode); got != 0 {
		t.Fatalf("fences in Conquest = %d, want 0", got)
	}
	wrongMap := thanhHoaSession(59)
	wrongMap.MapName = "wdeva_warfare_day"
	if got := thanhMatchingCount(fences, wrongMap); got != 0 {
		t.Fatalf("fences on another map = %d, want 0", got)
	}
}

func TestApplyThanhHoaIsIdempotentAndPreservesOtherMapRules(t *testing.T) {
	config := data.Config{Servers: []data.Server{{
		Host: "192.0.2.1", Port: 7779, Password: "secret",
		AxisFence: []data.Fence{thanhOtherMapFence()}, AlliesFence: []data.Fence{thanhOtherMapFence()},
	}}}

	if err := maprules.ApplyThanhHoa(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("first ApplyThanhHoa() error = %v", err)
	}
	if err := maprules.ApplyThanhHoa(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("second ApplyThanhHoa() error = %v", err)
	}

	server := config.Servers[0]
	if got := thanhMatchingCount(server.AxisFence, thanhHoaSession(59)); got != 62 {
		t.Fatalf("idempotent Axis/NVA fence count = %d, want 62", got)
	}
	vanTuong := thanhHoaSession(59)
	vanTuong.MapName = "wdeva_warfare_day"
	if got := thanhMatchingCount(server.AxisFence, vanTuong); got != 1 {
		t.Fatalf("preserved other-map Axis fence count = %d, want 1", got)
	}
	if got := thanhMatchingCount(server.AlliesFence, vanTuong); got != 1 {
		t.Fatalf("preserved other-map Allies fence count = %d, want 1", got)
	}
}

func TestApplyThanhHoaRejectsUnknownVariant(t *testing.T) {
	config := data.Config{Servers: []data.Server{{Host: "192.0.2.1", Port: 7779, Password: "secret"}}}
	if err := maprules.ApplyThanhHoa(&config, maprules.Variant("invalid")); err == nil {
		t.Fatal("ApplyThanhHoa() error = nil, want unknown variant error")
	}
}

func thanhHoaSession(playerCount int) *rconv2.GetSessionInfoResponse {
	return &rconv2.GetSessionInfoResponse{
		MapName: "wdevf_warfare_day", GameMode: rconv2.GetSessionInfoGameModeWarfare, PlayerCount: playerCount,
	}
}

func thanhOtherMapFence() data.Fence {
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

func thanhMatchingCount(fences []data.Fence, session *rconv2.GetSessionInfoResponse) int {
	count := 0
	for _, fence := range fences {
		if fence.Matches(session) {
			count++
		}
	}
	return count
}

func assertThanhAllowed(t *testing.T, fences []data.Fence, playerCount int, grids []rconv2.Grid, want bool) {
	t.Helper()
	for _, grid := range grids {
		got := false
		for _, fence := range fences {
			if fence.Matches(thanhHoaSession(playerCount)) && fence.Includes(grid) {
				got = true
				break
			}
		}
		if got != want {
			t.Errorf("grid %s%d allowed = %t, want %t", grid.X, grid.Y, got, want)
		}
	}
}
