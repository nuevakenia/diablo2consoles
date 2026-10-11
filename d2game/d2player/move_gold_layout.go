package d2player

// Layout of the gold dialog of the original (Game.exe 1.14b): the small box that opens when the coin button of
// the inventory (or the stash, or the trade window) is clicked. Found in this wave: UI_OpenPanelByIndex 0x44f7b0
// (the argument is the dialog kind), reached from the mouse-up handler of the inventory at 0x485d17 (kind 3,
// cursor empty, coin button) and 0x485d3a (kind 4, stash coin button).
// The dialog is created with UI_CreateBackgroundDialog 0x4b45a0 (picture "dialogbackground", frame 0), drawn by
// UI_DrawDialog 0x4b4a10 (picture standing on its bottom edge nY + height), its controls are made in the same
// function. All numbers below are screen pixels of the 800x600 screen (the dialog does not use the panel offset).
//
// Kinds (argument of 0x44f7b0, verified in the jump table and the string loads):
//
//	0, 1  drop gold            text 0xfc1, amount limit = carried gold (stat 14)
//	2     gold offer in trade  text 0xfce, amount limit = carried gold; OK sends the trade offer (0x4b5a60)
//	3     bank deposit         text 0xfd1, limit = carried gold (stat 14); OK sends packet 0x4f with arg 0x13
//	4     bank withdraw        text 0xfd2, limit = stash gold (stat 15); OK sends packet 0x4f with arg 0x14
//
// OK on kinds 0/1 sends packet 0x50 with the amount (server SERVER_HandleC2S50_DropPlayerGold 0x54a620).
// The amount typed is limited to the stat value and 10 digits.

// GoldDlgKind is the argument of the opener.
type GoldDlgKind int

// Gold dialog kinds.
const (
	GoldDlgDrop GoldDlgKind = iota
	GoldDlgDrop2
	GoldDlgTrade
	GoldDlgDeposit
	GoldDlgWithdraw
)

// Constants of the dialog (verified in the call arguments of 0x44f7b0).
const (
	goldDlgX, goldDlgY    = 0xd7, 0x8c // nX, nY of the dialog (picture top = nY, bottom = nY + frame height)
	goldDlgSpinnerX       = 0xdf       // spinner control (up/down arrows) anchor; size UNVERIFIED
	goldDlgSpinnerY       = 0xdb
	goldDlgEditX          = 0x102 // edit box anchor; width 100, 10 digits
	goldDlgEditY          = 0xe4
	goldDlgEditW          = 0x64
	goldDlgEditDigits     = 10
	goldDlgOkX            = 0xfa  // button type 0 (frame 0x10 of the buy/sell button picture, 0x11 pressed)
	goldDlgOkY            = 0x11f // both buttons
	goldDlgCancelX        = 0x163 // button type 1 (frame 0x0a, 0x0b pressed)
	goldDlgTextWidth      = 200   // text wrapped to this width (D2WIN_WrapTextToWidth)
	goldDlgTextRowAdvance = 0x18  // each wrapped line is a dialog entry (type 4), row advance 0x18
	goldDlgButtonSize     = 32    // UNVERIFIED: size of the OK / cancel pictures (assumed like the party close button)
)

// GoldDlgLayout is the result: top left rectangles (the original anchors pictures by the bottom left).
type GoldDlgLayout struct {
	Frame, Spinner, Edit, Ok, Cancel UIRect
	TextBaseline0                    int // baseline y of the first text line; the next lines add RowStep
	TextWidth, EditDigits, RowStep   int
	TextID                           int // string.tbl id
	Packet, PacketArg                int // sent by OK (0 when the kind has none, trade uses its own packet)
	StatLimit                        int // stat that limits the amount: 14 carried, 15 banked
}

// GoldDialog returns the layout of the dialog for a picture of frameW x frameH (the size of frame 0 of
// "dialogbackground", read from the game file at run time). The edit box height and the spinner size are not
// known from the exe (UNVERIFIED): 20 high edit box and 16x30 spinner are placeholders for the hit areas.
func GoldDialog(kind GoldDlgKind, frameW, frameH int) GoldDlgLayout {
	btnTop := goldDlgOkY - goldDlgButtonSize

	l := GoldDlgLayout{
		Frame:         UIRect{"golddlg", "frame", goldDlgX, goldDlgY, frameW, frameH},
		Spinner:       UIRect{"golddlg", "spinner", goldDlgSpinnerX, goldDlgSpinnerY, 16, 30},
		Edit:          UIRect{"golddlg", "edit", goldDlgEditX, goldDlgEditY, goldDlgEditW, 20},
		Ok:            UIRect{"golddlg", "ok", goldDlgOkX, btnTop, goldDlgButtonSize, goldDlgButtonSize},
		Cancel:        UIRect{"golddlg", "cancel", goldDlgCancelX, btnTop, goldDlgButtonSize, goldDlgButtonSize},
		TextBaseline0: goldDlgY + goldDlgTextRowAdvance,
		TextWidth:     goldDlgTextWidth, EditDigits: goldDlgEditDigits, RowStep: goldDlgTextRowAdvance,
		StatLimit: 14,
	}

	switch kind {
	case GoldDlgDrop, GoldDlgDrop2:
		l.TextID, l.Packet = 0xfc1, 0x50
	case GoldDlgTrade:
		l.TextID = 0xfce
	case GoldDlgDeposit:
		l.TextID, l.Packet, l.PacketArg = 0xfd1, 0x4f, 0x13
	case GoldDlgWithdraw:
		l.TextID, l.Packet, l.PacketArg, l.StatLimit = 0xfd2, 0x4f, 0x14, 15
	}

	return l
}
