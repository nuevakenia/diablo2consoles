package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 24: the shield block of a monster defender for every hit that goes through the attack outcome
// (COMBAT_RollAttackOutcome 0x57cc10): skill melee strikes, missiles of heroes and the strikes of allies. The roll
// itself is COMBAT_RollShieldBlock 0x57bfa0 (d2combat.RollShieldBlock): one step of the DEFENDER's seed, after a
// successful to-hit roll, chance = stat 0x14 gated by COMBAT_GetBlockChance 0x6228d0 (MonsterVitals.Block).

// blockNoAnimClasses are the monster classes that never play the block animation (COMBAT_ServerHandleUnitHit
// 0x57ae50, VERIFIED): they only get the data field 54 set instead.
var blockNoAnimClasses = map[int]bool{0xf3: true, 0x14d: true, 0x2c1: true}

// MonsterBlock rolls the shield block of monster m against a hit that already passed the to-hit roll and, when it
// blocks, runs the reaction of 0x57ae50: the monster plays its block animation (mode BL) when its class has one and
// is not one of 0xf3, 0x14d, 0x2c1 (those only get the silent mode flag), without the hit recovery gate and
// without damage. Reports whether the hit was blocked. missile says whether the hit is a missile's.
//
// VERIFIED: block rolled on the defender's seed (u.b.Seed), only for a living monster whose mode is neither
// dying (0) nor dead (0xc); the chance is not divided for monsters (the /3 is for running players only).
// UNVERIFIED: that a monster in the middle of its own attack animation is interrupted by BL (the exe issues an AI
// command, which the Go brain approximates with the same drop as hit recovery).
func (d *Director) MonsterBlock(m *d2mapentity.Monster, missile bool) bool {
	u := d.byEntity[m.ID()]
	if u == nil || u.mirror || !m.Alive() || m.Vitals.Block < 1 {
		return false
	}

	if md := u.b.Mode; md == d2monster.ModeDying || md == d2monster.ModeDead {
		return false
	}

	if !d2combat.RollShieldBlock(u.b.Seed, m.Vitals.Block, false) {
		return false
	}

	d.Counters.MonsterBlocks++
	d.emit("block", "MONSTER block name=%s id=%d chance=%d missile=%v", m.Label(), u.b.ID, m.Vitals.Block, missile)

	if blockNoAnimClasses[m.Stat.ID] || !m.HasMode(d2monster.ModeBlock) {
		return true // setmode 0x13 / data field 54: nothing visible
	}

	u.m.StopMoving()
	u.m.DropHitEvents()
	u.mv = nil

	if u.m.SetMode(d2monster.ModeBlock) {
		u.b.Wake = 1 << 30
		d.Counters.BlockAnimsMonster++
	} else {
		u.b.WakeNow(d.frame)
	}

	return true
}

// HeroPvPBlock rolls the defender-side shield block of the local hero against a hit of a hostile player (the
// attacker's client has already passed the to-hit roll). The hero rolls on her own generator (the defender's seed,
// VERIFIED order: after the to-hit roll, before the damage); a hero who is running has a third of her chance
// (heroAutoHit, COMBAT_RollShieldBlock 0x57bfa0 with helper 0x622ef0). The note carries the block animation
// cooldown (noteBlockAnim). UNVERIFIED: that the real exe resolves PvP in one process; here the attacker's client
// sends the hit and this runs on the defender.
func (d *Director) HeroPvPBlock(p *d2mapentity.Player) (blocked bool, note string) {
	t := p.Stats.Totals
	if t == nil || t.BlockPct < 1 {
		return false, ""
	}

	if !d2combat.RollShieldBlock(d.heroRoller(), t.BlockPct, heroAutoHit(p)) {
		return false, ""
	}

	return true, d.noteBlockAnim(p)
}
