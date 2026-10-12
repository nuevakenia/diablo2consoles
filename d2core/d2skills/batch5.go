package d2skills

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
)

// Engine side of batch 5 (docs: d2-re-notes/skills-batch5.md): the corpse facts and the ground item the
// cast target carries.

// corpseFacts describes a corpse for the raise / revive tests (d2skill.CorpseRaisable, CorpseExplodable). The
// selectable flag is the corpseSel column of monstats2 (bit 7 of the flag word, VERIFIED mapping, 0x658ee0); a monster
// without a monstats2 row counts as selectable.
func (e *Engine) corpseFacts(m *d2mapentity.Monster) *d2skill.CorpseFacts {
	f := &d2skill.CorpseFacts{Monster: true, Dead: true, Selectable: true}

	if st := e.sets[m.ID()]; st != nil {
		f.InStateMask150 = st.HasUDead(e.frame) // the states.txt udead column (0x63b870, VERIFIED)
	}

	if m.Stat == nil {
		f.WalkSpeed = 1

		return f
	}

	f.WalkSpeed = m.Stat.SpeedBase

	if e.asset != nil && e.asset.Records != nil && e.asset.Records.Monster.Stats2 != nil {
		if s2 := e.asset.Records.Monster.Stats2[m.Stat.ExtraDataKey]; s2 != nil {
			f.Selectable = s2.IsCorpseSelectable
		}
	}

	return f
}

// groundItemNear is the ground item nearest (sx, sy) within pickRadius, or nil. Valid says whether Iron Golem may
// use it: SRVST_020 (0x5c1070) tests a base-table flag and the item flag mask 0x10 / 0x100, whose meaning was not
// read; the port accepts weapons and armor (the items a golem can wear), UNVERIFIED.
func (e *Engine) groundItemNear(sx, sy int) *d2skill.GroundItem {
	if e.mapEngine == nil {
		return nil
	}

	var best *d2skill.GroundItem

	bd := pickRadius + 1

	for id, ent := range e.mapEngine.Entities() {
		it, ok := ent.(*d2mapentity.Item)
		if !ok || it.IsGold() || it.Item == nil {
			continue
		}

		x, y := int(it.Position.X()), int(it.Position.Y())

		d := chebyshev(x-sx, y-sy)
		if d > bd || (d == bd && best != nil && id >= best.ID) {
			continue
		}

		bd = d
		kind := it.Item.GetInventoryItemType()
		best = &d2skill.GroundItem{ID: id, X: x, Y: y, Kind: it.Item.GetItemCode(),
			Valid: kind == d2enum.InventoryItemTypeArmor || kind == d2enum.InventoryItemTypeWeapon}
	}

	return best
}

// takeGroundItem removes the item an Iron Golem consumed; false when it is gone.
func (e *Engine) takeGroundItem(id string) bool {
	if e.mapEngine == nil || id == "" {
		return false
	}

	ent, ok := e.mapEngine.Entities()[id]
	if !ok {
		return false
	}

	e.mapEngine.RemoveEntity(ent)

	return true
}

// armCaster makes a summoned caster minion (the Skeletal Mage, sumskill1 NecromageMissile) fire the missile of its
// summon skill when its pet AI casts, through the Director's SkillFirer, instead of the melee blow the monstats
// attack would give (the unit has a ranged first skill; MONAI_Pet_ThinkCaster 0x5e3a00 casts slot 0). The missile
// is the one of the sumskill (srvmissilea of NecromageMissile = necromage1), the damage that of the summoning
// skill's descriptor (U: the exe takes it from the monster's own skill level, sumsk1calc).
func (e *Engine) armCaster(sk *d2skill.Skill, o *d2skill.SummonOrder, m *d2mapentity.Monster) {
	if sk.SrvDoFunc != 31 || o.TrapSkill == "" {
		return
	}

	ts := e.pipe.Skills.ByName(o.TrapSkill)
	if ts == nil {
		return
	}

	missile := ts.SrvMissile
	if missile == "" {
		missile = ts.SrvMissileA
	}

	if missile == "" {
		return
	}

	// Wave 23 (VERIFIED): the mage type 0..3 is the low two bits of a roll of the unit's seed taken when the mage is
	// raised (SKILL_RaiseSkeleton_ApplyMasteryTable 0x5c1fd0, data bytes +0xf / +0x10) and SRVDO_149 0x5cc400 adds
	// it to srvmissilea: necromage1..4. The damage is the missile's own, at the level sumsk1calc.
	if o.TrapLevel < 1 {
		return // sumsk1calc <= 0: SKILL_AddSkillToUnit is not called (0x5c22c0), the mage has no spell
	}

	kind := 0
	if e.monSeed != nil {
		kind = int(e.monSeed.Step() & 3)
	}

	missile = d2skill.MageMissile(e.pipe.Missiles, missile, kind)

	if e.monsters.ArmTrap(m, d2monsters.TrapSpec{SkillID: sk.ID, SkillName: sk.Name, Missile: missile, OwnLevel: o.TrapLevel}) {
		e.emit("summon", "CASTER armed skill=%q missile=%s type=%d level=%d", sk.Name, missile, kind, o.TrapLevel)
	}
}
