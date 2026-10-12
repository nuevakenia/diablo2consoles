package d2automap

// The text lines that Game.exe 1.14b draws over the automap (AUTOMAP_DrawAutomap 0x4564a0).
//
// VERIFIED from the code: after the cells and the unit markers the running y is set to
// 0x18 (DAT_0079d244) and each line is drawn by D2WIN_DrawText with colour 4 (gold) at
// x = DAT_0079d2a8 - textWidth - 0x10 (right aligned, 16 from the right edge), then y += 0x10.
// The order is: game name line (string 0x1055 + name; only when the join info has one),
// password line (0x1056; same), the level display name (0x455770), the version line
// "v 1.14b" (0x4557c0), the difficulty line (0x4558b0: panel flag byte 1 -> string 0x1423,
// 2 -> 0x1422, formatted with the template 0x1057; nothing for other values), the local host
// address line (only when the panel layout mode is 6 or 8) and one more string-table line
// (id 0x58ca) when any panel is open. There is no game clock and no FPS line on the automap
// (the FPS/ping text is the separate debug UI_DrawFpsPingText 0x4736c0).
//
// UNVERIFIED: DAT_0079d2a8 is taken as the screen width (the view code divides it by two
// as the centre); the y is the text baseline (D2WIN_DrawText draws upward from y, UNVERIFIED);
// the font passed to D2WIN_SetCurrentFont (index 1) is taken as the 16 pixel font; the
// texts of the string-table ids are not read.
const (
	TextStartY      = 0x18
	TextStep        = 0x10
	TextRightMargin = 0x10
	// TextColorGold is the engine's text colour index 4.
	TextColorGold = 4
	// VersionText is the version line of 1.14b ("v %d.%d%c" with 1, 14, 'b').
	VersionText = "v 1.14b"
)

// TextLine is one placed line.
type TextLine struct {
	Text string
	X    int // left edge of the text
	Y    int // baseline
}

// TextStack places the non-empty lines (in the engine's order) for a screen width.
// widthOf measures a string in pixels. Empty strings are skipped and take no row,
// like the lines the engine does not draw.
func TextStack(lines []string, screenW int, widthOf func(string) int) []TextLine {
	out := make([]TextLine, 0, len(lines))
	y := TextStartY

	for _, s := range lines {
		if s == "" {
			continue
		}

		out = append(out, TextLine{Text: s, X: screenW - widthOf(s) - TextRightMargin, Y: y})
		y += TextStep
	}

	return out
}

// DifficultyStringID returns the string table id of the difficulty line for the
// panel flag byte (0 = no line).
func DifficultyStringID(flag int) int {
	switch flag {
	case 1:
		return 0x1423
	case 2:
		return 0x1422
	}

	return 0
}
