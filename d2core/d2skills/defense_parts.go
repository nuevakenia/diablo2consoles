package d2skills

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// heroDefenseParts is the Director's typed HeroDefenseParts hook (wave 22): the exe's absorb callbacks run
// on the damage struct type by type, so the port hands them the split hit instead of one summed number.
//   - Bone Armor (auraevent 22, 0x5c4300, d2skill.BoneArmorAbsorb): only the physical part is soaked by the pool.
//   - Energy Shield (0x5c8840, d2skill.EnergyShieldWalk): physical, fire, lightning, cold, magic one after
//     the other with the mana left after the earlier types; poison is not in the table. The attacker of a
//     Director hit is always a monster, so none of the three player-only-skipped slots is skipped (they stay
//     0: UNVERIFIED which damage kinds fill them).
//
// Thorns reflects what is left afterwards, as before. The returned note is empty (the log line is emitted here).
func (e *Engine) heroDefenseParts(p *d2mapentity.Player, attacker *d2mapentity.Monster, melee bool, hp *d2monsters.HitParts) string {
	set := e.setOf(p.ID())
	note := ""

	// Bone Armor: the stat is in 8.8; the pool maximum is not tracked (display only), the pool stands in.
	if pool := set.Stat(e.frame, "bonearmor"); pool > 0 && hp.Phys > 0 {
		left, poolLeft, _, _ := d2skill.BoneArmorAbsorb(hp.Phys<<8, pool, pool)
		taken := set.Drain(e.frame, "bonearmor", pool-poolLeft) >> 8
		hp.Phys = left >> 8
		note += fmt.Sprintf(" absorbed_by_armor=%d", taken)
	}

	if pct := set.Stat(e.frame, "x_energyshield_pct"); pct > 0 && hp.Total() > 0 {
		ratio := set.Stat(e.frame, "x_energyshield_ratio")
		in := [8]int{hp.Phys, hp.Fire, hp.Light, hp.Cold, hp.Magic}
		abs, manaLeft, ended := d2skill.EnergyShieldWalk(in, pct, ratio, p.Stats.Mana, false)

		total := 0
		for _, a := range abs {
			total += a
		}

		if total > 0 || ended {
			cost := p.Stats.Mana - manaLeft
			hp.Phys -= abs[0]
			hp.Fire -= abs[1]
			hp.Light -= abs[2]
			hp.Cold -= abs[3]
			hp.Magic -= abs[4]
			p.Stats.Mana = manaLeft
			note += fmt.Sprintf(" absorbed_by_shield=%d mana=%d", total, cost)

			if ended {
				p.Stats.Mana = 0
				set.Remove("energyshield")
				note += " shield_ended"
			}
		}
	}

	dmg := hp.Total()

	if pct := set.ThornsPct(e.frame); pct > 0 && melee && dmg > 0 {
		back := dmg * pct / 100
		e.monsters.Damage(attacker, back, p)
		note += fmt.Sprintf(" thorns=%d", back)
	}

	if note != "" {
		e.emit("damage", "DEFENSE hero=%s%s dmg_through=%d", p.Name(), note, dmg)
	}

	return ""
}
