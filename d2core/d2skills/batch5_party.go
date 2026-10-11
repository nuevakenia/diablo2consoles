package d2skills

import (
	"sort"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Holy Bolt on party members (batch 5). MISSILE hit function 7 (0x5a7a40, VERIFIED) heals any unit that is not an
// enemy of the bolt's owner by calc1..calc2 of the casting skill and ends the bolt without damage. A hero in the
// caster's party is no enemy (the exe's ally tests, 0x552320 / 0x552d80), so the bolt heals it exactly like a
// minion or the mercenary. The engine lists the party through Engine.Party (heroes of the caster's party other
// than the caster, local or remote) and hands each heal to Engine.OnPartyHeal so the game can tell the defender's
// client; the local copy of the hero is healed at once.

// partyMembers lists the living party heroes sorted by id.
func (e *Engine) partyMembers() []*d2mapentity.Player {
	if e.Party == nil {
		return nil
	}

	var out []*d2mapentity.Player

	for _, p := range e.Party() {
		if p != nil && !p.IsDead() && p.Stats != nil && p.Stats.Health > 0 {
			out = append(out, p)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })

	return out
}

// partyNear lists the living party heroes within a Chebyshev radius of a subtile.
func (e *Engine) partyNear(x, y, r int) []*d2mapentity.Player {
	var out []*d2mapentity.Player

	for _, p := range e.partyMembers() {
		if chebyshev(int(p.Position.X())-x, int(p.Position.Y())-y) <= r {
			out = append(out, p)
		}
	}

	return out
}

// Heal implements d2missile.Healer for a party hero (Holy Bolt, 0x5a7a40, VERIFIED): the amount arrives in 8.8
// fixed point and the life is clamped to the maximum.
func (t *playerTarget) Heal(amount int) {
	if t.p.Stats == nil {
		return
	}

	t.p.Stats.Health = minInt(t.p.Stats.Health+amount>>8, t.p.Stats.MaxHealth)

	if t.e.OnPartyHeal != nil {
		t.e.OnPartyHeal(t.p, amount)
	}
}

var _ d2missile.Healer = (*playerTarget)(nil)
