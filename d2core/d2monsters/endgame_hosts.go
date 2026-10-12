package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 22 hosts of the endgame AIs (notes: w22-monsters.md): the stats of a hero that Diablo, the Baal crab and the
// Ancients score (MONAI_Diablo_ScoreTargetResists 0x5e7420, the crab's copy 0x5fac60), the last town portal of a hero
// (PLAYER_GetLastPortalUnitId), the cold state test, the Rite of Passage start and the Mephisto scan of hostile units.

var _ d2monster.DiabloPortalSource = (*Director)(nil)

// Stat ids (itemstatcost) the score reads that d2statlist has no name for.
const (
	statMagicMaxDam = 53 // magicmaxdam, the exe's 0x35
	statColdState   = 0xb
	statPoisonLen   = 59 // poisonlength
)

// heroStat is a stat of the hero's summed list (items, buffs and curses), 0 when she has no totals.
func heroStat(p *d2mapentity.Player, id int) int {
	if p == nil || p.Stats == nil || p.Stats.Totals == nil || p.Stats.Totals.Stats == nil {
		return 0
	}

	return int(p.Stats.Totals.Stats.Get(id))
}

// heroHasState asks the host whether the hero has a state; without a host no hero has one.
func (d *Director) heroHasState(p *d2mapentity.Player, state int) bool {
	return p != nil && d.HeroHasState != nil && d.HeroHasState(p, state)
}

// fillHeroDiaStats adds the stats the Director did not provide before wave 22 to a Diablo candidate: the physical and
// magic resistance (0x24, 0x25 as the raw list values, the exe reads the stat list, not the capped display values),
// the damage group (0x16 flat maximum damage, 0x31, 0x33, 0x35, 0x37 the elemental and magic maximums, 0x3a the poison
// maximum) and state 0xb (cold).
//
// The exe stores the poison damage as a rate of 1/256 per frame and reads (stat >> 8). The hero list holds the total,
// so the rate per frame is total / length (UNVERIFIED mapping, the term is tiny: poison adds at most a few points to
// a score that is divided by 22).
func (d *Director) fillHeroDiaStats(p *d2mapentity.Player, st *d2monster.DiaStats) {
	st.S24 = heroStat(p, d2statlist.StatDamageResist)
	st.S25 = heroStat(p, d2statlist.StatMagicResist)
	st.S16 = heroStat(p, d2statlist.StatMaxDamage)
	st.S31 = heroStat(p, d2statlist.StatFireMax)
	st.S33 = heroStat(p, d2statlist.StatLightMax)
	st.S35 = heroStat(p, statMagicMaxDam)
	st.S37 = heroStat(p, d2statlist.StatColdMax)

	if total := heroStat(p, d2statlist.StatPoisonMax); total > 0 {
		length := heroStat(p, statPoisonLen)
		if length < 1 {
			length = 1
		}

		st.S3A = total / length
	}

	st.HasState0xb = d.heroHasState(p, statColdState)
}

// LastPortal implements d2monster.DiabloPortalSource: the hero's last town portal. The exe stores the portal object's
// unit id when the hero uses a town portal in a field level (PLAYER_TeleportToTownOnTownPortalUse) and finds the object
// again; the host supplies its level and place.
func (d *Director) LastPortal(_ *d2monster.Brain, t d2monster.Target) (id uint32, p d2monster.Point, levelID int, ok bool) {
	if !t.IsPlayer || d.HeroPortal == nil {
		return 0, d2monster.Point{}, 0, false
	}

	hero := d.playerFor(t.ID)
	if hero == nil {
		return 0, d2monster.Point{}, 0, false
	}

	lvl, x, y, found := d.HeroPortal(hero)
	if !found {
		return 0, d2monster.Point{}, 0, false
	}

	return uint32(t.ID), d2monster.Point{X: x, Y: y}, lvl, true
}

// baalWorldstoneInput is the crab's "worldstone" input (0x5fb830): the target is a hero, her last town portal lies in
// level 0x84 and, when the crab has an anchor, within 0x4b of it.
func baalWorldstoneInput(level, px, py int, ax, ay int, hasAnchor bool) bool {
	if level != worldstoneChamberLevel {
		return false
	}

	return !hasAnchor || d2monster.Distance(px-ax, py-ay) < baalPortalNear
}

// baalPortalNear is the distance below which the portal counts as near the anchor (0x4b).
const baalPortalNear = 0x4b

// RiteIntroHeard is the Ancients' start (QUEST_A5_RiteOfPassage_OnNpcMessageHeard 0x58a0c0, message 0x4e22 = 20002, the
// statues' introduction): when no portal was counted and none of the three statues is awake, the exe wakes the three
// statues (mode 3), clears their footprint and sets the "fight started" bytes (+0x10, +0x11) of the quest node. From
// then on QUEST_A5_RiteOfPassage_IsAncientFightActive (byte +0x11 == 0) is false and the Ancients attack. statuesFound
// is false when the host cannot find the three statue objects (the exe then does nothing). Returns whether the fight
// started.
func (d *Director) RiteIntroHeard(statuesFound bool) bool {
	if d.rite == nil {
		d.rite = &RiteState{}
	}

	r := d.rite
	if r.Portals > 0 || r.Statues != [3]int{} || !statuesFound {
		return false
	}

	r.Statues = [3]int{3, 3, 3}
	r.Started = true

	d.emit("boss", "RITE fight started: the Ancients leave their statues")

	return true
}

// noteSpawnForRite makes the Ancients wait: the exe tests the quest node data (+0x11 == 0), which is clear in a new
// game, so Ancients created before the introduction stand idle. A host that never hears the introduction (no quest
// system) leaves the state nil and the Ancients fight at once, as before.
func (d *Director) noteSpawnForRite(class int) {
	if d.rite == nil && class >= riteAncientFirst && class <= riteAncientLast {
		d.rite = &RiteState{}
	}
}

// The monstats rows of the three Ancients (exe classes 540..542).
const (
	riteAncientFirst = 540
	riteAncientLast  = 542
)
