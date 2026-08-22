package maprules

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kilan558/hllv-geofences/data"
)

var quangNgaiMapNames = []string{
	"wdevb_warfare_day",
	"wdevb",
	"Quảng Ngãi",
	"Quang Ngai",
}

// ApplyQuangNgai replaces only the Quảng Ngãi Warfare fences on every
// configured server. NVA/Axis starts at J3-J8 and USA/Allies at A3-A8, so the
// classic Carentan team geometry can be retained without faction mirroring.
func ApplyQuangNgai(config *data.Config, variant Variant) error {
	if config == nil {
		return errors.New("config must not be nil")
	}

	axis, allies, err := quangNgaiFences(variant)
	if err != nil {
		return err
	}
	for index := range config.Servers {
		server := &config.Servers[index]
		server.AxisFence = append(withoutQuangNgai(server.AxisFence), axis...)
		server.AlliesFence = append(withoutQuangNgai(server.AlliesFence), allies...)
	}
	return nil
}

func quangNgaiFences(variant Variant) (axis []data.Fence, allies []data.Fence, err error) {
	switch variant {
	case VariantMidcap:
		return quangNgaiMidcapFences(midcapPlayerCutoff)
	case VariantMidcapAdvanced:
		return quangNgaiMidcapFences(advancedPlayerCutoff)
	case VariantLastcap:
		return quangNgaiLastcapFences(lastcapPlayerCutoff)
	default:
		return nil, nil, fmt.Errorf("unknown geofence variant %q", variant)
	}
}

func quangNgaiMidcapFences(playerCutoff int) (axis []data.Fence, allies []data.Fence, err error) {
	// NVA/Axis receives the classic Carentan Axis half plus the opposite A
	// spawn strip. USA/Allies receives the classic Allies half plus J.
	axis = append(axis, quangNgaiRectangle("ABCDEFGHIJ", 2, 2, playerCutoff)...)
	axis = append(axis, quangNgaiRectangle("AEFGHIJ", 3, 8, playerCutoff)...)
	axis = append(axis, quangNgaiRectangle("ABCDEFGHIJ", 9, 9, playerCutoff)...)

	allies = append(allies, quangNgaiRectangle("ABCDEFGHIJ", 2, 2, playerCutoff)...)
	allies = append(allies, quangNgaiRectangle("ABCDEFJ", 3, 8, playerCutoff)...)
	allies = append(allies, quangNgaiRectangle("ABCDEFGHIJ", 9, 9, playerCutoff)...)
	return axis, allies, nil
}

func quangNgaiLastcapFences(playerCutoff int) (axis []data.Fence, allies []data.Fence, err error) {
	axis = quangNgaiRectangle("CDEFGHIJ", 2, 9, playerCutoff)
	allies = quangNgaiRectangle("ABCDEFGH", 3, 8, playerCutoff)
	return axis, allies, nil
}

func quangNgaiRectangle(columns string, firstRow, lastRow, playerCutoff int) []data.Fence {
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
						"map_name":  append([]string(nil), quangNgaiMapNames...),
						"game_mode": {"Warfare"},
					},
					LessThan: map[string]int{"player_count": playerCutoff},
				},
			})
		}
	}
	return fences
}

func withoutQuangNgai(fences []data.Fence) []data.Fence {
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
			if !isQuangNgaiMapName(mapName) {
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

func isQuangNgaiMapName(value string) bool {
	value = strings.TrimSpace(value)
	for _, mapName := range quangNgaiMapNames {
		if strings.EqualFold(value, mapName) {
			return true
		}
	}
	return false
}
