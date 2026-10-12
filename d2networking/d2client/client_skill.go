package d2client

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Client side skill dispatch (skills.txt cltstfunc / cltdofunc, Game.exe
// SkillsAma.cpp / SkillsNec.cpp, notes in gaps-slice-F.md). The exe picks, per
// skill, which of the cltmissile columns to create, how many, and where; before
// this the client created every non empty column at the cast point.

// clientShot is one missile to create: the record column, its start and aim point
// in subtiles.
type clientShot struct {
	slot int
	from d2skill.SubPoint
	to   d2skill.SubPoint
}

// cltSkillEnv evaluates skills.txt calc columns on the client: the skill level
// and Param1..6 of the row. Remote casters' levels are unknown to the client, so
// the caller passes level 1 for them (UNVERIFIED approximation).
type cltSkillEnv struct {
	d2calc.ZeroEnv
	level int
	rec   *d2records.SkillRecord
}

func (e cltSkillEnv) Field(code string) int {
	switch code {
	case "lvl":
		return e.level
	case "par1":
		return e.rec.Param1
	case "par2":
		return e.rec.Param2
	case "par3":
		return e.rec.Param3
	case "par4":
		return e.rec.Param4
	case "par5":
		return e.rec.Param5
	case "par6":
		return e.rec.Param6
	}

	return 0
}

// cltSlotName returns the missile name in a cltmissile column (0 base, 1 a .. 4 d).
func cltSlotName(rec *d2records.SkillRecord, slot int) string {
	switch slot {
	case 0:
		return rec.Cltmissile
	case 1:
		return rec.Cltmissilea
	case 2:
		return rec.Cltmissileb
	case 3:
		return rec.Cltmissilec
	case 4:
		return rec.Cltmissiled
	}

	return ""
}

// planClientShots decides the missiles of a cast from the skill's client start
// and do functions. caster and target are subtile positions. Skills without a
// dispatched function keep the old behaviour (every non empty column at the
// caster, aimed at the target).
func planClientShots(rec *d2records.SkillRecord, level int, caster, target d2skill.SubPoint) []clientShot {
	return planClientShotsWith(rec, level, caster, target, clientExtras{})
}

// clientExtras is what the client knows about the units around a cast (wave 24): the enemies it can see, so that the
// functions that scan for a second unit (Strafe CLTDO_020, Lightning Strike CLTDO_022) can draw their extra missiles.
// Without enemies the plan is the old single missile.
type clientExtras struct {
	Enemies []d2skill.ChainTarget
	// TargetID is the unit id of the enemy at the aim point (0 = none).
	TargetID int
}

// aimedEnemy finds the enemy the cast is aimed at: the one with TargetID, else the nearest to the aim point within
// two subtiles.
func (x clientExtras) aimedEnemy(target d2skill.SubPoint) (d2skill.ChainTarget, bool) {
	best, bd, ok := d2skill.ChainTarget{}, 3, false

	for _, e := range x.Enemies {
		if x.TargetID != 0 && e.ID == x.TargetID {
			return e, true
		}

		d := maxAbs(e.Pos.X-target.X, e.Pos.Y-target.Y)
		if x.TargetID == 0 && d < bd {
			best, bd, ok = e, d, true
		}
	}

	return best, ok
}

func maxAbs(a, b int) int {
	if a < 0 {
		a = -a
	}

	if b < 0 {
		b = -b
	}

	if b > a {
		return b
	}

	return a
}

// planClientShotsWith is planClientShots with the units the client can see.
func planClientShotsWith(rec *d2records.SkillRecord, level int, caster, target d2skill.SubPoint, ex clientExtras) []clientShot {
	env := cltSkillEnv{level: level, rec: rec}
	calc1 := func() int {
		if rec.Calc1.Empty() {
			return 1
		}

		return rec.Calc1.Eval(env)
	}

	var shots []clientShot

	add := func(ss ...d2skill.Shot) {
		for _, s := range ss {
			if cltSlotName(rec, s.Slot) != "" {
				shots = append(shots, clientShot{slot: s.Slot, from: s.From, to: s.To})
			}
		}
	}

	switch rec.Cltdofunc {
	case d2skill.CltDoMultipleShot:
		// weapon class of remote casters is unknown: assume a bow (column a)
		add(d2skill.MultiShotPlan(caster, target, calc1(), d2skill.MultiShotSlot(1, rec.Cltmissileb != ""))...)
	case d2skill.CltDoChargedStrike:
		add(d2skill.ChargedStrikePlan(caster, target, calc1(), 1)...)
	case d2skill.CltDoGuidedArrow:
		add(d2skill.GuidedArrowPlan(caster, target, false, 1))
	case d2skill.CltDoLightningStrike:
		// the primary strike bolt is drawn on the target; with a visible struck enemy the chain bolt of
		// CLTDO_022 (0x4f2340) follows: the scan around it picks the second unit (d2skill.LightningStrikePlan)
		add(d2skill.Shot{Slot: 1, From: target, To: target})

		if main, ok := ex.aimedEnemy(target); ok {
			calc2 := 0
			if !rec.Calc2.Empty() {
				calc2 = rec.Calc2.Eval(env)
			}

			if s, ok := d2skill.LightningStrikePlan(main, ex.Enemies, calc1(), calc2, 1); ok {
				add(s)
			}
		}
	case d2skill.CltDoStrafe:
		// the weapon class of remote casters is unknown: a bow (column a), see d2skill.StrafeSlot
		slot := d2skill.StrafeSlot(1, rec.Cltmissileb != "")

		if first, ok := ex.aimedEnemy(target); ok && !rec.Aurarangecalc.Empty() {
			rng := rec.Aurarangecalc.Eval(env)
			n := d2skill.StrafeBurst(calc1(), evalOr(rec.Calc3, env, 0), len(ex.Enemies))
			add(d2skill.StrafeClientBurst(caster, first, maxInt(n, 1), ex.Enemies, rng, slot)...)
		} else {
			add(d2skill.Shot{Slot: slot, From: caster, To: target})
		}
	case clientDoCurse:
		add(d2skill.Shot{Slot: 1, From: target, To: target})
	default:
		for slot := 0; slot <= 4; slot++ {
			add(d2skill.Shot{Slot: slot, From: caster, To: target})
		}

		return shots
	}

	// client start functions that add a cast effect on top of the do function
	switch rec.Cltstfunc {
	case clientStCurse:
		add(d2skill.Shot{Slot: 3, From: target, To: target}) // CLTST_018: Cltmissilec at the target
	case clientStTeeth:
		add(d2skill.Shot{Slot: 3, From: caster, To: target}) // CLTST_019: Cltmissilec from the caster
	}

	return shots
}

// cltdofunc / cltstfunc numbers of the curses (CLTST_018 / CLTDO_030) and Teeth (CLTST_019).
const (
	clientDoCurse = 30
	clientStCurse = 18
	clientStTeeth = 19
)

func evalOr(c *d2calc.Program, env d2calc.Env, def int) int {
	if c == nil || c.Empty() {
		return def
	}

	return c.Eval(env)
}

func maxInt(a, b int) int {
	if b > a {
		return b
	}

	return a
}
