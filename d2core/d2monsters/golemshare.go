package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// shareGolemDamage is the damage event of the Blood Golem (event 26, function
// 0x5c4600, VERIFIED): the owner's life pays OwnerDamagePct percent of the
// blow and the golem takes that much less. The owner's life never drops below
// one point and a dead owner shares nothing.
func (d *Director) shareGolemDamage(tu *unit, dmg int) int {
	if tu.ally == nil || tu.ally.opt.OwnerDamagePct <= 0 || tu.ally.owner == nil || tu.ally.owner.Stats == nil {
		return dmg
	}

	st := tu.ally.owner.Stats
	life, left := d2skill.BloodGolemDamageShare(dmg, tu.ally.opt.OwnerDamagePct, st.Health)

	if life != st.Health {
		d.emit("minion", "GOLEM share owner_life=%d->%d golem_dmg=%d->%d", st.Health, life, dmg, left)
	}

	st.Health = life

	return left
}

// ShareHealWithGolem is ITEM_ShareHealingWithPlayerGolem (0x5c4800, VERIFIED):
// heal points are about to reach the owner; a living Blood Golem of the owner
// takes HealSharePct percent of them (limited by its missing life) and the
// owner receives that much less. It returns the heal the owner still gets.
func (d *Director) ShareHealWithGolem(owner *d2mapentity.Player, heal int) int {
	if heal <= 0 {
		return heal
	}

	for _, u := range d.sortedUnits() {
		if u.ally == nil || u.ally.owner != owner || u.ally.opt.HealSharePct <= 0 || !u.m.Alive() {
			continue
		}

		v := &u.m.Vitals

		gain := d2skill.BloodGolemHealShare(heal, u.ally.opt.HealSharePct, v.HP, v.MaxHP)
		v.HP += gain

		if gain > 0 {
			d.emit("minion", "GOLEM share heal=%d golem_gain=%d hp=%d/%d", heal, gain, v.HP, v.MaxHP)
		}

		return heal - gain
	}

	return heal
}
