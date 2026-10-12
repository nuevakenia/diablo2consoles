package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// applyChaseReach fills Profile.ChaseReach from monstats2 MeleeRng. VERIFIED
// (w23): the byte the Corrupt Lancer AI passes to "run to target within"
// (monstats2 record +0xe) is the MeleeRng column, the monstats2 loader table
// at 0x658fe6 pairs the name MeleeRng with offset 0xe.
func applyChaseReach(p *d2monster.Profile, stat *d2records.MonStatRecord, stats2 d2records.MonStats2) {
	if p == nil || stat == nil {
		return
	}

	if rec := stats2[stat.ExtraDataKey]; rec != nil {
		p.ChaseReach = rec.MeleeRng
	}
}
