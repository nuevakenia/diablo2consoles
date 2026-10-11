package d2player

// Pure layout of the waypoint panel of the original (UI_DrawWaygatePanel 0x499190, row table at 0x721860, 9 records of 6 ints;
// the numbers were read from the exe file, the code that uses them from the decompiler). All values are panel relative
// (add the panel offset, 80 and 60 at 800x600); pictures stand on their bottom edge.
//
// Record fields: [0] icon x, [1] icon bottom y, [2] text x (the decompiler lost the x argument of the text call, so
// that [2] is the text x is UNVERIFIED but it is the only unexplained field of the record), [3] text bottom y,
// [4] hit-rect left, [5] hit-rect top. The old Go table used a uniform 36 pitch for the icons and rows; the real
// pitch is 36 except between rows 3 and 4 where it is 37.

type wpRowRecord struct{ iconX, iconBottom, textX, textBottom, hitX, hitTop int }

//nolint:gochecknoglobals // table of the exe (0x721860)
var wpRowTable = [9]wpRowRecord{
	{17, 89, 80, 84, 17, 60},
	{17, 125, 80, 119, 17, 96},
	{17, 161, 80, 154, 17, 132},
	{17, 197, 80, 189, 17, 168},
	{17, 234, 80, 224, 17, 205},
	{17, 270, 80, 259, 17, 241},
	{17, 306, 80, 294, 17, 277},
	{17, 342, 80, 329, 17, 313},
	{17, 378, 80, 364, 17, 349},
}

// Hit rectangle of a row: left = record.hitX, top = record.hitTop, height = iconBottom - hitTop (29 in every record),
// width UNVERIFIED (283: the width of the row picture is not in the exe).
const (
	wpMaxRows = 9
	wpHitW    = 283
)

// WaypointRowRects returns the rectangles of row i at the mode: "icon" (the row picture standing on its bottom; width
// UNVERIFIED), "hit" (the click area; the same box), "text" anchor (left x, bottom y). ok is false for i outside 0..8.
func (m Mode) WaypointRowRects(i int) (rs []UIRect, ok bool) {
	if i < 0 || i >= wpMaxRows {
		return nil, false
	}

	ox, oy := m.PanelOffset()
	r := wpRowTable[i]
	h := r.iconBottom - r.hitTop

	return []UIRect{
		{"waypoint", "hit", ox + r.hitX, oy + r.hitTop, wpHitW, h},
		{"waypoint", "text", ox + r.textX, oy + r.textBottom, 0, 0},
	}, true
}

// WaypointClose is the close button (32x32 standing on 0x1a1) and its hover rectangle (0x24 wide, 0x22 high from
// (0x111, 0x183), both panel relative, verified) and the tooltip box (string 0x1022) which is as wide as the text + 7,
// centred on panel x 0x122.5, y 0x172..0x183, text bottom 0x181.
func (m Mode) WaypointClose() (button, hover UIRect) {
	ox, oy := m.PanelOffset()

	return UIRect{"waypoint", "close", ox + 0x111, oy + 0x1a1 - 32, 32, 32},
		UIRect{"waypoint", "close_hover", ox + 0x111, oy + 0x183, 0x24, 0x22}
}

// WaypointTabBottom is the screen y of the bottom of the act tab pictures: 93 plus 1 in the expansion layout. The tab x
// values (panel relative) are wpTabXs for the expansion layout and wpTabXsClassic for the 4 tab one.
func WaypointTabBottom(expansion bool) int {
	if expansion {
		return 94
	}

	return 93
}

// wpTabXsClassic is the 4 tab table of the non-expansion branch (UNVERIFIED which game state selects it; the exe tests
// MAIN_CheckExpansionMpq() and UI_GetOpenPanelMask(); the Go game is always the expansion).
//
//nolint:gochecknoglobals // table
var wpTabXsClassic = [4]int{3, 0x51, 0x9f, 0xed}
