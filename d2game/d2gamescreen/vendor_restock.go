package d2gamescreen

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"

// advanceVendorRestock runs the town-leaving vendor restock check. In the
// original the check (0x534c20) is tail-called from the town transition
// (SERVER_HandleTownTransition 0x534d40) with the act of the town being left:
// it flags the vendors of that act whose stock is older than four minutes, and
// the flag is consumed the next time the vendor window opens. Entering a town
// or staying in it never restocks (VERIFIED, d2-re-notes/verify-vendor.md).
func (v *Game) advanceVendorRestock() {
	cur := v.currentLevel()
	prev := v.restockLevel
	v.restockLevel = cur

	if prev == 0 || prev == cur || !d2level.IsTown(prev) || v.gameControls == nil || v.gameControls.Trade == nil {
		return
	}

	v.gameControls.Trade.LeaveTown(d2level.ActOfLevel(prev) - 1)
}
