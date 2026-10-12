package d2player

// Pure layout of the waypoint panel of the original (UI_DrawWaygatePanel 0x499190, row table at 0x721860, 9 records of 6 ints;
// the numbers were read from the exe file, the code that uses them from the decompiler). All values are panel relative
// (add the panel offset, 80 and 60 at 800x600); pictures stand on their bottom edge.
//
// Record fields: [0] icon x, [1] icon bottom y, [2] text x (the decompiler lost the x argument of the text call, so
// that [2] is the text x is VERIFIED from the call site at 0x499564: EDX = record[2] + panel x, left aligned), [3] text bottom y,
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

// Hit rectangle of a row (VERIFIED, the row hit test at 0x498cf0): with the mouse in panel space the row is hit when
// left < x < left + 0x118 and top < y < top + 0x1e, both ends exclusive, where left and top are record fields [4] and
// [5]. As a half open box that is x = left+1, width 279, y = top+1, height 29.
const (
	wpMaxRows = 9
	wpHitW    = 0x118 - 1
	wpHitH    = 0x1e - 1
)

// WaypointRowRects returns the rectangles of row i at the mode: "icon" (not returned: the row picture is not a hit box), "hit" (the click area; the same box), "text" anchor (left x, bottom y). ok is false for i outside 0..8.
func (m Mode) WaypointRowRects(i int) (rs []UIRect, ok bool) {
	if i < 0 || i >= wpMaxRows {
		return nil, false
	}

	ox, oy := m.PanelOffset()
	r := wpRowTable[i]
	return []UIRect{
		{"waypoint", "hit", ox + r.hitX + 1, oy + r.hitTop + 1, wpHitW, wpHitH},
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

// wpTabXsClassic is the 4 tab table of the non-expansion branch. VERIFIED which state selects it: the 5 tab table needs
// MAIN_CheckExpansionMpq() (0x406370) and the getter at 0x449230 (named UI_GetOpenPanelMask, but it only returns the
// global 0x79857c that the expansion tests use) to be both non-zero; otherwise the 4 tab table is used. The Go game is
// always the expansion. Tab hit test (0x498c70): y <= 0x1e and x <= 0x140 in panel space, tab index from the cut points
// 0x40/0x80/0xc0/0x100 (expansion, 5 tabs) or 0x50/0xa0/0xf0 (classic, 4 tabs); left of 0x40 (0x50) is no tab.
//
//nolint:gochecknoglobals // table
var wpTabXsClassic = [4]int{3, 0x51, 0x9f, 0xed}
