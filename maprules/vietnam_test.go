package maprules_test

import (
	"testing"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/maprules"
)

func TestApplyVietnamReplacesAllFencesWithCanonicalVietnamMaps(t *testing.T) {
	warning := "Keep this warning"
	punish := "Keep this punishment"
	config := data.Config{Servers: []data.Server{{
		Host:     "192.0.2.1",
		Port:     7779,
		Password: "unchanged-secret",
		AxisFence: []data.Fence{
			vietnamTestFence("Carentan"),
			vietnamTestFence("wdeva_warfare_day"),
		},
		AlliesFence: []data.Fence{
			vietnamTestFence("Carentan"),
			vietnamTestFence("wdeva_warfare_day"),
		},
		Messages: &data.Messages{Warning: &warning, Punish: &punish},
	}}}

	if err := maprules.ApplyVietnam(&config, maprules.VariantMidcap); err != nil {
		t.Fatalf("ApplyVietnam() error = %v", err)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("Vietnam config is invalid: %v", err)
	}

	server := config.Servers[0]
	if server.Host != "192.0.2.1" || server.Port != 7779 || server.Password != "unchanged-secret" {
		t.Fatalf("ApplyVietnam() changed server credentials: %+v", server)
	}
	if server.Messages == nil || server.Messages.Warning != &warning || server.Messages.Punish != &punish {
		t.Fatalf("ApplyVietnam() changed configured messages: %+v", server.Messages)
	}

	for _, layer := range []string{
		"wdeva_warfare_day",
		"wdevb_warfare_day",
		"wdevc_warfare_day",
		"wdevd_warfare_day",
		"wdeve_warfare_day",
		"wdevf_warfare_day",
	} {
		session := vietnamTestSession(layer)
		if got := vietnamMatchingFences(server.AxisFence, session); got == 0 {
			t.Errorf("Axis/NVA fences for %s = 0, want canonical rules", layer)
		}
		if got := vietnamMatchingFences(server.AlliesFence, session); got == 0 {
			t.Errorf("Allies/USA fences for %s = 0, want canonical rules", layer)
		}
	}

	classic := vietnamTestSession("Carentan")
	if got := vietnamMatchingFences(server.AxisFence, classic); got != 0 {
		t.Errorf("Axis/NVA Classic-HLL fences = %d, want 0", got)
	}
	if got := vietnamMatchingFences(server.AlliesFence, classic); got != 0 {
		t.Errorf("Allies/USA Classic-HLL fences = %d, want 0", got)
	}
}

func TestApplyVietnamRejectsInvalidInputWithoutChangingFences(t *testing.T) {
	if err := maprules.ApplyVietnam(nil, maprules.VariantMidcap); err == nil {
		t.Fatal("ApplyVietnam(nil) error = nil, want validation error")
	}

	config := data.Config{Servers: []data.Server{{
		AxisFence:   []data.Fence{vietnamTestFence("Carentan")},
		AlliesFence: []data.Fence{vietnamTestFence("Carentan")},
	}}}
	if err := maprules.ApplyVietnam(&config, maprules.Variant("invalid")); err == nil {
		t.Fatal("ApplyVietnam() error = nil, want unknown variant error")
	}
	if len(config.Servers[0].AxisFence) != 1 || len(config.Servers[0].AlliesFence) != 1 {
		t.Fatal("ApplyVietnam() mutated fences after rejecting the variant")
	}
}

func TestApplyVietnamWithMaxPlayersUsesInclusiveLimitOnEveryMap(t *testing.T) {
	config := data.Config{Servers: []data.Server{{}}}
	if err := maprules.ApplyVietnamWithMaxPlayers(&config, maprules.VariantMidcap, 42); err != nil {
		t.Fatalf("ApplyVietnamWithMaxPlayers() error = %v", err)
	}

	for _, layer := range []string{
		"wdeva_warfare_day",
		"wdevb_warfare_day",
		"wdevc_warfare_day",
		"wdevd_warfare_day",
		"wdeve_warfare_day",
		"wdevf_warfare_day",
	} {
		atLimit := vietnamTestSession(layer)
		atLimit.PlayerCount = 42
		if got := vietnamMatchingFences(config.Servers[0].AxisFence, atLimit); got == 0 {
			t.Errorf("Axis/NVA fences for %s at inclusive limit = 0", layer)
		}
		if got := vietnamMatchingFences(config.Servers[0].AlliesFence, atLimit); got == 0 {
			t.Errorf("Allies/USA fences for %s at inclusive limit = 0", layer)
		}

		afterLimit := vietnamTestSession(layer)
		afterLimit.PlayerCount = 43
		if got := vietnamMatchingFences(config.Servers[0].AxisFence, afterLimit); got != 0 {
			t.Errorf("Axis/NVA fences for %s after limit = %d, want 0", layer, got)
		}
		if got := vietnamMatchingFences(config.Servers[0].AlliesFence, afterLimit); got != 0 {
			t.Errorf("Allies/USA fences for %s after limit = %d, want 0", layer, got)
		}
	}
}

func TestApplyVietnamWithMaxPlayersRejectsUnsafeLimitBeforeMutation(t *testing.T) {
	for _, limit := range []int{-1, 101} {
		config := data.Config{Servers: []data.Server{{
			AxisFence:   []data.Fence{vietnamTestFence("Carentan")},
			AlliesFence: []data.Fence{vietnamTestFence("Carentan")},
		}}}
		if err := maprules.ApplyVietnamWithMaxPlayers(&config, maprules.VariantMidcap, limit); err == nil {
			t.Errorf("ApplyVietnamWithMaxPlayers(maxPlayers=%d) error = nil", limit)
		}
		if len(config.Servers[0].AxisFence) != 1 || len(config.Servers[0].AlliesFence) != 1 {
			t.Errorf("ApplyVietnamWithMaxPlayers(maxPlayers=%d) mutated fences", limit)
		}
	}
}

func vietnamTestFence(mapName string) data.Fence {
	x, y := "A", 3
	return data.Fence{
		X: &x,
		Y: &y,
		Condition: &data.Condition{Equals: map[string][]string{
			"map_name":  {mapName},
			"game_mode": {"Warfare"},
		}},
	}
}

func vietnamTestSession(mapName string) *rconv2.GetSessionInfoResponse {
	return &rconv2.GetSessionInfoResponse{
		MapName:     mapName,
		GameMode:    rconv2.GetSessionInfoGameModeWarfare,
		PlayerCount: 59,
	}
}

func vietnamMatchingFences(fences []data.Fence, session *rconv2.GetSessionInfoResponse) int {
	count := 0
	for _, fence := range fences {
		if fence.Matches(session) {
			count++
		}
	}
	return count
}
