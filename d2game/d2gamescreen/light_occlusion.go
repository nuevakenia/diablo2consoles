package d2gamescreen

import (
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2lightmap"
)

// lightUnitLOS reports whether the hero and monster lights use the line-of-sight painter (light quality 2
// of the exe). Off by default (UNVERIFIED which quality OD2 should run at); OD2_LIGHT_QUALITY=2 turns it on.
func lightUnitLOS() bool { return os.Getenv("OD2_LIGHT_QUALITY") == "2" }

// lightBlocker is the occupancy test of the occluded light painters: nil when objects should not occlude
// (OD2_LIGHT_OBJ_LOS=0) or there is no map.
func (v *Game) lightBlocker() d2lightmap.Blocker {
	if v.gameClient == nil || v.gameClient.MapEngine == nil || os.Getenv("OD2_LIGHT_OBJ_LOS") == "0" {
		return nil
	}

	return v.gameClient.MapEngine.LightBlocked
}
