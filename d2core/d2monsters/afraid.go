package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// FBXAfraidOf implements d2monster.FBXAfraid, the fear gate of the Zakarum Zealot (MONAI_Think_ZakarumZealot
// 0x5f5ef0, VERIFIED batch 9): the target is a hero, or a monster led by a hero (a summon or a converted unit,
// MONAI_GetLeaderIdAndType with owner type 0); when that hero is looked up, the zealot stands in Act III (the level
// id's act is 2 counted from zero) and the hero's quest record of the current difficulty has quest slot 21 bit 0
// (QUESTREC_GetFlag(record, 0x15, 0): slot 21 is the Blackened Temple, bit 0 its done flag), the zealot runs from
// the target. The Options hook answers the act and quest part; without it nothing is feared.
func (d *Director) FBXAfraidOf(_ *d2monster.Brain, t d2monster.Target) bool {
	if d.opt.ZealotAfraid == nil {
		return false
	}

	var p *d2mapentity.Player

	switch {
	case t.IsPlayer:
		p = d.playerFor(t.ID)
	case t.ID >= unitTargetBase:
		if tu := d.units[t.ID-unitTargetBase]; tu != nil && tu.ally != nil {
			p = tu.ally.owner
		}
	case t.ID >= mercTargetBase:
		if tu := d.units[t.ID-mercTargetBase]; tu != nil && tu.merc != nil {
			p = tu.merc.owner
		}
	}

	return p != nil && d.opt.ZealotAfraid(p)
}

// PlayerScaling implements d2monster.PlayerScaler, the fifth word of the block MONSTER_CalcPlayerCountScaling
// (0x571760) fills (VERIFIED batch 9: the effective player count of the game, the larger of the live count and the
// forced one). The exe stores 1 instead when the monster's monstats byte +0x4c is non-zero; w24 read the column
// table of the loader (MONTBL_LoadMonstatsTable, the "Align" entry has offset 0x4c): that byte is Align, so a unit
// whose class is not hostile (Align != 0) always reports 1. Diablo's decision reads it as "fewer than two players"
// (0x5e7710).
func (d *Director) PlayerScaling(b *d2monster.Brain) int {
	if b == nil {
		return d.PlayerCount()
	}

	if u := d.unitOf(b); u != nil && u.m != nil && u.m.Stat != nil && u.m.Stat.Alignment != 0 {
		return 1
	}

	return d.PlayerCount()
}
