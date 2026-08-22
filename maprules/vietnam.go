package maprules

import (
	"errors"
	"fmt"

	"github.com/kilan558/hllv-geofences/data"
)

// ApplyVietnam replaces every configured fence with the canonical Warfare
// rules for the six supported HLL: Vietnam maps. Connection settings, grid
// geometry, timing, whitelists, and messages remain unchanged.
func ApplyVietnam(config *data.Config, variant Variant) error {
	if config == nil {
		return errors.New("config must not be nil")
	}
	// Validate before clearing existing rules so a bad variant cannot leave a
	// partially mutated runtime configuration.
	if _, _, err := vanTuongFences(variant); err != nil {
		return err
	}

	for index := range config.Servers {
		config.Servers[index].AxisFence = nil
		config.Servers[index].AlliesFence = nil
	}

	applyFunctions := []func(*data.Config, Variant) error{
		ApplyVanTuong,
		ApplyQuangNgai,
		ApplyHueOutskirts,
		ApplyDakToAirfield,
		ApplyCamRanhPort,
		ApplyThanhHoa,
	}
	for _, apply := range applyFunctions {
		if err := apply(config, variant); err != nil {
			return err
		}
	}
	return nil
}

// ApplyVietnamWithMaxPlayers applies all canonical Vietnam rules and replaces
// their player-count condition with an inclusive operator-defined maximum.
func ApplyVietnamWithMaxPlayers(config *data.Config, variant Variant, maxPlayers int) error {
	if maxPlayers < 0 || maxPlayers > 100 {
		return fmt.Errorf("max players must be between 0 and 100, got %d", maxPlayers)
	}
	if err := ApplyVietnam(config, variant); err != nil {
		return err
	}
	cutoff := maxPlayers + 1
	for serverIndex := range config.Servers {
		server := &config.Servers[serverIndex]
		setFencePlayerCutoff(server.AxisFence, cutoff)
		setFencePlayerCutoff(server.AlliesFence, cutoff)
	}
	return nil
}

func setFencePlayerCutoff(fences []data.Fence, cutoff int) {
	for index := range fences {
		condition := fences[index].Condition
		if condition == nil {
			continue
		}
		if condition.LessThan == nil {
			condition.LessThan = make(map[string]int)
		}
		condition.LessThan["player_count"] = cutoff
	}
}
