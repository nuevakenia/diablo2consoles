package d2player

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2locale"
)

// Pure layout of the quest log of the original (UI_DrawQuestLogPanel 0x49fc00; slot table 0x7231a8, text tables
// 0x723510..0x723520 read from the exe file). Panel relative values; add the panel offset (80, 60 at 800x600).
// Pictures stand on their bottom edge.
//
//	four panel pictures   as every left panel (UI_DrawCharacterStatsPanel layout)
//	act tabs (expansion)  x 5, 0x43, 0x81, 0xbf, 0xfd; bottom y H - 60 - 0x1c0 + 1 = 93 (the +1 is the expansion flag)
//	                      the classic branch has 4 tabs at 5, 0x52, 0x9f, 0xec (VERIFIED same selector as the 5 tab table: MAIN_CheckExpansionMpq() and the expansion global 0x79857c; the
//	                      classic loop draws tabs 0..3)
//	quest slots           x 26, 123, 220; bottoms 121 and 218 (six slots, 2 rows)
//	name line             bottom 248 (table 0x723518), VERIFIED centred on panel x 0xa0 (x = 0xa0 - width/2, 0x4a03c8)
//	description           wrapped at 270 px (0x10e), x 16 (table 0x723514, VERIFIED 0x4a0422), first line bottom 270 (table 0x723510), pitch 0x14 (20)
//	close button          32x32, x 0x116, bottom H - 60 - 0x3a = 482; hover x 0x116 wide 0x24, y 0x188 high 0x22, text 0x1030
//	"description" button  x 0xe2, same bottom (frame 0, 1 when toggled); hover x 0xe6 wide 0x1e, y 0x187 high 0x21, text 0xe88
//	tooltips              y H - 60 - 0x5f
const (
	questNameBottom     = 248
	questDescrBottom0   = 270
	questDescrPitch     = 0x14
	questDescrX         = 16 // table 0x723514
	questDescrWrapWidth = 0x10e
	questCloseDX        = 0x116
	questCloseBottomY   = 0x3a // H - e4 - 0x3a
	questDescrBtnDX     = 0xe2
)

// QuestLayoutRects returns the golden rectangles of the quest log for the mode: tabs, slots, close and description
// buttons with their hover boxes. The tab and slot sizes are those of the pictures (63x31 and 80x95: read from the
// files at run time, taken from the checked 9f-ui-layout golden).
func (m Mode) QuestLayoutRects() []UIRect {
	ox, oy := m.PanelOffset()
	rs := make([]UIRect, 0, 24)

	for i, x := range [5]int{5, 0x43, 0x81, 0xbf, 0xfd} {
		rs = append(rs, UIRect{"quest", "tab" + string(rune('0'+i)), ox + x, m.H - oy - 0x1c0 + 1 - 31, 63, 31})
	}

	slotX := [3]int{26, 123, 220}
	slotBottom := [2]int{121, 218}

	for i := 0; i < 6; i++ {
		bottom := oy + slotBottom[i/3]
		rs = append(rs, UIRect{"quest", "slot" + string(rune('0'+i)), ox + slotX[i%3], bottom - 95, 80, 95})
	}

	bottom := m.H - oy - questCloseBottomY

	rs = append(rs,
		UIRect{"quest", "close", ox + questCloseDX, bottom - 32, 32, 32},
		UIRect{"quest", "close_hover", ox + questCloseDX, oy + 0x188, 0x24, 0x22},
		UIRect{"quest", "descr_button", ox + questDescrBtnDX, bottom - 32, 32, 32},
		UIRect{"quest", "descr_hover", ox + 0xe6, oy + 0x187, 0x1e, 0x21},
		UIRect{"quest", "name_bottom", ox, oy + questNameBottom, 0, 0},
		UIRect{"quest", "descr_bottom0", ox + questDescrX, oy + questDescrBottom0, questDescrWrapWidth, questDescrPitch},
	)

	return rs
}

// QuestDescriptionText wraps the quest description at questDescrWrapWidth pixels (the exe wraps at 270 px, read
// at 0x49fc00) using the given pixel measure; lines are joined with newlines (pitch questDescrPitch).
func QuestDescriptionText(text string, measure func(string) int) string {
	return strings.Join(d2locale.Default().WrapText(text, questDescrWrapWidth, measure), "\n")
}
