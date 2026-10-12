package d2automap

// Ground item labels (the Alt key). Ported from INV_DrawGroundItemLabels 0x4bd100,
// UI_BuildGroundItemLabelText 0x44de00 and INV_ClampGroundLabelToScreen 0x4bcf20 of
// Game.exe 1.14b. VERIFIED from the code unless marked.

// MaxGroundLabels is the most labels per frame (the clamp routine aborts at 0x20).
const MaxGroundLabels = 32

// Engine text colour indexes used by the labels.
const (
	LabelWhite   = 0
	LabelBlue    = 3  // magic
	LabelGreen   = 2  // set
	LabelYellow  = 9  // rare
	LabelGold    = 4  // unique, quest items
	LabelOrange  = 8  // crafted, runes, essences, tokens, keys
	LabelGrey    = 5  // socketed or ethereal normal item
	LabelDarkGrn = 10 // tempered (quality 9)
)

// GroundItem is what the label colour depends on.
type GroundItem struct {
	Quality  int  // 1 low .. 9
	Socketed bool // item flag 0x800 (UNVERIFIED which flag name)
	Ethereal bool // item flag 0x400000
	Quest    bool // items.txt quest column non-zero
	Code     string
	IsRune   bool // ITEM_IsOfType 0x4a
}

var orangeCodes = map[string]bool{
	"ceh": true, "bet": true, "tes": true, "fed": true, "toa": true,
	"dhn": true, "bey": true, "mbr": true, "pk1": true, "pk2": true, "pk3": true,
}

// LabelColor is the colour index of a ground item label (UI_BuildGroundItemLabelText).
func LabelColor(it GroundItem) int {
	c := LabelWhite

	switch it.Quality {
	case 4:
		c = LabelBlue
	case 5:
		c = LabelGreen
	case 6:
		c = LabelYellow
	case 7:
		c = LabelGold
	case 8:
		c = LabelOrange
	case 9:
		c = LabelDarkGrn
	default:
		if it.Socketed || it.Ethereal {
			c = LabelGrey
		}
	}

	if it.Quest && it.Code != "leg" {
		c = LabelGold
	}

	if orangeCodes[it.Code] || it.IsRune {
		c = LabelOrange
	}

	return c
}

// LabelVisible reports whether an item at screen position (x, y) gets a label:
// x within the view (the mini map modes narrow it: 0..W/2-0x20 for display mode 1 and
// W/2..W for mode 2), y above -9 exclusive and up to H-0x10.
func LabelVisible(x, y, viewW, viewH, displayMode int) bool {
	lo, hi := 0, viewW

	switch displayMode {
	case 1:
		hi = viewW/2 - 0x20
	case 2:
		lo = viewW / 2
	}

	return x >= lo && x <= hi && y > -9 && y <= viewH-0x10
}

// LabelBox is a label rectangle (inclusive).
type LabelBox struct{ L, T, R, B int }

// LabelRect is the first box of a label before clamping: text width + 8 centred on the
// item x, the bottom at the item y, the top textHeight above plus 4.
func LabelRect(itemX, itemY, textW, textH int) LabelBox {
	w := textW + 8
	l := itemX - (w >> 1)

	return LabelBox{L: l, T: itemY - textH + 4, R: l + w, B: itemY}
}

// ClampLabel keeps a box inside the screen (left/top margin 6, right edge below viewR,
// bottom at most H-0x30) the way the first part of INV_ClampGroundLabelToScreen does.
func ClampLabel(b LabelBox, viewL, viewR, viewH int) LabelBox {
	if b.L < viewL {
		b.R += viewL - b.L
		b.L = 6
	}

	if b.T < 6 {
		b.B += 6 - b.T
		b.T = 6
	}

	if b.R >= viewR {
		d := b.R - viewR + 1
		b.R -= d
		b.L -= d
	}

	if lim := viewH - 0x30; b.B >= lim {
		d := b.B - lim
		b.B -= d
		b.T -= d
	}

	return b
}
