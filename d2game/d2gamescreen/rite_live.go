package d2gamescreen

import (
	"errors"
	"sort"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// Rite of Passage in the running game (wave 29): the Arreat Summit altar, the three statues that wake the Ancients,
// the town-portal reset and the reward of the third Ancient's death. The rules are d2quest.AncientsFight (exe
// facts in d2quest/w27_quests2.go); this file feeds it with the engine's events.
//
// VERIFIED (0x589f10, 0x589e50): each statue wakes the Ancient of its own slot at its own coordinates, the statue
// then cannot be used again until the reset. UNVERIFIED: the altar is a level object, the harness places one when the
// map has none; the Ancients returning after a portal closes (the exe clears the armed flags 0x10/0x11 and the
// re-arm test is not decoded).

// riteLive is the Rite state of the game screen.
type riteLive struct {
	fight     d2quest.AncientsFight
	ancients  []*d2mapentity.Monster // the Ancients woken in the current level
	gateOpen  bool
	altarUsed bool                       // the altar (546) is in mode 1 (VERIFIED 0x58b0f0)
	slotUsed  [d2quest.AncientCount]bool // the statue of the slot was spoken to (VERIFIED 0x589f10: it goes to mode 4)
}

const (
	riteStatueFirst  = d2quest.NPCAncientStatue1
	riteStatueLast   = d2quest.NPCAncientStatue3
	riteSpawnRing    = 4
	riteAltarOffset  = 15
	riteFightDamage  = 1 << 20
	riteStatusFormat = "RITE status portals=%d spawned=%d deaths=%d armed=%v done=%v gate=%v"
)

// riteEnter runs when the hero arrives in a level.
func (v *Game) riteEnter(level int) {
	f := &v.rite.fight
	v.rite.ancients = nil
	v.rite.slotUsed = [d2quest.AncientCount]bool{}

	if level != d2quest.LevelSummit {
		return
	}

	// the map is new: the Ancients of an earlier visit are gone and the statues must be spoken to again (UNVERIFIED)
	if !f.Done {
		f.Armed, f.Spawned, f.Deaths = false, 0, 0
	}

	v.rite.gateOpen = false
	v.rite.altarUsed = false

	if v.monsters != nil {
		v.monsters.RiteReset()
		v.monsters.RiteSetPortals(f.Portals)
	}

	if len(v.objectsOf(d2quest.AncientsAltar)) == 0 {
		x, y := v.heroTilePos()
		if v.newObjectAt(d2quest.AncientsAltar, int(x*subtilesInTile)+riteAltarOffset, int(y*subtilesInTile)) != nil {
			v.Infof("RITE the level has no altar: one is placed next to the hero (fallback: the exe has no code that creates it, UNVERIFIED place)")
		}
	}

	if len(v.objectsOf(d2quest.AncientsDoor)) == 0 {
		x, y := v.heroTilePos()
		if v.newObjectAt(d2quest.AncientsDoor, int(x*subtilesInTile)+riteAltarOffset, int(y*subtilesInTile)+riteAltarOffset) != nil {
			v.Infof("RITE the level has no gate door: one is placed next to the hero (harness fallback, UNVERIFIED place)")
		}
	}

	v.Infof("RITE Arreat Summit: altar=%d door=%d done=%v portals=%d", len(v.objectsOf(d2quest.AncientsAltar)),
		len(v.objectsOf(d2quest.AncientsDoor)), f.Done, f.Portals)
}

// riteStart is the statues' introduction (VERIFIED 0x58a0c0 / 0x589d90: message 20002 with no portal open and no statue
// awake sets all three statue objects to mode 3 and arms the fight; 20 frames later each wakes its Ancient at its own
// place, 0x589f10; the 20 frame delay is not modelled here, UNVERIFIED as harmless): each statue wakes its Ancient while no town portal is open and fewer than
// three are out (AncientsMayStart), and the fight starts.
func (v *Game) riteStart() {
	f := &v.rite.fight

	if v.monsters == nil {
		return
	}

	if v.currentLevel() != d2quest.LevelSummit {
		v.monsters.RiteIntroHeard(true) // outside the summit the harness keeps its old behaviour

		return
	}

	if !f.MayStart() {
		v.Infof("RITE the statues stay asleep (portals=%d spawned=%d done=%v)", f.Portals, f.Spawned, f.Done)

		return
	}

	// VERIFIED 0x589f10: every statue wakes the Ancient of its own slot at the statue's own coordinates.
	for _, st := range v.riteStatues() {
		if !f.MayStart() {
			break
		}

		if st.slot < 0 || v.rite.slotUsed[st.slot] {
			continue
		}

		stat := v.monsters.FindStat(strconv.Itoa(d2quest.NPCAncient1 + st.slot))
		if stat == nil {
			v.Warningf("RITE no monstats row for class %d", d2quest.NPCAncient1+st.slot)

			continue
		}

		m, err := v.monsters.SpawnNear(stat, st.x, st.y, riteSpawnRing)
		if err != nil {
			v.Warningf("RITE Ancient slot %d: %v", st.slot, err)

			continue
		}

		f.Wake()
		v.rite.slotUsed[st.slot] = true
		v.rite.ancients = append(v.rite.ancients, m)
		v.Infof("RITE Ancient %d wakes at its statue (%s, class %d)", f.Spawned, m.Label(), stat.ID)
	}

	v.monsters.RiteSetPortals(f.Portals)
	started := v.monsters.RiteIntroHeard(true)
	v.Infof("RITE the fight starts: ancients=%d started=%v", len(v.rite.ancients), started)
}

// riteStatue is one statue of the summit: where it stands (subtiles) and the Ancient slot it wakes.
type riteStatue struct{ x, y, slot int }

// riteStatues lists the statues in the order slot 0, 1, 2 would wake: the statue objects (rows 474-476) of the map,
// else the statue NPCs of the harness; with neither, the three Ancients stand next to the hero (UNVERIFIED fallback,
// the exe has no statue-less spawn).
func (v *Game) riteStatues() []riteStatue {
	var out []riteStatue

	for _, row := range []int{d2quest.StatueObject1, d2quest.StatueObject2, d2quest.StatueObject3} {
		slot, _ := d2quest.AncientSlotForStatueObject(row)

		for _, ob := range v.objectsOf(row) {
			ox, oy := ob.GetPositionF()
			out = append(out, riteStatue{int(ox * subtilesInTile), int(oy * subtilesInTile), slot})
		}
	}

	if len(out) == 0 && v.monsters != nil {
		for _, m := range v.monsters.Monsters() {
			if slot, ok := d2quest.AncientSlotForStatueMonster(m.MonstatID()); ok {
				x, y := m.SubtilePos()
				out = append(out, riteStatue{x, y, slot})
			}
		}
	}

	if len(out) == 0 {
		hx, hy := v.heroTilePos()
		for slot := 0; slot < d2quest.AncientCount; slot++ {
			out = append(out, riteStatue{int(hx * subtilesInTile), int(hy * subtilesInTile), slot})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].slot < out[j].slot })

	return out
}

