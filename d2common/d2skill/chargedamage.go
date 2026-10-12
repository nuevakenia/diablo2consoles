package d2skill

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"

// Charge-up damage of the assassin finishers (w23). SRVDO_AssassinFinisherBuildDamage 0x5d3d90 runs, for every
// kick or blow it builds (Dragon Talon 0x5d4190, Dragon Claw 0x5d4ba0, Dragon Flight 0x5d6290), the charge-state
// adders, VERIFIED:
//
//   - Type 1 first (SKILL_ApplyChargeStatesType1ToDamage 0x5d2180, builder prgdam = 1, Tiger Strike): the damage
//     percent of the struct grows by calc1 of the builder (at its level) times the charge count (1..3). The
//     builder's target overlay is put on the target.
//   - After the damage is built, types 2 to 4 (0x5d2240) for every held charge state of a builder with that prgdam:
//     * 2 (Cobra Strike, 0x5d1e60): v = SKILLS_GetBasePlusPerLevelValue (Param1 + (level-1)*Param2); count 1 adds v
//       to the life leech of the struct, count 2 adds v to life and mana leech, count 3 adds 2v to both.
//     * 4 (Fists of Fire, Claws of Thunder, Blades of Ice, 0x5d2030): the builder's elemental skill damage is
//       rolled and added; the result gets flag 0x4000; a positive calc1 moves calc1 percent of the physical
//       damage (at most all of it) into the builder's element. For cold at count 3 a freeze length term is added
//       (not ported, UNVERIFIED: the divisor is a record field that was not decoded).
//     * 3 (0x5d1f40) is the same as 4 without the calc1 conversion; no skill uses it.
//   - The release (0x5d3ae0, see chargerelease.go) happens when the kick that carries the bonus has hit, so a
//     later kick of the same Dragon Talon no longer has the charges. The port gives the bonus to every blow up to
//     and including the first one that hit.

// chargeBuilders lists the held charge states with their builder skill and count.
type chargeHeld struct {
	b     *Skill
	count int
}

func (c *cast) heldCharges() []chargeHeld {
	var out []chargeHeld

	for _, st := range chargeStats {
		n := c.u.Stat(st)
		if n < 1 {
			continue
		}

		if n > 3 {
			n = 3
		}

		if b := c.p.builderOf(st); b != nil {
			out = append(out, chargeHeld{b: b, count: n})
		}
	}

	return out
}

// chargePct is the type 1 bonus: calc1 of each prgdam 1 builder times its count (percent of the damage).
func (c *cast) chargePct(held []chargeHeld) int {
	pct := 0

	for _, h := range held {
		if h.b.PrgDam != 1 {
			continue
		}

		lvl := maxInt(c.u.SkillLevel(h.b.ID), 1)
		pct += c.p.env(h.b, lvl, c.u).eval(h.b.Calc[1]) * h.count
	}

	return pct
}

// chargeBasePlusPerLevel is SKILLS_GetBasePlusPerLevelValue: Param1 + (level-1)*Param2.
func chargeBasePlusPerLevel(sk *Skill, lvl int) int { return sk.Params[1] + (lvl-1)*sk.Params[2] }

// applyChargeDamage adds the type 2 to 4 adders to a blow that hit.
func (c *cast) applyChargeDamage(m *MeleeResult, held []chargeHeld) {
	if m == nil || !m.Hit {
		return
	}

	for _, h := range held {
		lvl := maxInt(c.u.SkillLevel(h.b.ID), 1)

		switch h.b.PrgDam {
		case 2:
			v := chargeBasePlusPerLevel(h.b, lvl)

			switch h.count {
			case 1:
				m.Damage.LifeLeech += int32(v)
			case 2:
				m.Damage.LifeLeech += int32(v)
				m.Damage.ManaLeech += int32(v)
			default:
				m.Damage.LifeLeech += int32(2 * v)
				m.Damage.ManaLeech += int32(2 * v)
			}
		case 3, 4:
			env := c.p.env(h.b, lvl, c.u)
			d := h.b.Descriptor(env, lvl, 0, 0, c.u.Stat(masteryStat[h.b.EType]))
			el := d.Roll(c.u.Roller())

			m.Damage.Fire += el.Fire
			m.Damage.Lightning += el.Lightning
			m.Damage.Magic += el.Magic
			m.Damage.Cold += el.Cold
			m.Damage.ColdLen += el.ColdLen
			m.Damage.Poison += el.Poison
			m.Damage.PoisonLen += el.PoisonLen
			m.Damage.Burn += el.Burn
			m.Damage.BurnLen += el.BurnLen
			m.Damage.StunLen += el.StunLen
			m.Damage.FreezeLen += el.FreezeLen

			if h.b.PrgDam == 4 {
				if pct := env.eval(h.b.Calc[1]); pct > 0 {
					moved := int32(mulDiv(int(m.Damage.Physical), pct, 100))
					if moved > m.Damage.Physical {
						moved = m.Damage.Physical
					}

					m.Damage.Physical -= moved
					addElement(&m.Damage, h.b.EType, moved)
				}
			}
		}
	}

	m.Total = m.Damage.SumTotal(false)
}

// addElement is SKILL_AddElementalToDamageStruct for the converted amount: the element named by the skill's etype.
// Poison and burn keep their own length rules and are not used by any builder (UNVERIFIED), they fall back to fire.
func addElement(d *d2combat.Damage, etype string, v int32) {
	switch etype {
	case "ltng":
		d.Lightning += v
	case "cold":
		d.Cold += v
	case "mag":
		d.Magic += v
	default:
		d.Fire += v
	}
}
