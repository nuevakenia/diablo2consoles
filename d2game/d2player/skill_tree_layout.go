package d2player

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// Pure layout of the skill tree panel of the original (UI_DrawSkillTreePanel 0x4a8e00, UI_DrawSkillTreeBackground
// 0x4a7220, UI_DrawSkillTreeTabTitles 0x4a7430, UI_DrawSkillTreeIcon 0x4a86b0, UI_GetSkillTreeTierY 0x4a7100,
// UI_SelectSkillTreeTab 0x4a7c90, UI_DrawSkillPointsRemaining 0x4a8c60, UI_HandleSkillTreeTabHover 0x4a7a60).
// Everything below is read from the exe (disassembly of the text calls, the close table at 0x723eac read from the
// file). e0 / e4 is the panel offset: e0 = 80, e4 = -60 at 800x600 and 0 at 640x480.
//
//	panel pictures      four pictures at x = W - e0 - 0x140 and W - e0 - 0x40, bottoms H + e4 - 0xe0 and H + e4 - 0x30
//	                    (frames 0..3 common, frames 4*(tab+1)+0..3 the selected tab); the class picture has 7 sets
//	icons               column x = W - e0 - {0x131, 0xec, 0xa7}; bottom y = H + e4 - {0x1a2, 0x15e, 0x11a, 0xd6,
//	                    0x91, 0x4d}; 48x48; the level number (UI_DrawSkillTreeIcon 0x4a86b0, VERIFIED) is
//	                    drawn left aligned at x = icon left + 0x30 (minus 4 and font 4 above level 9, else font 1), bottom
//	                    icon bottom + 0xc; colour 3 (blue) when the bonus levels are positive, 1 (red) when negative, else 0
//	tab titles          text centred in the box [W - e0 - 0x5a, W - e0], bottom y = H + e4 + c (table below), the
//	                    box is the argument pair of UI_DrawTextLine; VERIFIED centred: 0x4a3750 puts x1 + ((box + 1 - width) >> 1) when
//	                    the text is narrower than x2 - x1 + 1, else x1
//	points number       UI_DrawText at x = W - e0 - 0x34, bottom H + e4 - 400 (VERIFIED left aligned, 0x4a8a47; colour 1 when the player has state 0x36, else 0), UI_DrawSkillPointsRemaining
//	                    draws "0" centred in the box [W - e0 - 0x41, W - e0 - 0x19] when no points are left (VERIFIED, 0x4a8cdf)
//	close button        32x32 (buy/sell button frame 10, 11 when pressed) at x = W - e0 + closeX(class, tab), bottom y H + e4 - 0x3f
//	tab hit spans       [H + e4 - 0x174, -0x108, -0x9c, -0x30] (existing, verified)
const (
	skillTreeBoxWidth    = 0x5a
	skillTreePointsDX    = 0x34
	skillTreePointsZeroX = 0x19
	skillTreePointsDY    = -400
	skillTreeCloseDY     = 0x3f
)

// skillTreeCloseX is the table at 0x723eac, ints indexed by tab (1..3) + 3 * class (0 Amazon, 1 Sorceress,
// 2 Necromancer, 3 Paladin, 4 Barbarian, 5 Druid, 6 Assassin); the first int of the table is unused. Value is the
// offset from W - e0 of the close button x.
var skillTreeCloseX = [7][3]int{ //nolint:gochecknoglobals // constant table
	{-0x95, -0xdc, -0x131},
	{-0x131, -0x131, -0x95},
	{-0x131, -0x95, -0x131},
	{-0x131, -0xdc, -0x131},
	{-0x95, -0x131, -0x95},
	{-0x95, -0x95, -0x95},
	{-0xdc, -0x95, -0x131},
}

// SkillTreeClassIndex maps a hero to the class index of the exe tables (and -1 for none).
func SkillTreeClassIndex(h d2enum.Hero) int {
	switch h {
	case d2enum.HeroAmazon:
		return 0
	case d2enum.HeroSorceress:
		return 1
	case d2enum.HeroNecromancer:
		return 2
	case d2enum.HeroPaladin:
		return 3
	case d2enum.HeroBarbarian:
		return 4
	case d2enum.HeroDruid:
		return 5
	case d2enum.HeroAssassin:
		return 6
	}

	return -1
}

// SkillTreeCloseX returns the screen x of the close button for the class index and tab (0..2), as the exe computes
// it (W - e0 plus the table value).
func (m Mode) SkillTreeCloseX(class, tab int) int {
	if class < 0 || class > 6 || tab < 0 || tab > 2 {
		return 0
	}

	ox, _ := m.PanelOffset()

	return m.W - ox + skillTreeCloseX[class][tab]
}

// SkillTreeCloseBottom is the bottom edge of the close button.
func (m Mode) SkillTreeCloseBottom() int {
	_, oy := m.PanelOffset()

	return m.H - oy - skillTreeCloseDY
}

// SkillTreeTitle is one line of a tab title: the string table id and the offset c of its bottom edge from H + e4.
type SkillTreeTitle struct {
	StrID int
	C     int
}

// skillTreeCommonTitles are drawn for every class before the class specific ones (ids 0x1083..0x1085).
var skillTreeCommonTitles = []SkillTreeTitle{{0x1083, -455}, {0x1084, -443}, {0x1085, -431}} //nolint:gochecknoglobals

