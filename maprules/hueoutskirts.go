package maprules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kilan558/hllv-geofences/data"
)

var hueOutskirtsMapNames = []string{
	"wdevc_warfare_day",
	"wdevc",
	"Huế Outskirts",
	"Hue Outskirts",
}

// ApplyHueOutskirts replaces only the Huế Outskirts Warfare fences on every
// configured server. NVA/Axis starts at C1-H1 and uses the classic SMDM
// Allies geometry; USA/Allies starts at C10-H10 and uses the Axis geometry.
func ApplyHueOutskirts(config *data.Config, variant Variant) error {
	if config == nil {
		return errors.New("config must not be nil")
	}

	axis, allies, err := hueOutskirtsFences(variant)
	if err != nil {
		return err
	}
	for index := range config.Servers {
		server := &config.Servers[index]
		server.AxisFence = append(withoutHueOutskirts(server.AxisFence), axis...)
		server.AlliesFence = append(withoutHueOutskirts(server.AlliesFence), allies...)
	}
	return nil
}

func hueOutskirtsFences(variant Variant) (axis []data.Fence, allies []data.Fence, err error) {
	switch variant {
	case VariantMidcap:
		return hueOutskirtsMidcapFences(midcapPlayerCutoff)
	case VariantMidcapAdvanced:
		return hueOutskirtsMidcapFences(advancedPlayerCutoff)
	case VariantLastcap:
		return hueOutskirtsLastcapFences(lastcapPlayerCutoff)
	default:
		return nil, nil, fmt.Errorf("unknown geofence variant %q", variant)
	}
}

func hueOutskirtsMidcapFences(playerCutoff int) (axis []data.Fence, allies []data.Fence, err error) {
	// NVA/Axis receives classic SMDM Allies: top half plus the opposite HQ row.
	axis = append(axis, hueOutskirtsRectangle("BI", 1, 10, playerCutoff)...)
	axis = append(axis, hueOutskirtsRectangle("CDEFGH", 1, 6, playerCutoff)...)
	axis = append(axis, hueOutskirtsRectangle("CDEFGH", 10, 10, playerCutoff)...)

	// USA/Allies receives classic SMDM Axis: bottom half plus the opposite HQ row.
	allies = append(allies, hueOutskirtsRectangle("BI", 1, 10, playerCutoff)...)
	allies = append(allies, hueOutskirtsRectangle("CDEFGH", 1, 1, playerCutoff)...)
	allies = append(allies, hueOutskirtsRectangle("CDEFGH", 5, 10, playerCutoff)...)
	return axis, allies, nil
}

func hueOutskirtsLastcapFences(playerCutoff int) (axis []data.Fence, allies []data.Fence, err error) {
	axis = append(axis, hueOutskirtsRectangle("BI", 1, 10, playerCutoff)...)
	axis = append(axis, hueOutskirtsRectangle("CDEFGH", 1, 8, playerCutoff)...)

	allies = append(allies, hueOutskirtsRectangle("BI", 1, 10, playerCutoff)...)
	allies = append(allies, hueOutskirtsRectangle("CDEFGH", 3, 10, playerCutoff)...)
	return axis, allies, nil
}

func hueOutskirtsRectangle(columns string, firstRow, lastRow, playerCutoff int) []data.Fence {
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
						"map_name":  append([]string(nil), hueOutskirtsMapNames...),
						"game_mode": {"Warfare"},
					},
					LessThan: map[string]int{"player_count": playerCutoff},
				},
			})
		}
	}
	return fences
}

func withoutHueOutskirts(fences []data.Fence) []data.Fence {
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
			if !isHueOutskirtsMapName(mapName) {
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

		condition := *fence.Condition
		condition.Equals = cloneStringMap(condition.Equals)
		condition.Equals["map_name"] = remaining
		fence.Condition = &condition
		result = append(result, fence)
	}
	return result
}

func isHueOutskirtsMapName(value string) bool {
	value = strings.TrimSpace(value)
	for _, mapName := range hueOutskirtsMapNames {
		if strings.EqualFold(value, mapName) {
			return true
		}
	}
	return false
}
