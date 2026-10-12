package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2state"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Engine side of the Amazon, Barbarian and Assassin effects added with
// docs/skills-coverage-abc.md: Find Potion / Find Item ("loot"), Grim Ward
// ("ward") and Whirlwind ("whirl"). The rules (rolls, tables, orders) are in
// d2common/d2skill/class_abc.go.

// abcState is the bookkeeping of those effects.
type abcState struct {
	// looted holds the corpses state 0x76 was set on (Find Potion / Item / Grim Ward).
	looted map[string]bool
	// whirling is the hero ids whose Whirlwind is running.
	whirling map[string]bool
	// shields counts the Blade Shield casts per hero (the newest one wins).
	shields map[string]int
}

// abcReset forgets the corpses and runs of the old area (ids are reused by the
// next one; the watches that drove the runs are dropped by AreaChanged) and
// takes the whirling state off the heroes.
func (e *Engine) abcReset() {
	for id := range e.abc.whirling {
		if s := e.sets[id]; s != nil {
			s.Remove("whirlwind")
		}
	}

	e.abc = abcState{}
}

func (e *Engine) isLooted(id string) bool { return e.abc.looted[id] }

// Looted reports whether a corpse was worked on by Find Potion, Find Item or
// Grim Ward (state 0x76 in the exe): those skills refuse it from then on.
func (e *Engine) Looted(corpseID string) bool { return e.isLooted(corpseID) }

func (e *Engine) markLooted(id string) {
	if e.abc.looted == nil {
		e.abc.looted = map[string]bool{}
	}

	e.abc.looted[id] = true
}

func (e *Engine) corpseByID(id string) *d2mapentity.Monster {
	for _, c := range e.monsters.Corpses() {
		if c.ID() == id {
			return c
		}
	}

	return nil
}

// actNow is the act of the hero's level (1..5), 1 when the host did not say.
func (e *Engine) actNow() int {
	if e.opt.Act != nil {
		if a := e.opt.Act(); a >= 1 {
			return a
		}
	}

	return 1
}

// loot applies Find Potion and Find Item: the corpse is marked looted and,
// when the chance roll won, the potion or the monster's treasure class
// drops on the ground at the corpse.
func (e *Engine) loot(p *d2mapentity.Player, sk *d2skill.Skill, ef *d2skill.Effect) {
	o := ef.Loot
	c := e.corpseByID(o.CorpseID)

	if c == nil || e.isLooted(o.CorpseID) {
		e.emit("loot", "LOOT skill=%q corpse=%s refused=gone_or_looted", sk.Name, o.CorpseID)

		return
	}

	e.markLooted(o.CorpseID)

	cx, cy := c.SubtilePos()

	if !o.Drop {
		e.emit("loot", "LOOT skill=%q kind=%s corpse=%s roll=%d chance=%d drop=false", sk.Name, o.Kind, c.Label(), o.Roll, o.Chance)

		return
	}

	switch o.Kind {
	case "potion":
		code := d2skill.FindPotionCode(e.actNow(), e.opt.Difficulty, o.Column)
		err := error(nil)

		if code != "" {
			err = e.monsters.DropItemCode(code, cx, cy)
		}

		e.emit("loot", "LOOT skill=%q kind=potion corpse=%s roll=%d chance=%d drop=true column=%d code=%s act=%d diff=%d err=%v",
			sk.Name, c.Label(), o.Roll, o.Chance, o.Column, code, e.actNow(), e.opt.Difficulty, err)
	default:
		tc := e.monsters.LootCorpse(c, o.TCType)
		e.emit("loot", "LOOT skill=%q kind=item corpse=%s roll=%d chance=%d drop=true tctype=%d tc=%q", sk.Name, c.Label(), o.Roll,
			o.Chance, o.TCType, tc)
	}
}

