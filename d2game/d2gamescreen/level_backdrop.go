package d2gamescreen

import (
	"os"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2drlg"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapgen"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2maprenderer"
)

// updateBackdrops gives the renderer the levels that touch the level just built, so the terrain goes on beyond the
// border instead of ending in black (wave 41 hunt: arriving in a new outdoor level showed a black half screen: the
// hero arrives edgeInset tiles inside the level, the map ends there and nothing was drawn behind the border).
// OD2_BACKDROP=0 turns it off.
func (v *Game) updateBackdrops() {
	v.mapRenderer.SetBackdrops(nil)

	if os.Getenv("OD2_BACKDROP") == "0" {
		return
	}

	w := v.gameClient.MapEngine.World()
	if w.Level == 0 {
		return
	}

	cur, ok := w.Rects[w.Level]
	if !ok {
		return
	}

	var out []d2maprenderer.Backdrop

	for id, r := range w.Rects {
		if id == w.Level {
			continue
		}

		if _, touch := d2level.SharedBorder(cur, r); !touch || !v.gameClient.CanLoadLevel(id) {
			continue
		}

		eng, err := d2mapgen.BuildBackdrop(v.asset, v.logLevel, id, d2mapgen.LoadRequest{
			Seed: d2mapgen.HeroMapSeed, Difficulty: d2drlg.Difficulty(v.gameClient.LevelDifficulty()),
		})
		if err != nil {
			v.Warningf("LEVEL backdrop of level %d: %v", id, err)
			continue
		}

		nw := eng.World()
		if nw.Level != id {
			continue
		}

		out = append(out, d2maprenderer.Backdrop{Engine: eng, DX: nw.OriginX - w.OriginX, DY: nw.OriginY - w.OriginY})
		v.Infof("LEVEL backdrop: level %d beside level %d, offset (%d,%d)", id, w.Level, nw.OriginX-w.OriginX,
			nw.OriginY-w.OriginY)
	}

	v.mapRenderer.SetBackdrops(out)
}
