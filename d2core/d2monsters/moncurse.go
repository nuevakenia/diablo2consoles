package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Wave 36: the curses a boss casts on a hero (Summoner's Weaken, Baal's Defense Curse and Blood Mana: srvdofunc 30)
// and Duriel's Holy Freeze aura (srvdofunc 81, started by his think function through d2monster.AuraStarter).
//
// A curse puts the skill's aurastat pairs on every hero within the aura range of the target for the aura length, as
// timed hero stats (the path the Cursed monster modifier already uses, curseHero). The numbers come from skills.txt
// through d2skill.MonsterCurseOf (the calc columns at the monster's skill level). VERIFIED in the exe: the curse
// do function 0x5c1590 applies the aura target state with those stats. UNVERIFIED: the hero's curse length
// reduction (cursereduce) is not applied, the range is read in subtiles, Blood Mana's own effect (it has no
// aurastat pair, only a damage range and a state) is not modelled, and the heroes are the ones within range of the
// target (the exe scans the area of the aim point).

// landCurse applies the curse of the monster's skill. It returns true (the cast has no direct damage).
func (d *Director) landCurse(u *unit, rec *d2records.SkillRecord, lvl int) bool {
	reg := d.skillRegistry()
	if reg == nil {
		return true
	}

	sk := reg.ByID(rec.ID)
	c := d2skill.MonsterCurseOf(reg, sk, lvl, u.m.Vitals.Level)

	if c == nil || len(c.Stats) == 0 {
		d.emit("bossfx", "MONSTER curse name=%s id=%d skill=%q stats=0 (no aurastat pair: state %q only)", u.m.Label(), u.b.ID,
			rec.Skill, sk.AuraTargetState)

		return true
	}

	ax, ay := d.skillAim(u)
	radius := c.Radius

	if radius < 1 {
		radius = 1
	}

	n := 0

	for _, tid := range d.targetIDs() {
		p := d.targets[tid]
		if !d.targetable(p) {
			continue
		}

		px, py := playerSubtile(p)
		if (px-ax)*(px-ax)+(py-ay)*(py-ay) > radius*radius {
			continue
		}

		d.curseHero(p, c.Stats, c.Frames)

		n++
	}

	d.Counters.Curses += n
	d.emit("bossfx", "MONSTER curse name=%s id=%d skill=%q state=%q radius=%d frames=%d stats=%v heroes=%d", u.m.Label(), u.b.ID,
		rec.Skill, c.State, radius, c.Frames, c.Stats, n)

	return true
}

// monAura is a running aura of a monster: the skill slot, its level and the enemy stats it puts on heroes.
type monAura struct {
	slot, level int
	range_      int
	stats       []d2skill.StatMod
	period      int
	next        int
}

const auraRefresh = 25 // frames between two applications of an aura on the heroes in range (UNVERIFIED: perdelay is 50)

// StartAura implements d2monster.AuraStarter: Duriel's Holy Freeze becomes his active skill. The caster takes the
// passive pairs on its own state (cold damage, d2skill.MonsterAuraStats, VERIFIED shape of SRVDO_081 0x5cee90), and
// every auraRefresh frames the heroes within the aura range take the aura's enemy stats (slow, attack rate) clamped
// by the cold effect of the target (d2state.ClampAuraSlow, VERIFIED 0x5cd240: -50 for a non-monster).
func (d *Director) StartAura(b *d2monster.Brain, slot, level int) {
	u := d.unitOf(b)
	if u == nil || u.skill == nil && b.Profile == nil {
		return
	}

	s := b.Profile.Skills[slot]
	rec := d.asset.Records.GetSkillByName(s.Name)
	reg := d.skillRegistry()

	if rec == nil || reg == nil {
		return
	}

	lvl := s.Level
	if lvl < 1 {
		lvl = 1
	}

	sk := reg.ByID(rec.ID)
	a := &monAura{slot: slot, level: lvl, period: auraRefresh}

	if c := d2skill.MonsterCurseOf(reg, sk, lvl, u.m.Vitals.Level); c != nil {
		a.range_, a.stats = c.Radius, c.Stats
	}

	for _, sm := range d2skill.MonsterAuraStats(reg, sk, lvl, u.m.Vitals.Level) {
		switch sm.Stat {
		case "coldmindam":
			addAuraElem(u, "cold", sm.Value, true)
		case "coldmaxdam":
			addAuraElem(u, "cold", sm.Value, false)
		}
	}

	if d.fx.auras == nil {
		d.fx.auras = map[uint32]*monAura{}
	}

	d.fx.auras[b.ID] = a
	d.emit("bossfx", "MONSTER aura name=%s id=%d skill=%q level=%d range=%d enemy_stats=%v", u.m.Label(), b.ID, s.Name, lvl, a.range_, a.stats)
}

// addAuraElem adds a minimum or maximum to the cold element of the monster's blows.
func addAuraElem(u *unit, typ string, v int, isMin bool) {
	el := &u.m.UMod.Elems

	for i := range *el {
		if (*el)[i].Type == typ {
			if isMin {
				(*el)[i].Min += v
			} else {
				(*el)[i].Max += v
			}

			return
		}
	}

	e := d2mapentity.UModElem{Type: typ}
	if isMin {
		e.Min = v
	} else {
		e.Max = v
	}

	*el = append(*el, e)
}

// HasRightSkill implements d2monster.RightSkillChecker: UNIT_GetRightSkill(unit) != NULL once the aura is active.
func (d *Director) HasRightSkill(b *d2monster.Brain) bool {
	_, ok := d.fx.auras[b.ID]

	return ok
}

// stepAuras applies the running auras of living monsters to the heroes in range.
func (d *Director) stepAuras() {
	for id, a := range d.fx.auras {
		u := d.units[id]
		if u == nil || !u.m.Alive() || u.m.Vitals.HP <= 0 {
			delete(d.fx.auras, id)

			continue
		}

		if d.frame < a.next {
			continue
		}

		a.next = d.frame + a.period
		sx, sy := u.m.SubtilePos()
		stats := d2state.ClampAuraSlow(toStateMods(a.stats), -50)

		for _, tid := range d.targetIDs() {
			p := d.targets[tid]
			if !d.targetable(p) {
				continue
			}

			px, py := playerSubtile(p)
			if (px-sx)*(px-sx)+(py-sy)*(py-sy) > a.range_*a.range_ {
				continue
			}

			d.curseHero(p, fromStateMods(stats), a.period+1)
		}
	}
}

func toStateMods(in []d2skill.StatMod) []d2state.StatMod {
	out := make([]d2state.StatMod, len(in))
	for i, m := range in {
		out[i] = d2state.StatMod{Stat: m.Stat, Value: m.Value}
	}

	return out
}

func fromStateMods(in []d2state.StatMod) []d2skill.StatMod {
	out := make([]d2skill.StatMod, len(in))
	for i, m := range in {
		out[i] = d2skill.StatMod{Stat: m.Stat, Value: m.Value}
	}

	return out
}