// riteSyncPortals follows the town portals open in level 120 (VERIFIED 0x58ace0 rules in AncientsFight).
func (v *Game) riteSyncPortals() {
	n := 0

	for i := range v.portal.pairs {
		if v.portal.pairs[i].Field.Level == d2quest.LevelSummit {
			n++
		}
	}

	f := &v.rite.fight
	reset, rearm := f.PortalsChanged(n)

	if v.monsters == nil || v.currentLevel() != d2quest.LevelSummit {
		return
	}

	v.monsters.RiteSetPortals(n)

	switch {
	case reset:
		for _, m := range v.rite.ancients {
			v.monsters.DismissMonster(m)
		}

		v.rite.ancients = nil
		v.rite.slotUsed = [d2quest.AncientCount]bool{}

		v.monsters.RiteReset()
		v.Infof("RITE a town portal opened in the summit: the Ancients are reset")
	case rearm:
		v.Infof("RITE the last portal closed: the fight is armed again")
		v.riteStart()
	}
}

// riteKilled is the death hook: the third Ancient ends the fight.
func (v *Game) riteKilled(ev d2monsters.KillEvent) {
	if !d2quest.IsAncientClass(ev.Class) || ev.Monster == nil {
		return
	}

	known := false

	for _, m := range v.rite.ancients {
		if m == ev.Monster {
			known = true
		}
	}

	if !known {
		return
	}

	f := &v.rite.fight
	if !f.AncientDied() {
		v.Infof("RITE Ancient died: %d of %d", f.Deaths, d2quest.AncientCount)

		return
	}

	v.Infof("RITE the third Ancient died: the fight is done")
	v.riteRewardParty()
}