// skillTreeTitles are the class specific tab title lines (class index as above). Offsets c = parameter - 480 for the
// lines drawn through UI_DrawSkillTreeTitleLine (0x4a71e0: y = H + e4 + parameter - 0x1e0), the others straight from
// the text calls. Strings 0x57ed..0x57f5 are the expansion classes.
var skillTreeTitles = [7][]SkillTreeTitle{ //nolint:gochecknoglobals // constant table
	{{0x1088, -330}, {0x1089, -318}, {0x1086, -306}, {0x108a, -222}, {0x108b, -210}, {0x1086, -198},
		{0x108c, -121}, {0x108d, -109}, {0x1086, -97}},
	{{0x1099, -324}, {0x1087, -312}, {0x109a, -216}, {0x1087, -204}, {0x109b, -110}, {0x1087, -98}},
	{{0x1092, -324}, {0x1087, -312}, {0x1093, -222}, {0x1094, -210}, {0x1087, -198}, {0x1095, -104}},
	{{0x108e, -324}, {0x108f, -312}, {0x1090, -216}, {0x108f, -204}, {0x1091, -110}, {0x1086, -98}},
	{{0x1096, -318}, {0x1097, -216}, {0x1098, -204}, {0x1097, -110}, {0x1086, -98}},
	{{0x57f0, -318}, {0x57ee, -216}, {0x57ef, -204}, {0x57ed, -110}},
	{{0x57f4, -330}, {0x57f5, -318}, {0x57f2, -216}, {0x57f3, -204}, {0x57f1, -110}},
}

// SkillTreeTitles returns the title lines of the class (common ones first) with their screen bottom edges for the
// mode, as (string id, y) pairs. x range: SkillTreeTitleBox.
func (m Mode) SkillTreeTitles(class int) []SkillTreeTitleLine {
	if class < 0 || class > 6 {
		return nil
	}

	_, oy := m.PanelOffset()
	base := m.H - oy

	lines := make([]SkillTreeTitleLine, 0, len(skillTreeCommonTitles)+len(skillTreeTitles[class]))
	for _, t := range append(append([]SkillTreeTitle{}, skillTreeCommonTitles...), skillTreeTitles[class]...) {
		lines = append(lines, SkillTreeTitleLine{t.StrID, base + t.C})
	}

	return lines
}

// SkillTreeTitleLine is a title line placed on the screen.
type SkillTreeTitleLine struct {
	StrID, Y int
}

// SkillTreeTitleBox is the horizontal box [x1, x2] the title lines are centred in.
func (m Mode) SkillTreeTitleBox() (x1, x2 int) {
	ox, _ := m.PanelOffset()

	return m.W - ox - skillTreeBoxWidth, m.W - ox
}

// SkillTreePoints returns the left x and bottom y of the remaining skill points number and the x of the "0" drawn
// when the character has no points.
func (m Mode) SkillTreePoints() (x, y, zeroX int) {
	ox, oy := m.PanelOffset()

	return m.W - ox - skillTreePointsDX, m.H - oy + skillTreePointsDY, m.W - ox - skillTreePointsZeroX
}

// SkillTreeIconRect is the 48x48 icon rectangle (x, y, w, h) of the skill with the tier row 1..6 and column 1..3 of its
// skilldesc record for the mode (UI_DrawSkillTreeIcon 0x4a86b0 + UI_GetSkillTreeTierY 0x4a7100).
func (m Mode) SkillTreeIconRect(row, column int) (x, y, w, h int) {
	if row < 1 || row > 6 || column < 1 || column > 3 {
		return 0, 0, 0, 0
	}

	ox, oy := m.PanelOffset()
	colOff := [3]int{0x131, 0xec, 0xa7}
	rowOff := [6]int{0x1a2, 0x15e, 0x11a, 0xd6, 0x91, 0x4d}

	return m.W - ox - colOff[column-1], m.H - oy - rowOff[row-1] - 48, 48, 48
}

// SkillTreeLayoutRects lists the golden rectangles of the panel for a class: the close button per tab, the first icon
// of each column and the points number anchor.
func (m Mode) SkillTreeLayoutRects(class int) []UIRect {
	var rs []UIRect

	for tab := 0; tab < 3; tab++ {
		rs = append(rs, UIRect{"skills", "close_tab" + string(rune('1'+tab)), m.SkillTreeCloseX(class, tab), m.SkillTreeCloseBottom() - 32, 32, 32})
	}

	for c := 1; c <= 3; c++ {
		x, y, w, h := m.SkillTreeIconRect(1, c)
		rs = append(rs, UIRect{"skills", "icon_r1_c" + string(rune('0'+c)), x, y, w, h})
	}

	px, py, _ := m.SkillTreePoints()

	return append(rs, UIRect{"skills", "points", px, py, 0, 0})
}

// SkillTreeLevelNumber is where UI_DrawSkillTreeIcon (0x4a86b0) draws the level number under the icon: the left x and
// the bottom y of the text and whether the small font 4 (levels above 9, moved 4 to the left) is used instead of font 1.
// VERIFIED against the disassembly (the text call has the alignment argument 0 = left).
func (m Mode) SkillTreeLevelNumber(row, column, level int) (x, bottom int, bigFont bool) {
	ix, iy, w, h := m.SkillTreeIconRect(row, column)
	if w == 0 {
		return 0, 0, false
	}

	x = ix + 0x30
	if level > 9 {
		x -= 4
		bigFont = true
	}

	return x, iy + h + 0xc, bigFont
}
