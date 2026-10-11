package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2difficulty"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monreg"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The effect of the monster modifiers (monumod.txt) on a spawned pack leader and its group: the exe's init callbacks
// (d2monreg.FxCtx, VERIFIED) applied to the engine's vitals. The exe keeps everything in the unit's stat list; the
// engine keeps life, level, experience, defense and the attacks in Vitals and the rest in Monster.UMod.
//
// Mapping (engine choices, UNVERIFIED where they differ from a plain stat list): damage percent (stat 0x19) scales
// the minimum and maximum of A1, A2 and S1; to-hit percent (0x77) scales their to-hit; armor percent (0x10) scales
// the defense; the elemental damage stats become extra hits (UModBonus.Elems); poison is rate * length / 256.

// MonsterResistsOf is MonsterResists plus the resists the modifiers gave the monster.
func MonsterResistsOf(m *d2mapentity.Monster) [6]int {
	r := MonsterResists(m.Stat, m.Vitals.Difficulty)

	for i, v := range m.UMod.Resist {
		r[i] += v
	}

	return r
}

// fxResistStats maps the exe's resist stat ids to the index of MonsterResists.
var fxResistStats = map[int]int{0x24: 0, 0x25: 1, 0x27: 2, 0x29: 3, 0x2b: 4, 0x2d: 5}

// fxCtx builds the table view the callbacks read.
func (d *Director) fxCtx() *d2monreg.FxCtx {
	consts := d.asset.Records.Monster.Unique.Constants
	levels := d.asset.Records.Monster.Levels
	diff := int(d.opt.Difficulty)

	return &d2monreg.FxCtx{
		Difficulty: diff,
		Expansion:  !d.opt.Classic,
		Const: func(row int) int {
			if row < 0 || row >= len(consts) {
				return 0
			}

			return consts[row]
		},
		LvlCount: len(levels),
		LvlDamage: func(row int, _ bool, dd int) int {
			rec := levels[row]
			if rec == nil {
				return 0
			}

			return monlvlRow(rec, d.opt.Classic)[dd].dm
		},
		ChampionDamageBonus: d2difficulty.Default.Row(d2difficulty.Level(diff)).ChampionDamageBonus,
	}
}

// ApplyUModEffects runs the modifier init callbacks for a pack leader and the members linked to it: the four base
// ids, then the leader's own modifier list; a member gets the minion version of each. It does nothing twice.
func (d *Director) ApplyUModEffects(leader *d2mapentity.Monster, members []*d2mapentity.Monster) {
	if leader == nil || leader.UMod.Applied || leader.Stat == nil {
		return
	}

	leader.UMod.Applied = true

	lu := d.newFxUnit(leader)
	units := make([]d2monreg.FxUnit, 0, len(members))
	fx := []*fxUnit{lu}

	for _, m := range members {
		if m == nil || m.Stat == nil || m.UMod.Applied {
			continue
		}

		m.UMod.Applied = true

		u := d.newFxUnit(m)
		units = append(units, u)
		fx = append(fx, u)
	}

	d.fxCtx().ApplyInit(lu, leader.Modifiers, units)

	for _, u := range fx {
		u.finish()
	}
}

// fxUnit adapts a Monster to d2monreg.FxUnit.
type fxUnit struct {
	d     *Director
	m     *d2mapentity.Monster
	extra map[int]int
	base  [6]int
	steps int
}

func (d *Director) newFxUnit(m *d2mapentity.Monster) *fxUnit {
	return &fxUnit{d: d, m: m, extra: map[int]int{}, base: MonsterResists(m.Stat, m.Vitals.Difficulty)}
}

func (u *fxUnit) Stat(id int) int {
	v := &u.m.Vitals

	switch id {
	case 6:
		return v.HP << 8
	case 7:
		return v.MaxHP << 8
	case 12:
		return v.Level
	case 13:
		return v.Experience
	case 0x1f:
		return v.Defense
	}

	if i, ok := fxResistStats[id]; ok {
		return u.base[i] + u.extra[id]
	}

	return u.extra[id]
}

func (u *fxUnit) BaseStat(id int) int { return u.Stat(id) }

