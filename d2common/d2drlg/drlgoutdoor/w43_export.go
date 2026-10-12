package drlgoutdoor

// RectsAdjacent reports whether two level rectangles share a border segment (the test that links the levels of an
// outdoor world). Exported for scripts/mapinfo.go, which plans the route of the full-game chain.
func RectsAdjacent(a, b Rect) bool { return rectsAdjacent(a, b) }
