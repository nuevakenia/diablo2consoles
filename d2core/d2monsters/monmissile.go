package d2monsters

import (
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 36 boss fidelity (notes: w36-bosses.md). Monster skills used to be a straight bolt or nothing: a skill whose
// srvdofunc makes missiles (Diablo's lightning, Andariel's spray, Mephisto's poison nova and charged bolts, the
// Diablo wall) dealt no damage at all. This file hosts the missile simulator (d2common/d2missile) for monster
// owners: the do function creates missiles with the numbers of the exe (what was read, see each function), the sim
// flies them and applies the per-missile rules, and a hit on a hero is resolved by resolveAttack with the to-hit roll
// already done by the sim (fx.forceHit), so block, resists, absorbs, thorns and the like stay the ones of every
// monster blow.
//
// Ported from the exe (VERIFIED, read in wave 36):
//   - SRVDO_152_DiabLight 0x5ca970: one missile (srvmissilea) at the caster toward the skill target, speed from
//     the level velocity, lifetime from SRVDO_SetMissileLifeFromCalc 0x5ca5b0 (calc1 clamped 1..255; below 1 the
//     missile's Param2 - 1 + the skill level).
//   - SRVDO_099_PrimePoisonNova 0x5cafd0: 8 clouds at the 8 ring points of radius 2 (speed Param1 << 6) and 8 more
//     at every calc2-th of the 16 ring points starting with the second (speed Param2 << 6), missile level calc1.
//   - SRVDO_ChargedBolt 0x5c73a0 (do 17): calc1 bolts of the progressive missile, each aimed at the target through
//     path type 10 (the hero skill's CastTrapBolts uses the same rule).
//   - SRVDO_102_DiabWall 0x5cb4b0: calc1 wall makers from the caster toward the target, each turned by its index in
//     the per-missile callback 0x5cb410 (the turn and the random speed jitter of that callback are UNVERIFIED here:
//     the makers fan out by 0.4 rad steps).
//   - SRVDO_088_AndrialSpray 0x5c97d0: one missile (srvmissilea) per action frame of the cast, starting a short way
//     in front of Andariel; the table of 9 sweep directions per facing is not ported: the nine missiles fan over
//     +-0.35 rad at once (UNVERIFIED shape and timing).

// bossFX is the state of the wave 36 effects of a Director.
type bossFX struct {
	sim      *d2missile.Sim
	forceHit bool                // resolveAttack skips the to-hit roll (the sim rolled it)
	casts    map[uint32]monCast  // brain id -> the last missile cast (mode, damage column)
	auras    map[uint32]*monAura // brain id -> a running aura (Duriel's Holy Freeze)
	events   int                 // missile hit events applied (tests)
	carry    map[carryKey]int    // the 8.8 fraction of a hit below one life point, per hero, monster and part
	_        struct{}
}

type monCast struct {
	mode  d2monster.Mode
	level int
}

// monTarget adapts a hero to d2missile.Target.
type monTarget struct {
	d *Director
	p *d2mapentity.Player
}

func (t monTarget) ID() string     { return t.p.ID() }
func (t monTarget) IsPlayer() bool { return true }
func (t monTarget) Alive() bool    { return t.d.targetable(t.p) }

func (t monTarget) Level() int {
	if t.p.Stats == nil {
		return 1
	}

	return t.p.Stats.Level
}

func (t monTarget) Defense(missile bool) int {
	tot := t.p.Stats.Totals
	if tot == nil {
		return 0
	}

	def := tot.Defense
	if missile && tot.Stats != nil {
		def += int(tot.Stats.Get(statArmorMissile))
	}

	return def
}

const statArmorMissile = 0x20 // armor vs missile, the exe's stat 0x20 (resolveAttack reads it the same way)

type monWorld struct {
	d2path.Grid
	d *Director
}

func (w monWorld) Targets(x, y int) []d2missile.Target {
	var out []d2missile.Target

	for _, tid := range w.d.targetIDs() {
		p := w.d.targets[tid]
		if p == nil || !w.d.targetable(p) {
			continue
		}

		px, py := playerSubtile(p)
		if abs(px-x) <= shotCollideRadius && abs(py-y) <= shotCollideRadius {
			out = append(out, monTarget{d: w.d, p: p})
		}
	}

	return out
}

func (w monWorld) IsEnemy(o d2missile.Owner, t d2missile.Target) bool {
	return !o.IsPlayer && t.IsPlayer()
}
func (w monWorld) Frame() int { return w.d.frame }

// monSim is the missile simulator of the monsters, created on first use; nil without the missile table.
func (d *Director) monSim() *d2missile.Sim {
	if d.fx.sim != nil {
		return d.fx.sim
	}

	if d.asset == nil || d.asset.Records == nil {
		return nil
	}

	tbl := d.asset.Records.MissileTable()
	if tbl == nil {
		return nil
	}

	s := d2missile.NewSim(monWorld{Grid: d.grid, d: d}, tbl)
	s.OnEvent = d.monMissileEvent
	d.fx.sim = s

	return s
}

// stepBossFX is one frame of the wave 36 effects.
func (d *Director) stepBossFX() {
	if d.fx.sim != nil {
		d.fx.sim.Step()
	}

	d.stepAuras()
}

func (d *Director) skillRegistry() *d2skill.Registry {
	if d.asset == nil || d.asset.Records == nil {
		return nil
	}

	return d.asset.Records.SkillTable()
}

// skillAim is the point a cast aims at: the target hero's cell, else the ground point of the request.
func (d *Director) skillAim(u *unit) (int, int) {
	if p := d.playerFor(u.attackTarget); p != nil && d.targetable(p) {
		return playerSubtile(p)
	}

	return u.aimX, u.aimY
}

func ownerID(u *unit) string { return strconv.FormatUint(uint64(u.b.ID), 10) }

func (d *Director) monOwner(u *unit, mode d2monster.Mode) d2missile.Owner {
	atk := columnAttack(&u.m.Vitals, mode)

	return d2missile.Owner{
		ID: ownerID(u), IsPlayer: false, Level: u.m.Vitals.Level, AttackRating: d2combat.MonsterAttackRating(atk.ToHit, 0, 0),
		Roller: u.b.Seed, Gone: func() bool { return !u.m.Alive() },
		Pos: func() (float64, float64) {
			x, y := u.m.SubtilePos()

			return float64(x) + 0.5, float64(y) + 0.5
		},
	}
}

// monMissile creates one missile of a monster skill. A nil result means the sim refused it (too far, no table).
func (d *Director) monMissile(u *unit, mode d2monster.Mode, name string, lvl int, dx, dy float64, tune func(*d2missile.CreateParams)) *d2missile.Missile {
	s := d.monSim()
	if s == nil || name == "" {
		return nil
	}

	spec := s.Table.ByName(name)
	if spec == nil {
		return nil
	}

	sx, sy := u.m.SubtilePos()
	p := d2missile.CreateParams{
		Spec: spec, Owner: d.monOwner(u, mode), Level: lvl,
		X: float64(sx) + 0.5, Y: float64(sy) + 0.5, DestX: dx, DestY: dy,
		Damage: d.missileDesc(u, name, lvl, mode), ChildDamage: d.childDescs(u, spec, lvl, mode, 0),
	}

	if tune != nil {
		tune(&p)
	}

	m, err := s.Create(p)
	if err != nil {
		return nil
	}

	return m
}

// landBossSkill runs the exe's do function of a boss skill at the hit frame of its cast. It returns true when the
// skill was handled here (no plain bolt or melee blow follows).
func (d *Director) landBossSkill(u *unit, mode d2monster.Mode) bool {
	if u.skill == nil || u.skill.rec == nil {
		return false
	}

	rec, lvl := u.skill.rec, u.skill.level

	if d.fx.casts == nil {
		d.fx.casts = map[uint32]monCast{}
	}

	d.fx.casts[u.b.ID] = monCast{mode: mode, level: lvl}

	switch rec.Srvdofunc {
	case 30, 112:
		return d.landCurse(u, rec, lvl)
	case 17, 88, 99, 102, 152:
	default:
		return false
	}

	ax, ay := d.skillAim(u)
	tx, ty := float64(ax)+0.5, float64(ay)+0.5
	name := rec.Srvmissilea

	if name == "" {
		name = rec.Srvmissile
	}

	reg := d.skillRegistry()
	sk := (*d2skill.Skill)(nil)

	if reg != nil {
		sk = reg.ByID(rec.ID)
	}

	calc := func(n int) int { return d2skill.MonsterCalc(reg, sk, lvl, u.m.Vitals.Level, n) }
	n := 0

	switch rec.Srvdofunc {
	case 152: // DiabLight
		life := calc(1)
		if life < 1 {
			life = 0
			if s := d.monSim(); s != nil {
				if sp := s.Table.ByName(name); sp != nil {
					life = sp.Param[1] - 1 + lvl
				}
			}
		}

		if life < 2 {
			life = 1
		} else if life > 0xfe {
			life = 0xff
		}

		if d.monMissile(u, mode, name, lvl, tx, ty, func(p *d2missile.CreateParams) { p.Range = life }) != nil {
			n++
		}
	case 99: // PrimePoisonNova
		n = d.poisonNova(u, mode, name, lvl, calc(1), calc(2))
	case 17: // Charged Bolt
		for i, bolts := 0, calc(1); i < bolts && ax != 0 && ay != 0; i++ {
			i := i

			if d.monMissile(u, mode, name, lvl, tx, ty, func(p *d2missile.CreateParams) {
				p.PathType10, p.PathSeed = true, d2skill.ChargedBoltSeed(ax, i)
			}) != nil {
				n++
			}
		}
	case 102: // DiabWall
		for i, makers := 0, calc(1); i < makers; i++ {
			i := i

			if d.monMissile(u, mode, name, lvl, tx, ty, func(p *d2missile.CreateParams) {
				p.Angle = fanAngle(i, 0.4)
			}) != nil {
				n++
			}
		}
	case 88: // AndrialSpray
		const sweep = 9

		for i := 0; i < sweep; i++ {
			i := i

			if d.monMissile(u, mode, name, lvl, tx, ty, func(p *d2missile.CreateParams) {
				p.Angle = (float64(i)/float64(sweep-1) - 0.5) * 0.7
			}) != nil {
				n++
			}
		}
	}

	d.emit("bossfx", "MONSTER missiles name=%s id=%d skill=%q do=%d count=%d aim=(%d,%d)", u.m.Label(), u.b.ID, rec.Skill, rec.Srvdofunc,
		n, ax, ay)

	return true
}

// fanAngle turns index i into 0, +step, -step, +2 step...: the makers of a wall spread both ways.
func fanAngle(i int, step float64) float64 {
	k := float64((i + 1) / 2)
	if i%2 == 1 {
		return k * step
	}

	return -k * step
}

// The ring of SRVDO_099 (the exe's two 16 entry tables at 0x6e4738 and 0x6e4778, VERIFIED values).
var (
	novaRingY = [16]int{2, 2, 2, 1, 0, -1, -2, -2, -2, -2, -2, -1, 0, 1, 2, 2}
	novaRingX = [16]int{0, 1, 2, 2, 2, 2, 2, 1, 0, -1, -2, -2, -2, -2, -2, -1}
)

// novaPoints lists the destination offsets of the ring clouds: the 8 even points (speed Param1) and, when calc2 is
// above 1, the points 1, 1+calc2, ... below byte offset 0x3c (speed Param2).
func novaPoints(calc2 int) (inner, outer [][2]int) {
	for i := 0; i < 16; i += 2 {
		inner = append(inner, [2]int{novaRingX[i], novaRingY[i]})
	}

	if calc2 > 1 {
		for off := 0; off < 0x3c; off += calc2 * 4 {
			i := 1 + off/4
			if i < 16 {
				outer = append(outer, [2]int{novaRingX[i], novaRingY[i]})
			}
		}
	}

	return inner, outer
}

func (d *Director) poisonNova(u *unit, mode d2monster.Mode, name string, lvl, calc1, calc2 int) int {
	s := d.monSim()
	if s == nil {
		return 0
	}

	sp := s.Table.ByName(name)
	if sp == nil {
		return 0
	}

	if calc1 < 1 {
		calc1 = lvl
	}

	if calc2 < 1 {
		calc2 = 1
	}

	inner, outer := novaPoints(calc2)
	sx, sy := u.m.SubtilePos()
	n := 0

	fire := func(pts [][2]int, param int) {
		for _, o := range pts {
			if d.monMissile(u, mode, name, calc1, float64(sx)+0.5+float64(o[0]), float64(sy)+0.5+float64(o[1]),
				func(p *d2missile.CreateParams) { p.Velocity = param << 6 }) != nil {
				n++
			}
		}
	}

	fire(inner, sp.Param[0])
	fire(outer, sp.Param[1])

	return n
}

// carryKey names one 8.8 remainder: a hero, the monster whose missile hit and the part (0 physical, 1 element).
type carryKey struct {
	hero string
	unit uint32
	part int
}

// monMissileEvent applies a missile event of the monsters' sim: a hit on a hero (the sim rolled the damage in 8.8 and
// the to-hit) goes through resolveAttack like every monster blow. A hit below one life point (the standing fires hit
// every frame for a few 1/256) keeps its fraction until it adds up.
func (d *Director) monMissileEvent(e d2missile.Event) {
	if e.Kind != d2missile.EventHit {
		return
	}

	t, ok := e.Target.(monTarget)
	if !ok || e.Missile == nil {
		return
	}

	id, err := strconv.ParseUint(e.Missile.Owner.ID, 10, 32)
	if err != nil {
		return
	}

	u := d.units[uint32(id)]
	if u == nil {
		return
	}

	cast := d.fx.casts[u.b.ID]
	atk := d.damageToAttack(u, t.p, e.Damage)

	if atk.Max <= 0 && atk.ElemMax <= 0 {
		return
	}

	d.fx.events++
	d.fx.forceHit = true
	d.Counters.Attacks++
	d.Counters.ShotHits++
	d.resolveAttack(u, t.p, cast.mode, atk, " missile")
	d.fx.forceHit = false
}

// damageToAttack turns the rolled 8.8 damage of a missile into whole points (keeping the fractions per hero) as a
// monster attack: physical plus the one element the missile carries (fire, lightning, magic, cold, or the poison
// total rate * length, dealt at once like every monster poison here, UNVERIFIED).
func (d *Director) damageToAttack(u *unit, p *d2mapentity.Player, dmg d2combat.Damage) d2mapentity.MonsterAttack {
	if d.fx.carry == nil {
		d.fx.carry = map[carryKey]int{}
	}

	whole := func(part int, v int32) int {
		k := carryKey{hero: p.ID(), unit: u.b.ID, part: part}
		tot := int(v) + d.fx.carry[k]
		d.fx.carry[k] = tot & 0xff

		return tot >> 8
	}

	atk := d2mapentity.MonsterAttack{ToHit: u.m.Vitals.A1.ToHit}
	atk.Min = whole(0, dmg.Physical)
	atk.Max = atk.Min

	type el struct {
		name string
		v    int32
	}

	for _, e := range []el{{"fire", dmg.Fire}, {"ltng", dmg.Lightning}, {"mag", dmg.Magic}, {"cold", dmg.Cold},
		{"pois", int32(int64(dmg.Poison) * int64(dmg.PoisonLen))}} {
		if e.v <= 0 {
			continue
		}

		atk.ElemType = e.name
		atk.ElemMin = whole(1, e.v)
		atk.ElemMax = atk.ElemMin

		break
	}

	return atk
}

// missileDesc is the damage descriptor of a missile of a monster in 8.8 fixed point: the missile row's own columns,
// or, when its Skill column names a skills.txt row, that row's (the rule skillDamage applies to a srvmissile), plus
// the monster's unit damage of the mode scaled by SrcDam/128.
func (d *Director) missileDesc(u *unit, name string, lvl int, mode d2monster.Mode) d2missile.DamageDesc {
	if d.asset == nil || d.asset.Records == nil {
		return d2missile.DamageDesc{}
	}

	mr := d.asset.Records.GetMissileByName(name)
	if mr == nil {
		return d2missile.DamageDesc{}
	}

	if lvl < 1 {
		lvl = 1
	}

	spec := specFromMissile(mr)

	if mr.SkillName != "" {
		if lr := d.asset.Records.GetSkillByName(mr.SkillName); lr != nil {
			spec = specFromSkill(lr)
		}
	}

	base := columnAttack(&u.m.Vitals, mode)

	var desc d2missile.DamageDesc

	if spec.SrcDam > 0 {
		desc.PhysMin = int32(base.Min*spec.SrcDam/128) << 8
		desc.PhysMax = int32(base.Max*spec.SrcDam/128) << 8
	}

	desc.PhysMin += spec.PhysMin(nil, lvl, 0, false)
	desc.PhysMax += spec.PhysMax(nil, lvl, 0, false)

	if desc.PhysMax < desc.PhysMin {
		desc.PhysMax = desc.PhysMin
	}

	el := d2missile.Elem{Min: spec.ElemMin(nil, lvl), Max: spec.ElemMax(nil, lvl), Len: int32(spec.ElemLen(nil, lvl))}

	switch normElem(spec.EType) {
	case "fire":
		desc.Fire = el
	case "ltng":
		desc.Lightning = el
	case "mag":
		desc.Magic = el
	case "cold":
		desc.Cold = el
	case "pois":
		desc.Poison = el
	}

	return desc
}

// childDescs gives the sub missiles of a missile their own damage (the sim hands the parent's table to its children).
func (d *Director) childDescs(u *unit, spec *d2missile.Spec, lvl int, mode d2monster.Mode, depth int) map[string]d2missile.DamageDesc {
	if depth > 2 || spec == nil || d.fx.sim == nil {
		return nil
	}

	var names []string

	names = append(names, spec.SubMissile[:]...)
	names = append(names, spec.HitSubMissile[:]...)
	names = append(names, spec.ExplosionMissile)

	out := map[string]d2missile.DamageDesc{}

	for _, n := range names {
		if n == "" {
			continue
		}

		child := d.fx.sim.Table.ByName(n)
		if child == nil {
			continue
		}

		out[child.Name] = d.missileDesc(u, n, lvl, mode)

		for k, v := range d.childDescs(u, child, lvl, mode, depth+1) {
			out[k] = v
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}