func (u *fxUnit) SetStat(id, val int) {
	v := &u.m.Vitals

	switch id {
	case 6:
		v.HP = maxInt(val>>8, 1)
	case 7:
		v.MaxHP = maxInt(val>>8, 1)
	case 12:
		v.Level = val
	case 13:
		v.Experience = maxInt(val, 0)
	case 0x1f:
		v.Defense = val
	default:
		if i, ok := fxResistStats[id]; ok {
			u.extra[id] = val - u.base[i]
		} else {
			u.extra[id] = val
		}
	}
}

func (u *fxUnit) AddStat(id, val int) {
	if val != 0 {
		u.SetStat(id, u.Stat(id)+val)
	}
}

func (u *fxUnit) IsChampion() bool { return u.m.TypeFlags&d2mapentity.MonTypeChampion != 0 }

func (u *fxUnit) OrTypeFlags(mask uint16) { u.m.TypeFlags |= mask }

// AdvanceNameSeed: the engine does not keep the exe's name seed (the unit seed is the Brain's); the step is counted.
func (u *fxUnit) AdvanceNameSeed() { u.steps++ }

func (u *fxUnit) NameSeed() uint16 {
	if u.m.Brain != nil {
		return uint16(u.m.Brain.Seed.Lo)
	}

	return 0
}

func (u *fxUnit) Velocity() int {
	if u.m.Stat == nil {
		return 0
	}

	return u.m.Stat.SpeedBase
}

func (u *fxUnit) HalfDamage() bool {
	return u.m.Stat != nil && strings.EqualFold(u.m.Stat.BaseKey, "willowisp1")
}

// GrantSkill is not wired to the monster's skill list yet: the grant is logged (Teleportation and the aura of an
// Aura Enchanted monster need the monster skill slots; UNVERIFIED how the AI picks them).
func (u *fxUnit) GrantSkill(skill, level int, active bool) {
	u.m.UMod.Skills = append(u.m.UMod.Skills, d2mapentity.UModSkill{Skill: skill, Level: level, Active: active})
}

func (u *fxUnit) SetAIFlag20()        { u.m.UMod.AIFlag20 = true }
func (u *fxUnit) ScheduleTimerEvent() { u.m.UMod.TimerEvent = true }

// finish turns the stats the callbacks left in extra into the engine's fields.
func (u *fxUnit) finish() {
	m, e := u.m, u.extra

	for id, i := range fxResistStats {
		m.UMod.Resist[i] = e[id]
	}

	v := &m.Vitals

	if p := e[0x10]; p != 0 {
		v.Defense = v.Defense * (100 + p) / 100
	}

	for _, a := range []*d2mapentity.MonsterAttack{&v.A1, &v.A2, &v.S1} {
		if p := e[0x19]; p != 0 {
			a.Min = a.Min * (100 + p) / 100
			a.Max = a.Max * (100 + p) / 100
		}

		if p := e[0x77]; p != 0 {
			a.ToHit = a.ToHit * (100 + p) / 100
		}
	}

	m.UMod.Elems = m.UMod.Elems[:0]

	add := func(t string, lo, hi int) {
		if hi > 0 {
			m.UMod.Elems = append(m.UMod.Elems, d2mapentity.UModElem{Type: t, Min: lo, Max: hi})
		}
	}

	add("fire", e[0x30], e[0x31])
	add("ltng", e[0x32], e[0x33])
	add("cold", e[0x36], e[0x37])
	add("pois", e[0x39]*e[0x3b]>>8, e[0x3a]*e[0x3b]>>8)

	m.UMod.DrainMin, m.UMod.DrainMax = e[0x3e]>>8, e[0x3f]>>8

	m.SetUModSpeed(e[0x43])
}

// uModHit adds what the monster's modifiers put on every hit (Fire / Lightning / Cold / Poison Enchanted) and takes
// the mana of Mana Burn. The elements are reduced by the hero's resistance like the attack's own element. The rolls
// are taken on the unit's seed only when a modifier carries the effect.
func (d *Director) uModHit(u *unit, p *d2mapentity.Player) int {
	b := &u.m.UMod
	total := 0

	for _, e := range b.Elems {
		r := e.Min + int(u.b.Seed.Roll(int32(e.Max-e.Min+1)))
		total += applyElemResist(r, e.Type, p.Stats.Totals)
	}

	if b.DrainMax > 0 {
		n := b.DrainMin + int(u.b.Seed.Roll(int32(b.DrainMax-b.DrainMin+1)))
		if n > p.Stats.Mana {
			n = p.Stats.Mana
		}

		p.Stats.Mana -= n
	}

	return total
}
