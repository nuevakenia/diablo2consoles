package d2mapgen

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

// BuildBackdrop builds a level into a map engine of its own, for the renderer to draw beyond the edge of the level
// the hero stands in (d2maprenderer.Backdrop). Only the terrain matters: no monsters, no population, and none of the
// slow probing of the maze levels, so it costs the level's generation time (30-100 ms for an outdoor level).
// The engine's World says where the level sits.
func BuildBackdrop(a *d2asset.AssetManager, l d2util.LogLevel, levelID int, req LoadRequest) (*d2mapengine.MapEngine, error) {
	eng := d2mapengine.CreateMapEngine(l, a)

	g := &MapGenerator{asset: a, engine: eng, backdrop: true}
	g.Logger = d2util.NewLogger()
	g.Logger.SetLevel(l)
	g.Logger.SetPrefix(logPrefix)

	g.providers = []LevelProvider{actTownProvider{}, townProvider{}}
	if RealMapsEnabled() {
		g.providers = append([]LevelProvider{outdoorProvider{}, presetProvider{}}, g.providers...)
	}

	if _, err := g.LoadLevel(levelID, req); err != nil {
		return nil, fmt.Errorf("backdrop of level %d: %w", levelID, err)
	}

	return eng, nil
}
