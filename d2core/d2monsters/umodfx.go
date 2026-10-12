package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2difficulty"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monreg"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
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

// AdvanceNameSeed is MONSTER_AdvanceSeedAndNameSeed 0x59e880 (VERIFIED): the unit's seed takes one step and the low
// 16 bits of the new low word become the name seed (monster data +0x14). The Go unit seed is the Brain's.
func (u *fxUnit) AdvanceNameSeed() {
	u.steps++

	if u.m.Brain != nil && u.m.Brain.Seed != nil {
		u.m.UMod.NameSeed = uint16(u.m.Brain.Seed.Step())
	}
}

func (u *fxUnit) NameSeed() uint16 { return u.m.UMod.NameSeed }

func (u *fxUnit) Velocity() int {
	if u.m.Stat == nil {
		return 0
	}

	return u.m.Stat.SpeedBase
}

func (u *fxUnit) HalfDamage() bool {
	return u.m.Stat != nil && strings.EqualFold(u.m.Stat.BaseKey, "willowisp1")
}

// auraStatIDs maps the stat names the aura skills put on their own state (SRVDO_065 / 066 / 081 passive stats) to the
// stat ids finish() reads.
var auraStatIDs = map[string]int{
	"firemindam": 0x30, "firemaxdam": 0x31, "lightmindam": 0x32, "lightmaxdam": 0x33,
	"coldmindam": 0x36, "coldmaxdam": 0x37, "damagepercent": 0x19, "item_tohit_percent": 0x77,
}

// GrantSkill records the grant (UMod.Skills) and puts the passive stats of an aura skill on the monster, the way the
// exe's aura state does for its caster (d2skill.MonsterAuraStats: Might and Fanaticism damage percent, the elemental
// damage of Holy Fire, Holy Freeze and Holy Shock, Blessed Aim to-hit). The effect on allies in the aura range, the
// Thorns and Conviction effects and the upkeep are not modelled (UNVERIFIED for monsters). Teleport (0xb8) is no
// aura: SetAIFlag20 opens the wounded teleport (0x5aedc0) for the unit.
func (u *fxUnit) GrantSkill(skill, level int, active bool) {
	u.m.UMod.Skills = append(u.m.UMod.Skills, d2mapentity.UModSkill{Skill: skill, Level: level, Active: active})

	if !active || u.d == nil || u.d.asset == nil || u.d.asset.Records == nil {
		return
	}

	reg := u.d.asset.Records.SkillTable()

	for _, sm := range d2skill.MonsterAuraStats(reg, reg.ByID(skill), level, u.m.Vitals.Level) {
		if id, ok := auraStatIDs[sm.Stat]; ok {
			u.AddStat(id, sm.Value)
		}
	}
}

func (u *fxUnit) SetAIFlag20() {
	u.m.UMod.AIFlag20 = true

	if u.m.Brain != nil { // 0x5aedc0 tests this flag on the unit's brain
		u.m.Brain.CanTeleport = true
	}
}
func (u *fxUnit) ScheduleTimerEvent() {
	u.m.UMod.TimerEvent = true
	u.m.UMod.TimerAt = u.d.frame + d2monreg.TimerPeriod // 0x59eea0: event 7 at frame + 0x4b
}

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
func (d *Director) uModHit(u *unit, p *d2mapentity.Player) int { return d.uModHitInto(u, p, nil) }

// uModHitInto is uModHit that also books each element roll in parts (when not nil).
func (d *Director) uModHitInto(u *unit, p *d2mapentity.Player, parts *HitParts) int {
	b := &u.m.UMod
	total := 0

	for _, e := range b.Elems {
		r := e.Min + int(u.b.Seed.Roll(int32(e.Max-e.Min+1)))
		r, heal := applyElemReduce(r, e.Type, p.Stats.Totals)
		if parts != nil {
			parts.Heal += heal
		}

		parts.add(e.Type, r)
		total += r
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
