package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// The hero's plain melee swing and the item thorns go through the same hit
// resolution as a skill strike (hurt: resists per type, then the item events),
// instead of subtracting a bare number from the monster's life.

// heroStatGetter reads a stat id of the hero the way BuildAttackerDamage
// (0x5794e0) reads its attacker: the equipment's list plus what her states and
// passives add under the same stat (names of itemStatIDs).
func (e *Engine) heroStatGetter(p *d2mapentity.Player) func(int) int32 {
	h := e.hero(p)
	names := map[int]string{}

	for n, id := range itemStatIDs {
		names[id] = n
	}

	return func(id int) int32 {
		var v int64

		if t := p.Stats.Totals; t != nil && t.Stats != nil {
			v = t.Stats.Get(id)
		}

		if n, ok := names[id]; ok {
			v += int64(e.setOf(p.ID()).Stat(e.frame, n) + h.passive(n))
		}

		return int32(v)
	}
}

// heroPlainHit finishes a plain melee hit of a hero (to-hit already rolled by
// the director): the weapon roll phys (whole points) gets the strike chances,
// elemental, poison, leech and stun parts of BuildAttackerDamage, then the hit
// is resolved against the monster like a skill melee strike. It reports true
// when it handled the hit.
func (e *Engine) heroPlainHit(p *d2mapentity.Player, m *d2mapentity.Monster, phys int) bool {
	if p == nil || p.Stats == nil || m == nil || !m.Alive() {
		return false
	}

	h := e.hero(p)
	get := e.heroStatGetter(p)

	in := d2combat.MeleeIn{
		Get: get, AttackerKind: 0, DefenderKind: 1,
		DefenderDemon: m.Stat != nil && m.Stat.IsDemon, DefenderUndead: e.isUndead(m),
		ActiveWeapon: p.Equipment != nil && p.Equipment.RightHand != nil,
		Phys:         int32(phys) << d2combat.FixedShift,
		Damage:       d2combat.Damage{Result: d2combat.ResultHit},
	}
	out := d2combat.BuildAttackerDamage(h.seed, in)
	d := out.Damage

	e.Counters.Melee++
	e.Counters.Hits++
	e.emit("hit", "SKILL melee skill=%q target=%s hit=true %s", "attack", m.Label(), describe(&d))

	e.hurt(m, p, &d, "attack")
	e.leech(p, m, &d)
	e.itemEvents(m, p, true)

	return true
}

// heroThorns is the item thorns of a hero (stats 78 and 128, event funcs 6 and
// 10, 0x5bd250 / 0x5bd310, VERIFIED): a flat physical and a flat lightning
// damage, both 8.8, dealt to the monster that hit her in melee, with the
// monster's resists applied; the hero is the attacker of record.
func (e *Engine) heroThorns(p *d2mapentity.Player, m *d2mapentity.Monster, phys, light int32) {
	if m == nil || !m.Alive() {
		return
	}

	d := d2combat.Damage{Result: d2combat.ResultHit, Physical: phys, Lightning: light}

	e.hurt(m, p, &d, "thorns")
}
