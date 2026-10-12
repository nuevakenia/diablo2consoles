package d2mapengine

// LightBlocked reports whether a sub-tile blocks light for the occluded light painters: the collision mask
// 0x22 of the exe (block-LOS and block-light DT1 flags; notes renderer.md b4) or a closed door (UNVERIFIED
// that doors count). Sub-tiles outside the map block nothing.
func (m *MapEngine) LightBlocked(subX, subY int) bool {
	if subX < 0 || subY < 0 {
		return false
	}

	if m.objBlock.Blocked(subX, subY) {
		return true
	}

	tx, ty := subX/subtilesPerTile, subY/subtilesPerTile
	if tx >= m.size.Width || ty >= m.size.Height {
		return false
	}

	tile := m.TileAt(tx, ty)
	if tile == nil {
		return false
	}

	f := tile.GetSubTileFlags(subX%subtilesPerTile, subY%subtilesPerTile)

	return f.BlockLOS || f.BlockLight
}