// ward starts a Grim Ward: the corpse is consumed and every WardOrder.Period
// frames the enemies within the radius get the skill's terror state, for the
// lifetime of the ward.
func (e *Engine) ward(p *d2mapentity.Player, sk *d2skill.Skill, ef *d2skill.Effect) {
	w := ef.Ward

	if c := e.corpseByID(w.CorpseID); c != nil {
		e.markLooted(w.CorpseID)
		e.monsters.RemoveCorpse(c)
	}

	end := e.frame + w.Life
	e.emit("state", "WARD start skill=%q at=(%d,%d) radius=%d period=%d life=%d state=%s fear=%d missile=%s", sk.Name, w.X, w.Y,
		w.Radius, w.Period, w.Life, w.State, w.Fear, w.Missile)

	e.watches = append(e.watches, &watch{every: w.Period, next: e.frame, fn: func() bool {
		if e.frame >= end {
			e.emit("state", "WARD end skill=%q", sk.Name)

			return false
		}

		n := 0

		for _, m := range e.monstersNear(w.X, w.Y, w.Radius) {
			n++

			e.applyMonsterState(m, d2state.Instance{Name: w.State, Until: e.frame + maxInt(w.Fear, 1), Source: p.ID(),
				SkillID: sk.ID, Level: ef.Level})
		}

		if n > 0 {
			e.emit("state", "WARD pulse skill=%q affected=%d state=%s", sk.Name, n, w.State)
		}

		return true
	}})
}

// whirl runs a Whirlwind: the hero walks the path to the destination at the
// base walk speed of its class (0.375 subtile per frame, d2skill.WhirlTickDistance,
// V wave 22) and the hit scheduler picks the victims within Radius in id order,
// until the destination, the longest run or a wall ends it. The skill does not
// start while the caster has the "inferno" state (state 12, SRVST_038).
func (e *Engine) whirl(p *d2mapentity.Player, u *heroUnit, sk *d2skill.Skill, ef *d2skill.Effect) {
	o := ef.Whirl
	if e.abc.whirling == nil {
		e.abc.whirling = map[string]bool{}
	}

	if e.abc.whirling[p.ID()] {
		return
	}

	if e.HasState(p.ID(), whirlRefusedState) {
		e.emit("state", "WHIRL refused skill=%q why=state_%d", sk.Name, d2skill.WhirlwindRefusedState)

		return
	}

	e.abc.whirling[p.ID()] = true

	hx, hy := u.Pos()
	e.emit("state", "WHIRL start skill=%q from=(%d,%d) to=(%d,%d) radius=%d delay=%d pct=%d", sk.Name, hx, hy, o.X, o.Y, o.Radius,
		o.Delay, o.Pct)

	if o.State != "" {
		e.setOf(p.ID()).Apply(e.frame, d2state.Instance{Name: o.State, Source: p.ID(), SkillID: sk.ID, Level: ef.Level, Count: 1})
	}

	steps, hits, last := 0, 0, ""
	budget, moved := 0.0, false
	sched := &d2skill.WhirlSched{}

	finish := func(why string) bool {
		delete(e.abc.whirling, p.ID())

		if o.State != "" {
			e.setOf(p.ID()).Remove(o.State)
		}

		x, y := u.Pos()
		e.emit("state", "WHIRL end skill=%q why=%s at=(%d,%d) steps=%d hits=%d", sk.Name, why, x, y, steps, hits)

		return false
	}

	e.watches = append(e.watches, &watch{every: 1, next: e.frame + 1, fn: func() bool {
		x, y := u.Pos()

		if x == o.X && y == o.Y {
			return finish("arrived")
		}

		if steps >= o.MaxSteps {
			return finish("max_run")
		}

		// the path velocity is the base walk speed (d2skill.WhirlTickDistance, V wave 22): cells are entered as the
		// travelled distance allows
		budget += d2skill.WhirlTickDistance

		for {
			x, y = u.Pos()
			sx, sy := sign(o.X-x), sign(o.Y-y)

			if (sx == 0 && sy == 0) || steps >= o.MaxSteps || budget < d2skill.WhirlStepCost(sx, sy) {
				break
			}

			if e.pipe.Walkable != nil && !e.pipe.Walkable(x+sx, y+sy) {
				if moved {
					break
				}

				return finish("wall")
			}

			e.setHeroPos(p, x+sx, y+sy)

			budget -= d2skill.WhirlStepCost(sx, sy)
			steps++
			moved = true
		}

		nx, ny := u.Pos()
		moved = false

		// SKILL_WhirlwindComputeNextStepDelay: the scheduler says how many hits this frame
		for n := sched.Step(e.frame, o.Delay, o.Dual); n > 0; n-- {
			// SKILL_ScanRadiusForTargets: euclidean radius, victims rotate in id order
			var in []d2skill.Foe

			for _, f := range e.pipe.Foes(nx, ny, o.Radius) {
				dx, dy := f.X-nx, f.Y-ny
				if dx*dx+dy*dy <= d2skill.WhirlRadius2 {
					in = append(in, f)
				}
			}

			ids := make([]string, len(in))
			for i, f := range in {
				ids[i] = f.Target.ID()
			}

			i := d2skill.WhirlNext(ids, last)
			if i < 0 {
				last = ""

				break
			}

			last = ids[i]

			if r := e.pipe.WhirlStrike(u, sk.ID, in[i].Target); r != nil {
				hits++
				e.meleeResult(p, sk, r)
			}
		}

		return true
	}})
}

