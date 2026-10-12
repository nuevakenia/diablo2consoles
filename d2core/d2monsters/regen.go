package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monstats"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// statHPRegen is item stat 0x4a (hpregen), the monster life regeneration per 25 Hz frame in 1/256 life points.
const statHPRegen = 0x4a

// initRegen sets stat 0x4a of a new monster from the monstats DamageRegen column (0x00571af0: maximum life in
// 1/256 points times DamageRegen, shifted right by 12). A pack leader gets it zeroed afterwards by the modifier
// init (0x59e960, d2monreg hpMultiply). VERIFIED 0x00571f0c / 0x005a4340.
func (d *Director) initRegen(u *unit, r *d2records.MonStatRecord) {
	if u.ally != nil || r == nil {
		return
	}

	v := d2monstats.RegenStat(u.m.Vitals.MaxHP<<8, r.HealthRegenPerFrame)
	if v == 0 {
		return
	}

	if u.stats == nil {
		u.stats = map[int]int{}
	}

	u.stats[statHPRegen] = v
}

// regenStep is MONSTER_LifeRegenEventTick (0x005a4340) for a living monster: every frame the life stat (1/256
// points) grows by stat 0x4a, up to the maximum. UNVERIFIED and not modelled: state 0x34 (blocks positive
// regeneration), the mask 0x14c states that drop the base value (poison, burn, open wounds are not stats here).
func (d *Director) regenStep(u *unit) {
	r := u.stats[statHPRegen]
	if r <= 0 || !u.m.Alive() {
		return
	}

	v := &u.m.Vitals
	if v.HP >= v.MaxHP {
		u.regenFrac = 0

		return
	}

	life := v.HP<<8 + u.regenFrac + r
	if life >= v.MaxHP<<8 {
		v.HP, u.regenFrac = v.MaxHP, 0

		return
	}

	v.HP, u.regenFrac = life>>8, life&0xff
}
