package worker

import (
	"fmt"
	"math"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/kilan558/hllv-geofences/data"
)

type teamSide uint8

const (
	teamUnknown teamSide = iota
	teamAllies
	teamAxis
)

// The HLL:V definition currently carried by go-hll-rcon still uses the old
// WW2 labels. The values below are the actual HLL:V faction IDs: US=1, NVA=6.
const (
	hllvTeamUS  rconv2.GetPlayersTeam = 1
	hllvTeamNVA rconv2.GetPlayersTeam = 6
)

func hllvTeamSide(team rconv2.GetPlayersTeam) teamSide {
	switch team {
	case hllvTeamUS:
		return teamAllies
	case hllvTeamNVA:
		return teamAxis
	default:
		return teamUnknown
	}
}

var hllvNumpads = [3][3]int{
	{7, 8, 9},
	{4, 5, 6},
	{1, 2, 3},
}

func gridForPosition(position rconv2.GetPlayerPosition, geometry data.GridGeometry) (rconv2.Grid, error) {
	geometry, err := geometry.Normalized()
	if err != nil {
		return rconv2.Grid{}, err
	}

	x := float64(position.X) - geometry.CenterOffsetX
	y := float64(position.Y) - geometry.CenterOffsetY
	xGrid := math.Floor(x / geometry.SectorSize)
	yGrid := math.Floor(y / geometry.SectorSize)
	xIndex := int(xGrid) + 5
	yIndex := int(yGrid) + 6
	if xIndex < 0 || xIndex >= 10 || yIndex < 1 || yIndex > 10 {
		return rconv2.Grid{}, fmt.Errorf(
			"world position (%.2f,%.2f,%.2f) resolves outside the 10x10 grid (column=%d row=%d)",
			position.X,
			position.Y,
			position.Z,
			xIndex,
			yIndex,
		)
	}

	third := geometry.SectorSize / 3
	xInGrid := x - xGrid*geometry.SectorSize
	yInGrid := y - yGrid*geometry.SectorSize
	xNumpad := min(2, max(0, int(math.Floor(xInGrid/third))))
	yNumpad := min(2, max(0, int(math.Floor(yInGrid/third))))

	return rconv2.Grid{
		X:      string(rune('A' + xIndex)),
		Y:      yIndex,
		Numpad: hllvNumpads[yNumpad][xNumpad],
	}, nil
}
