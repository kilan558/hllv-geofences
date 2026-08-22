package maprules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kilan558/hllv-geofences/data"
)

// Variant selects the geofence geometry and player cutoff inherited from the
// corresponding classic-HLL Carentan configuration.
type Variant string

const (
	VariantMidcap         Variant = "midcap"
	VariantLastcap        Variant = "lastcap"
	VariantMidcapAdvanced Variant = "midcap-advanced"
)

var vanTuongMapNames = []string{
	"wdeva_warfare_day", // HLL:V layer ID
	"wdeva",             // HLL:V base-map ID used by some RCON builds
	"Vạn Tường",         // Display name used by some RCON builds
}

// ApplyVanTuong replaces only the Vạn Tường Warfare fences on every server in
// config. Existing rules for other maps and all connection data are preserved.
// Repeated calls are idempotent.
func ApplyVanTuong(config *data.Config, variant Variant) error {
	if config == nil {
		return errors.New("config must not be nil")
	}

	axis, allies, err := vanTuongFences(variant)
	if err != nil {
		return err
	}
	for index := range config.Servers {
		server := &config.Servers[index]
		server.AxisFence = append(withoutVanTuong(server.AxisFence), axis...)
		server.AlliesFence = append(withoutVanTuong(server.AlliesFence), allies...)
	}
	return nil
}

func vanTuongFences(variant Variant) (axis []data.Fence, allies []data.Fence, err error) {
	switch variant {
	case VariantMidcap:
		// Vạn Tường reverses Carentan's factions: NVA/Axis starts on A,
		// US/Allies on J. Therefore each side receives Carentan's opposite
		// team geometry.
		return midcapFences(midcapPlayerCutoff)
	case VariantMidcapAdvanced:
		return midcapFences(advancedPlayerCutoff)
	case VariantLastcap:
		return lastcapFences(lastcapPlayerCutoff)
	default:
		return nil, nil, fmt.Errorf("unknown geofence variant %q", variant)
	}
}

func midcapFences(playerCutoff int) (axis []data.Fence, allies []data.Fence, err error) {
	// Carentan lets both teams traverse the complete rows 2 and 9. NVA gets
	// the classic Allies A-F half plus the opposing J spawn strip; US gets
	// the classic Axis E-J half plus the opposing A spawn strip.
	axis = append(axis, rectangle("ABCDEFGHIJ", 2, 2, playerCutoff)...)
	axis = append(axis, rectangle("ABCDEFJ", 3, 8, playerCutoff)...)
	axis = append(axis, rectangle("ABCDEFGHIJ", 9, 9, playerCutoff)...)

	allies = append(allies, rectangle("ABCDEFGHIJ", 2, 2, playerCutoff)...)
	allies = append(allies, rectangle("AEFGHIJ", 3, 8, playerCutoff)...)
	allies = append(allies, rectangle("ABCDEFGHIJ", 9, 9, playerCutoff)...)
	return axis, allies, nil
}

func lastcapFences(playerCutoff int) (axis []data.Fence, allies []data.Fence, err error) {
	// The old Carentan lastcap file contains every Axis coordinate twice.
	// Store each effective coordinate once; duplicate allowed-area entries do
	// not change enforcement and only make the runtime configuration larger.
	axis = rectangle("ABCDEFGH", 3, 8, playerCutoff)
	allies = rectangle("CDEFGHIJ", 2, 9, playerCutoff)
	return axis, allies, nil
}

func rectangle(columns string, firstRow, lastRow, playerCutoff int) []data.Fence {
	fences := make([]data.Fence, 0, len(columns)*(lastRow-firstRow+1))
	for row := firstRow; row <= lastRow; row++ {
		for _, column := range columns {
			x := string(column)
			y := row
			fences = append(fences, data.Fence{
				X: &x,
				Y: &y,
				Condition: &data.Condition{
					Equals: map[string][]string{
						"map_name":  append([]string(nil), vanTuongMapNames...),
						"game_mode": {"Warfare"},
					},
					LessThan: map[string]int{"player_count": playerCutoff},
				},
			})
		}
	}
	return fences
}

func withoutVanTuong(fences []data.Fence) []data.Fence {
	result := make([]data.Fence, 0, len(fences))
	for _, fence := range fences {
		if fence.Condition == nil {
			result = append(result, fence)
			continue
		}

		mapNames, exists := fence.Condition.Equals["map_name"]
		if !exists {
			result = append(result, fence)
			continue
		}

		remaining := make([]string, 0, len(mapNames))
		for _, mapName := range mapNames {
			if !isVanTuongMapName(mapName) {
				remaining = append(remaining, mapName)
			}
		}
		if len(remaining) == len(mapNames) {
			result = append(result, fence)
			continue
		}
		if len(remaining) == 0 {
			continue
		}

		// Preserve a grouped rule for its other maps without mutating the
		// original Condition shared by the caller.
		condition := *fence.Condition
		condition.Equals = cloneStringMap(condition.Equals)
		condition.Equals["map_name"] = remaining
		fence.Condition = &condition
		result = append(result, fence)
	}
	return result
}

func isVanTuongMapName(value string) bool {
	value = strings.TrimSpace(value)
	for _, mapName := range vanTuongMapNames {
		if strings.EqualFold(value, mapName) {
			return true
		}
	}
	return false
}

func cloneStringMap(source map[string][]string) map[string][]string {
	clone := make(map[string][]string, len(source))
	for key, values := range source {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}
