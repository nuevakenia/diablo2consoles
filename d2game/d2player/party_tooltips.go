package d2player

import "fmt"

// Party screen button tooltips: UI_DrawPartyScreenButtonTooltips 0x4977f0 shows, per relation bit of a row, a
// pair of string ids while the bit is clear and another pair while it is set (VERIFIED in the exe):
//
//	bit 08 (hostility): 0x100a,0x100b clear / 0x100c,0x100d set
//	bit 01 (corpse loot toggle): 0x100e,0x100f clear / 0x1010,0x1011 set
//	bit 02 (hearing): 0x1014,0x1015 clear / 0x1012,0x1013 set
//	bit 04 (squelch): 0x1016 clear / 0x1017 set
//
// In string.tbl the strParty<n> keys sit at id 0x1004+n-1 for n up to 19 (checked against a real string.tbl; the
// squelch-set text 0x1017 is key 22), so the pairs map onto the keys below. "Active" in the switch widgets is the clear-bit text.
const partyStringIDBase = 0x1004 - 1 // id of strParty<n> == partyStringIDBase + n

// partyTooltipKeys are the strParty numbers of a tooltip (one or two lines).
type partyTooltipKeys []int

var (
	partyHostileClear = partyTooltipKeys{7, 8}   // ids 0x100a, 0x100b
	partyHostileSet   = partyTooltipKeys{9, 10}  // ids 0x100c, 0x100d
	partyLootClear    = partyTooltipKeys{11, 12} // ids 0x100e, 0x100f
	partyLootSet      = partyTooltipKeys{13, 14} // ids 0x1010, 0x1011
	partyHearClear    = partyTooltipKeys{17, 18} // ids 0x1014, 0x1015
	partyHearSet      = partyTooltipKeys{15, 16} // ids 0x1012, 0x1013
	partySquelchClear = partyTooltipKeys{19}     // id 0x1016
	partySquelchSet   = partyTooltipKeys{22}     // id 0x1017 (the table's key numbering jumps from 19 to 22: not linear here)
)

// ids returns the string ids of the keys.
func (k partyTooltipKeys) ids() []int {
	out := make([]int, len(k))
	for i, n := range k {
		out[i] = partyStringIDBase + n
	}

	return out
}

// text joins the lines through lookup (a TranslateString).
func (k partyTooltipKeys) text(lookup func(interface{}) string) string {
	s := ""

	for i, n := range k {
		if i > 0 {
			s += "\n"
		}

		s += lookup(fmt.Sprintf("strParty%d", n))
	}

	return s
}
