package d2monsters

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2path"
)

// Wave 20 hosts of the Act 5 / endgame AIs (notes: w20-ai-act5.md): the owner
// test of the Baal tentacle and clone, the hero mode and the hop of the
// taunter, the taunt cast, the state tests of the throne and the Ancient, and
// the Rite of Passage fight state.

var (
	_ d2monster.OwnerChecker          = (*Director)(nil)
	_ d2monster.TargetModer           = (*Director)(nil)
	_ d2monster.SelfMover             = (*Director)(nil)
	_ d2monster.RawCaster             = (*Director)(nil)
	_ d2monster.TargetStater          = (*Director)(nil)
	_ d2monster.AuraTargetStateSource = (*Director)(nil)
	_ d2monster.RiteOfPassage         = (*Director)(nil)
)

// OwnerGone implements d2monster.OwnerChecker (UNIT_FindOwnerUnitIfFlagged +
// the dead test): a summoned unit's owner is the monster that summoned it (the
// Baal clone, the tentacle) or the hero of a minion. A unit with no recorded
// owner is treated as owned (alive), so units the harness places by hand are
// not dismissed (the exe would also dismiss an ownerless tentacle).
func (d *Director) OwnerGone(b *d2monster.Brain) bool {
	u := d.unitOf(b)
	if u == nil {
		return false
	}

	if u.summoner != 0 {
		o := d.units[u.summoner]

		return o == nil || !o.m.Alive()
	}

	return ownerGone(u.ally)
}

// TargetMode implements d2monster.TargetModer: a hero is in the neutral mode
// (1) when he is standing still and not casting; moving or casting is a walk,
// a dead hero the death mode. Monsters report their brain mode.
func (d *Director) TargetMode(_ *d2monster.Brain, t d2monster.Target) d2monster.Mode {
	if p := d.playerFor(t.ID); p != nil {
		if p.IsDead() || p.IsDying() {
			return d2monster.ModeDying
		}

		v := p.GetVelocity()
		if p.IsCasting() || !v.IsZero() {
			return d2monster.ModeWalk
		}

		return d2monster.ModeNeutral
	}

	if t.ID >= unitTargetBase {
		if tu := d.units[t.ID-unitTargetBase]; tu != nil {
			return tu.b.Mode
		}
	}

	return d2monster.ModeNeutral
}

// MoveSelfToTarget implements d2monster.SelfMover (SERVER_MoveUnitToLevelPosition
// of the taunter onto the hero): the unit lands on the free cell nearest the
// target within 3 subtiles.
func (d *Director) MoveSelfToTarget(b *d2monster.Brain, t d2monster.Target) bool {
	u := d.unitOf(b)
	if u == nil || d.fp == nil {
		return false
	}

	p, ok := d2path.NearestFree(d.fp, d2path.MaskMonster, d2path.Point{X: t.X, Y: t.Y}, 3)
	if !ok {
		return false
	}

	u.m.TeleportTo(p.X, p.Y)
	b.X, b.Y = p.X, p.Y
	d.footprint(u)
	u.mv = nil
	b.WakeNow(d.frame)

	return true
}

// CastSkillID implements d2monster.RawCaster for the skills the endgame AIs
// name directly. Only Baal Taunt (284) is simulated: it is a voice line with
// no damage, so the cast is traced and accepted. Other ids report false and
// the AIs fall back as before.
func (d *Director) CastSkillID(b *d2monster.Brain, skill int, _ d2monster.Mode, t *d2monster.Target, _ *d2monster.Point) bool {
	if skill != d2monster.SkillBaalTaunt {
		return false
	}

	if u := d.unitOf(b); u != nil {
		id := uint32(0)
		if t != nil {
			id = t.ID
		}

		d.emit("boss", "BAAL taunt name=%s id=%d target=%d", u.m.Label(), b.ID, id)
	}

	return true
}

// TargetHasState implements d2monster.TargetStater. Monsters answer from the
// AI-set states; heroes carry no AI states in this engine.
func (d *Director) TargetHasState(_ *d2monster.Brain, t d2monster.Target, state int) bool {
	if t.ID >= unitTargetBase {
		if tu := d.units[t.ID-unitTargetBase]; tu != nil {
			return tu.states[state]
		}
	}

	return false
}

// SlotAuraTargetState implements d2monster.AuraTargetStateSource: the
// states.txt id of the skills.txt auratargetstate of the skill in the slot,
// -1 when none.
func (d *Director) SlotAuraTargetState(b *d2monster.Brain, slot int) int {
	if b == nil || b.Profile == nil || slot < 0 || slot >= d2monster.NumSkills ||
		!b.Profile.Skills[slot].Used() || d.asset == nil || d.asset.Records == nil {
		return -1
	}

	rec := d.asset.Records.GetSkillByName(b.Profile.Skills[slot].Name)
	if rec == nil || strings.TrimSpace(rec.Auratargetstate) == "" {
		return -1
	}

	name := strings.TrimSpace(rec.Auratargetstate)
	if st, ok := d.asset.Records.States[strings.ToLower(name)]; ok && st != nil {
		return st.ID
	}

	if st, ok := d.asset.Records.States[name]; ok && st != nil {
		return st.ID
	}

	return -1
}

// RiteState is the Rite of Passage fight state the Ancients and their statues
// read (quest node 0x23 data: byte +0x11 "fight started", word +0xc portals).
type RiteState struct {
	// Started is the byte at +0x11 of the quest node data. The exe's
	// IsAncientFightActive is its negation (the Ancients wait until it is set).
	Started bool
	// Portals is the portal count at +0xc.
	Portals int
}

// SetRiteState installs the Rite of Passage state. Without it the Ancients
// fight at once and the statues never cast (no quest host).
func (d *Director) SetRiteState(r *RiteState) { d.rite = r }

// AncientFightActive implements d2monster.RiteOfPassage with the exe's own
// (misleading) name: true while the fight has NOT started and the Ancients
// stand idle. Absent state: false (they fight).
func (d *Director) AncientFightActive() bool { return d.rite != nil && !d.rite.Started }

// FightDone implements d2monster.RiteOfPassage (the same byte, set).
func (d *Director) FightDone() bool { return d.rite != nil && d.rite.Started }

// PortalCount implements d2monster.RiteOfPassage.
func (d *Director) PortalCount() int {
	if d.rite == nil {
		return 0
	}

	return d.rite.Portals
}
