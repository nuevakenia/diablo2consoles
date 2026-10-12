package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monreg"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The run time hooks of the monster modifiers (d2monreg/umodhook.go has the table of the exe): Cursed (7, raised when
// the monster's damage lands), Multishot (29, raised when the monster creates a missile) and the timer of modifier 41.

// hasMod says whether the monster's modifier list holds the id.
func hasMod(m *d2mapentity.Monster, id int) bool {
	for _, v := range m.Modifiers {
		if v == id {
			return true
		}
	}

	return false
}

// isUnique is the "unique" argument the exe's hooks get (type word mask 0x8).
func isUnique(m *d2mapentity.Monster) bool { return m.TypeFlags&d2mapentity.MonTypeUnique != 0 }

// multishot is the Multishot hook (0x5a1120): after a unique that carries modifier 29 created a missile, the exe
// casts the same skill twice more from the monster at the two points beside the aim point. The extra missiles use the
// same attack; they cannot multiply again (the exe sets the 0x80 guard while it fires them, launchShot bypasses this
// hook). Missiles whose missiles.txt row has NoUniqueMod do not take part.
func (d *Director) multishot(u *unit, mode d2monster.Mode, atk d2mapentity.MonsterAttack, ax, ay int) {
	if !hasMod(u.m, d2monreg.UModMultishot) || !isUnique(u.m) {
		return
	}

	name := shotMissile(u, mode)
	row, noUniqueMod := 0, false

	if d.asset != nil && d.asset.Records != nil && name != "" {
		if rec := d.asset.Records.GetMissileByName(name); rec != nil {
			row, noUniqueMod = rec.Id, rec.IgnoreBossModifiers
		}
	}

	if !d2monreg.MultishotEligible(true, noUniqueMod, false) {
		return
	}

	sx, sy := u.m.SubtilePos()

	for _, to := range d2monreg.MultishotTargets(sx, sy, ax, ay, row) {
		if d.launchShot(u, mode, atk, to[0], to[1]) {
			d.Counters.Multishots++
		}
	}

	d.emit("umod", "MONSTER multishot name=%s id=%d missile=%q aim=(%d,%d)", u.m.Label(), u.b.ID, name, ax, ay)
}

// cursedHit is the Cursed hook (0x5a0060, raised from COMBAT_ApplyDamageToUnit when a monster's damage landed): a
// unique with modifier 7 casts Amplify Damage three times in four, at skill level monster level / 5 + 1, on every
// hostile unit within the (clamped) radius of the monster's target. The stats go on the hero as timed stats
// (TimedHeroStats). Needs the skill table; without one nothing happens. UNVERIFIED: the radius is read in subtiles
// with a round area, and a second curse replaces the first (the exe's stacking of the same state is not recorded).
func (d *Director) cursedHit(u *unit, hit *d2mapentity.Player) {
	if !hasMod(u.m, d2monreg.UModCursed) || !isUnique(u.m) || d.asset == nil || d.asset.Records == nil {
		return
	}

	cast, lvl := d2monreg.CursedRoll(u.b.Seed, true, u.m.Vitals.Level)
	if !cast {
		return
	}

	reg := d.asset.Records.SkillTable()
	if reg == nil {
		return
	}

	c := d2skill.MonsterCurseOf(reg, reg.ByID(d2skill.MonsterCurseSkill), lvl, u.m.Vitals.Level)
	if c == nil {
		return
	}

	radius := d2monreg.CurseRadius(c.Radius)
	cx, cy := playerSubtile(hit)
	n := 0

	for _, tid := range d.targetIDs() {
		p := d.targets[tid]
		if !d.targetable(p) {
			continue
		}

		px, py := playerSubtile(p)
		if (px-cx)*(px-cx)+(py-cy)*(py-cy) > radius*radius {
			continue
		}

		d.curseHero(p, c.Stats, c.Frames)

		n++
	}

	d.Counters.Curses += n
	d.emit("umod", "MONSTER cursed name=%s id=%d level=%d radius=%d frames=%d heroes=%d", u.m.Label(), u.b.ID, lvl, radius,
		c.Frames, n)
}

// curseHero puts the curse stats on a hero as timed stats until frames pass.
func (d *Director) curseHero(p *d2mapentity.Player, stats []d2skill.StatMod, frames int) {
	if p == nil || p.Stats == nil {
		return
	}

	if d.curses == nil {
		d.curses = map[*d2mapentity.Player]map[string]timedStat{}
	}

	if d.curses[p] == nil {
		d.curses[p] = map[string]timedStat{}
	}

	for _, s := range stats {
		d.curses[p][s.Stat] = timedStat{value: s.Value, until: d.frame + frames}
	}

	if p.Stats.Recalc != nil {
		p.Stats.Recalc()
	}
}

// cursedStats returns the unexpired curse stats of a hero.
func (d *Director) cursedStats(p *d2mapentity.Player) map[string]int {
	var out map[string]int

	for stat, ts := range d.curses[p] {
		if ts.until <= d.frame {
			continue
		}

		if out == nil {
			out = map[string]int{}
		}

		out[stat] += ts.value
	}

	return out
}

// umodTimer is the timer event of modifier 41 (init 0x59eea0 schedules event 7 at frame + 75, the hook 0x5a1d10 runs
// at the event): while the monster is alive it re-arms the AI think event two frames ahead when the exe's rule says so
// (d2monreg.TimerWakes) and schedules itself again 75 frames later. The Go AI is not event driven, so the think event
// is the brain's wake frame.
func (d *Director) umodTimer(u *unit) {
	b := &u.m.UMod
	if !b.TimerEvent || d.frame < b.TimerAt {
		return
	}

	alive, wake := d2monreg.TimerWakes(int(u.b.Mode), d.baseClass(u))
	if !alive {
		return
	}

	b.TimerAt = d.frame + d2monreg.TimerPeriod

	if wake && u.b.Wake > d.frame+d2monreg.TimerWakeDelay {
		u.b.WakeNow(d.frame + d2monreg.TimerWakeDelay)
	}

	d.Counters.UModTimers++
}

// baseClass is the monstats row of the unit's base class (MONSTER_GetBaseClassId: the BaseId column).
func (d *Director) baseClass(u *unit) int {
	st := u.m.Stat
	if st == nil {
		return -1
	}

	if st.BaseKey == "" || strings.EqualFold(st.BaseKey, st.Key) {
		return st.ID
	}

	for id, r := range d.statByID {
		if strings.EqualFold(r.Key, st.BaseKey) {
			return id
		}
	}

	return st.ID
}
