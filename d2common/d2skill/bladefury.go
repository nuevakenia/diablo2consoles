package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"

// Blade Fury upkeep (w23). SRVST_026 0x5d53c0 + SRVDO_048 0x5d5260, VERIFIED in this wave:
//
//   - The first use with no state 0xc needs startmana*256 <= mana, creates state 0xc expiring at frame+21, resets
//     the skill's target-type slot to 0 and returns WITHOUT a throw and without paying.
//   - Every later use (state present) moves the expiry to frame+7. If the slot is <= frame the do function runs:
//     it needs prgcalc1-1 > 0, clears unit flag 0x40, needs a target position, and throws one progressive
//     missile only when slot < frame (strictly); the slot then becomes frame+prgcalc1-1. The start function pays
//     the mana cost only when the do function threw. If the slot is still ahead, the action events are pushed
//     back one frame and nothing else happens (the port refuses the repeat with the cooldown reason).
//
// The slot is the unit's cooldown field, the state is bfState (unit id to expiry frame).
const (
	bladeFuryFirstFrames  = 21
	bladeFuryRefreshFrame = 7
)

func isBladeFury(sk *Skill) bool { return sk.SrvStFunc == 26 && sk.SrvDoFunc == 48 }

// bladeFuryActive reports whether the unit holds state 0xc.
func (p *Pipeline) bladeFuryActive(u Unit) bool {
	return p.bfState[u.ID()] > p.frame()
}

// bladeFuryRefresh moves the expiry of state 0xc to frame+7 (STATS_SetStatListExpireFrame in SRVST_026).
func (p *Pipeline) bladeFuryRefresh(u Unit) {
	if p.bfState == nil {
		p.bfState = map[string]int{}
	}

	p.bfState[u.ID()] = p.frame() + bladeFuryRefreshFrame
}

// bladeFuryDo is the whole of Blade Fury's action frame (the exe start function plus its do function).
func (p *Pipeline) bladeFuryDo(u Unit, sk *Skill, lvl int, tgt Target) DoResult {
	f := p.frame()
	res := DoResult{OK: true, Level: lvl}

	if !p.bladeFuryActive(u) {
		if sm := sk.StartMana; sm > 0 && u.IsPlayer() && u.Mana() < sm*256 {
			return DoResult{Reason: ReasonMana, Level: lvl}
		}

		if p.bfState == nil {
			p.bfState = map[string]int{}
		}

		p.bfState[u.ID()] = f + bladeFuryFirstFrames
		u.SetCooldown(sk.ID, 0)

		return res
	}

	p.bladeFuryRefresh(u)

	cost := costOf(u, sk, lvl)
	if u.IsPlayer() && cost > u.Mana() {
		return DoResult{Reason: ReasonMana, Level: lvl}
	}

	env := p.env(sk, lvl, u)
	slot := u.Cooldown(sk.ID)

	// the start function lets slot <= frame through, the do function throws only when slot < frame; a slot still
	// ahead only pushes the events back
	if slot >= f {
		return res
	}

	p.runDo(u, sk, lvl, tgt, env, &res)

	if !res.OK {
		return res
	}

	if len(res.Missiles) > 0 && u.IsPlayer() && cost > 0 {
		rest, _ := d2combat.PayMana(u.Mana(), cost)
		u.SetMana(rest)
		res.ManaPaid = cost
	}

	return res
}
