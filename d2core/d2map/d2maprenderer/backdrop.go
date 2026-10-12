package d2maprenderer

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2math/d2vector"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapengine"
)

// Backdrop is a neighbouring level that is drawn (floors, walls, shadows and roofs, no entities) beyond the edge of
// the map the renderer shows. Without it the screen of a hero standing near the border of an outdoor level is black
// on the far side of the border. The original has the same picture because the rooms of the levels that touch the
// level the hero stands in are created too, so the terrain continues over the border (UNVERIFIED how far the
// original builds them; the engine draws every tile of the neighbour that falls on screen).
//
// A tile (x, y) of Engine lies at (x+DX, y+DY) in the main map.
type Backdrop struct {
	Engine *d2mapengine.MapEngine
	DX, DY int
}

// backdropLayer is a backdrop with its own renderer (palette, tile images), created when it is first needed.
type backdropLayer struct {
	Backdrop
	r *MapRenderer
}

// SetBackdrops replaces the neighbouring levels drawn beyond the map edge (nil: none).
func (mr *MapRenderer) SetBackdrops(bs []Backdrop) {
	mr.backdrops = nil

	for _, b := range bs {
		if b.Engine != nil {
			mr.backdrops = append(mr.backdrops, &backdropLayer{Backdrop: b})
		}
	}
}

// BackdropCount is the number of neighbouring levels set.
func (mr *MapRenderer) BackdropCount() int { return len(mr.backdrops) }

// ensure builds the renderer of the backdrop: it shares the viewport, the light and the fade state of the main one
// and owns the tile image cache of the neighbour's level type.
func (b *backdropLayer) ensure(mr *MapRenderer) *MapRenderer {
	if b.r != nil {
		return b.r
	}

	r := &MapRenderer{
		asset:     mr.asset,
		renderer:  mr.renderer,
		mapEngine: b.Engine,
		viewport:  mr.viewport,
		light:     mr.light,
		Logger:    mr.Logger,
		offX:      b.DX,
		offY:      b.DY,
	}

	savedFades := mr.light.fades
	r.generateTileCache() // resets the (shared) fades: keep the main renderer's
	mr.light.fades = savedFades

	b.r = r

	return r
}

// renderBackdrops draws the neighbouring levels that show on screen, before the main map.
func (mr *MapRenderer) renderBackdrops(target d2interface.Surface) {
	if len(mr.backdrops) == 0 {
		return
	}

	sz := d2display.Get()
	cam := mr.viewport.camera

	if cam == nil || cam.GetPosition() == nil {
		return
	}

	px, py := cam.GetPosition().X(), cam.GetPosition().Y()

	defer func() { mr.viewport.camera = cam }()

	for _, b := range mr.backdrops {
		// the same screen seen from the neighbour's own tile coordinates: its camera is the main camera moved by the
		// ortho offset of the two origins
		ox, oy := mr.viewport.WorldToOrtho(float64(b.DX), float64(b.DY))
		pos := d2vector.NewPosition(px-ox, py-oy)
		mr.viewport.camera = &Camera{position: &pos}

		size := b.Engine.Size()
		startX, startY, endX, endY := TileRange(mr.viewport, sz.W, sz.H, size.Width, size.Height)

		if startX >= endX || startY >= endY {
			continue
		}

		r := b.ensure(mr)
		r.animTicks, r.animAcc = mr.animTicks, mr.animAcc
		r.roofCover = mr.roofCover

		rows := VisibleTileRows(mr.viewport, sz.W, sz.H, startX, startY, endX, endY)

		r.renderPass1(target, startY, endY, rows)
		r.renderBackdropUpper(target, startY, endY, rows)
	}
}

// renderBackdropUpper draws shadows, upper walls and roofs of the visible tiles, row by row.
func (mr *MapRenderer) renderBackdropUpper(target d2interface.Surface, startY, endY int, rows *TileRows) {
	for tileY := startY; tileY < endY; tileY++ {
		from, to := rows.Row(tileY)

		for tileX := from; tileX < to; tileX++ {
			tile := mr.mapEngine.TileAt(tileX, tileY)
			if tile == nil {
				continue
			}

			mr.viewport.PushTranslationWorld(float64(tileX), float64(tileY))

			for _, shadow := range tile.Components.Shadows {
				if !shadow.Hidden() && shadow.Prop1 != 0 {
					mr.renderShadow(shadow, target, tileX, tileY)
				}
			}

			mr.renderTilePass2(tile, target, tileX, tileY)
			mr.renderTilePass3(tile, target, tileX, tileY)
			mr.viewport.PopTranslation()
		}
	}
}
