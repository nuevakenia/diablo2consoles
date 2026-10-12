package d2gamescreen

import (
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2lightmap"
)

// lightUnitLOS reports whether the hero and monster lights use the line-of-sight painter (light quality 2
// of the exe). On by default: the exe's ladder (d2lightmap.QualityState) gives level 2 whenever the frame
// statistic is above 15 and the display toggle is 0 (toggle meaning UNVERIFIED); OD2_LIGHT_QUALITY=0 or 1 turns it off.
func lightUnitLOS() bool {
	q := os.Getenv("OD2_LIGHT_QUALITY")
	return q == "" || q == "2"
}

// lightBlocker is the occupancy test of the occluded light painters: nil when objects should not occlude
// (OD2_LIGHT_OBJ_LOS=0) or there is no map.
func (v *Game) lightBlocker() d2lightmap.Blocker {
	if v.gameClient == nil || v.gameClient.MapEngine == nil || os.Getenv("OD2_LIGHT_OBJ_LOS") == "0" {
		return nil
	}

	return v.gameClient.MapEngine.LightBlocked
}
