package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// Wave 20, Druid summons: the cycling vines of Cycle of Life and Vines.
//
// The monsters of both skills (cycleoflife, vinecreature; AI CycleOfLife) carry
// one monstats skill only, CorpseCycler / VineCycler (V, MonStats rows): they
// never use Vine Attack. Their think function (d2monster thinkCycleOfLife,
// 0x5eb9c0) casts the cycler on a corpse when the owner is missing life
// (class 0x1aa) or mana, with a gap of aip1 frames (50) between two casts and
// an idle sleep of aip3 (25) when there is nothing to do. The start function
// SRVST_063 (0x5d1090) sets state 0x76 on the corpse (each corpse feeds one
// cycle) and creates the recycler delay missile, whose SrvDoFunc 29 / 33
// restores calc1 percent of the owner's maximum life / mana 45 frames later
// (d2missile/vines.go). Only the Plague Poppy attacks (Vine Attack).
//
// UNVERIFIED: the vine walks to its corpse in the exe (WalkTo, melee range);
// the engine's vines stand still, so every corpse within the cycler's
// aurarangecalc (10 subtiles) of the vine counts as reachable.

const (
	cyclerGap   = 50 // aip1: frames between two casts
	cyclerSleep = 25 // aip3: idle sleep
)

// RestorePct gives the hero pct percent of its maximum life (or mana) and
// reports whether anything was missing (d2missile.Owner.Restore; MulDiv of
// whole points as in 0x5ad910, clamped to the maximum).
func (h *heroUnit) RestorePct(mana bool, pct int) bool {
	if h.merc != nil || h.p == nil || h.p.Stats == nil {
		return false
	}

	st := h.p.Stats

	if mana {
		if st.Mana >= st.MaxMana {
			return false
		}

		st.Mana = minInt(st.Mana+st.MaxMana*pct/100, st.MaxMana)

		return true
	}

	if st.Health >= st.MaxHealth {
		return false
	}

	st.Health = minInt(st.Health+st.MaxHealth*pct/100, st.MaxHealth)

	return true
}

// cycleNeeds is the owner gate of thinkCycleOfLife: the life cycler wants the
// owner below its maximum life, the mana cycler below its maximum mana.
func cycleNeeds(h *heroUnit, mana bool) bool {
	if h.p == nil || h.p.Stats == nil {
		return false
	}

	if mana {
		return h.p.Stats.Mana < h.p.Stats.MaxMana
	}

	return h.p.Stats.Health < h.p.Stats.MaxHealth
}

// cycleTick runs one think of a cycling vine (see the comment above).
func (e *Engine) cycleTick(t *trapRun) {
	sk := e.pipe.Skills.ByName(t.cycler)
	if sk == nil {
		return
	}

	missile := e.pipe.Missiles.ByName(sk.SrvMissileA)
	if missile == nil {
		return
	}

	mana := missile.SrvDoFunc == 33

	if !cycleNeeds(t.u, mana) {
		t.next = e.frame + cyclerSleep
		return
	}

	r := d2skill.NewEnv(sk, t.u.SkillLevel(t.skillID), t.u, e.pipe.Skills).Eval(sk.AuraRangeCalc)
	if r < 5 {
		r = 5
	}

	tx, ty := t.m.SubtilePos()

	var best string

	bd := r + 1

	for _, c := range e.monsters.Corpses() {
		if e.isLooted(c.ID()) {
			continue
		}

		cx, cy := c.SubtilePos()
		if d := chebyshev(cx-tx, cy-ty); d < bd {
			best, bd = c.ID(), d
		}
	}

	if best == "" {
		t.next = e.frame + cyclerSleep
		return
	}

	m := e.pipe.CastCycler(t.u, t.cycler, t.u.SkillLevel(t.skillID))
	if m == nil {
		t.next = e.frame + cyclerSleep
		return
	}

	e.markLooted(best)
	t.next = e.frame + cyclerGap
	e.emit("summon", "CYCLER skill=%q cycler=%s corpse=%s restore_pct=%d", t.skillName, t.cycler, shortID(best), m.RestorePct)
}