// riteRewardParty names the heroes the death pays (VERIFIED 0x58a7b0: the killer and the heroes in level 120, and
// the party members in Act 5, at or above the level gate and without the done bit). The local hero's reward is the
// quest system's own (a5_w20.go); other players are only reported, their records live in their own games.
func (v *Game) riteRewardParty() {
	r := v.quests()
	if r == nil || v.gameClient == nil || v.gameClient.Roster == nil {
		return
	}

	diff := r.g.Difficulty

	for _, m := range v.gameClient.Roster.PartyMembers(v.me()) {
		if m.ID == v.me() {
			continue
		}

		if d2quest.RiteRewardRecipient(m.Level, diff, m.Area == d2quest.LevelSummit, false) {
			v.Infof("RITE party member %q (level %d) is paid by the Ancients", m.Name, m.Level)
		}
	}
}

// riteOperate handles the two summit objects (w39 correction, VERIFIED against the operate table at 0x730350: the
// table entry of OperateFn n is at 0x730350 + 4*(n-62), so fn 65 = 0x58b0f0 is the altar (546) and fn 66 = 0x58b1e0
// is the door (547); fn 67 = 0x588a60 is Frozen Anya, which cross-checks the base). It reports whether the object
// was one of them.
func (v *Game) riteOperate(ob *d2mapentity.Object) bool {
	switch ob.Record().Index {
	case d2quest.AncientsAltar:
		v.riteOperateAltar(ob)
	case d2quest.AncientsDoor:
		v.riteOperateDoor(ob)
	default:
		return false
	}

	return true
}

// riteOperateAltar is 0x58b0f0: an altar that was not used yet (mode 0) resets the Ancients when a town portal
// is open, speaks message 20002 (the Ancients' introduction, which starts the statue sequence) and goes to mode 1.
// The exe also moves the quest node to state 2 (message 0x59f) and queues the callback 0x589c70; the node bytes
// are UNVERIFIED and not modelled (the introduction path of the harness does the start).
func (v *Game) riteOperateAltar(ob *d2mapentity.Object) {
	if v.rite.altarUsed {
		v.Infof("RITE the altar was used already")

		return
	}

	v.rite.altarUsed = true

	if _, err := ob.Open(); err != nil {
		v.Warningf("RITE altar: %v", err)
	}

	v.Infof("RITE the altar is operated: the Ancients' introduction")
	v.riteStart()
}

// riteOperateDoor is 0x58b1e0 (the gate to the Worldstone Keep): works only for a hero whose Rite record has bit 0
// or bit 1 and not while the fight of this game is still going; otherwise nothing happens.
func (v *Game) riteOperateDoor(ob *d2mapentity.Object) {
	r := v.quests()
	open := r != nil && r.g.RiteAltarOpen() // both the done and the reward-pending bit

	if !v.rite.fight.AltarWorks(open, false) {
		v.Infof("RITE the door does nothing (rite open=%v, fight running=%v)", open, v.rite.fight.Running())

		return
	}

	if opened, err := ob.Open(); err != nil {
		v.Warningf("RITE door: %v", err)
	} else if !opened {
		v.Infof("RITE the door is already open")
	}

	v.riteOpenGate()
	v.Infof("RITE the door opens the way to the Worldstone Keep")
}

// riteOpenGate opens the door objects to the Worldstone Keep (the level gate itself follows the quest bits).
func (v *Game) riteOpenGate() {
	v.rite.gateOpen = true

	for _, d := range v.objectsOf(d2quest.AncientsDoor) {
		if _, err := d.Open(); err != nil {
			v.Warningf("RITE gate: %v", err)
		}
	}
}

func (v *Game) commandRiteStart(_ []string) error {
	v.riteStart()

	return nil
}

func (v *Game) commandRiteDoor(_ []string) error {
	d := v.objectsOf(d2quest.AncientsDoor)
	if len(d) == 0 {
		return errors.New("no gate door in this level")
	}

	v.riteOperate(d[0])

	return nil
}

func (v *Game) commandRiteAltar(_ []string) error {
	alt := v.objectsOf(d2quest.AncientsAltar)
	if len(alt) == 0 {
		return errors.New("no Ancients' altar in this level")
	}

	v.riteOperate(alt[0])

	return nil
}

// commandRiteKill kills the Ancients that stand, as the hero.
func (v *Game) commandRiteKill(_ []string) error {
	if v.monsters == nil {
		return errors.New("no monsters")
	}

	n := 0

	for _, m := range append([]*d2mapentity.Monster(nil), v.rite.ancients...) {
		if m.Alive() {
			v.monsters.Damage(m, riteFightDamage, v.localPlayer)

			n++
		}
	}

	if n == 0 {
		return errors.New("no living Ancient")
	}

	return nil
}

func (v *Game) commandRiteStatus(_ []string) error {
	f := v.rite.fight
	v.Infof(riteStatusFormat, f.Portals, f.Spawned, f.Deaths, f.Armed, f.Done, v.rite.gateOpen)

	return nil
}