// shield runs a Blade Shield: for ef.Frames frames, every ef.Interval frames
// the enemies within ef.Radius of the hero take the skill's damage.
func (e *Engine) shield(p *d2mapentity.Player, u *heroUnit, sk *d2skill.Skill, ef *d2skill.Effect) {
	end := e.frame + ef.Frames

	if e.abc.shields == nil {
		e.abc.shields = map[string]int{}
	}

	e.abc.shields[p.ID()]++ // a new cast replaces the shield that is running
	gen := e.abc.shields[p.ID()]

	e.emit("state", "SHIELD start skill=%q radius=%d interval=%d frames=%d", sk.Name, ef.Radius, ef.Interval, ef.Frames)

	e.watches = append(e.watches, &watch{every: ef.Interval, next: e.frame + ef.Interval, fn: func() bool {
		if e.frame >= end || e.abc.shields[p.ID()] != gen || !e.HasState(p.ID(), ef.State) {
			e.emit("state", "SHIELD end skill=%q", sk.Name)

			return false
		}

		// VERIFIED SRVDO_054 0x5d6880: the periodic damage call (SRVDO_142 0x5d6750) is skipped in a town room
		// (the state itself stays on).
		if u.InTown() {
			return true
		}

		hx, hy := u.Pos()
		n := 0

		for _, m := range e.monstersNear(hx, hy, ef.Radius) {
			mt := e.target(m)

			// VERIFIED (0x5d6750): with result flags bit 0 clear each unit is rolled against first
			if ef.RollToHit {
				if hit, _, _ := d2combat.RollToHit(u.Roller(), d2combat.ToHitInput{AttackRating: u.AttackRating(), Defense: mt.Defense(false),
					AttackerLevel: u.Level(), DefenderLevel: mt.Level(), AttackRatingPct: ef.Pct + u.Stat("item_tohit_percent")}); !hit {
					continue
				}
			}

			dmg := e.rollDesc(u, ef.Desc)
			n++

			e.hurt(m, p, &dmg, sk.Name)
		}

		if n > 0 {
			e.Counters.AreaHits += n
			e.noteLanded(sk.Name, n)
			e.emit("hit", "SKILL area skill=%q at=(%d,%d) radius=%d targets=%d", sk.Name, hx, hy, ef.Radius, n)
		}

		return true
	}})
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}

	return 0
}
