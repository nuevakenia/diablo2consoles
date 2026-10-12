package d2skills

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2animspeed"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

// itemIASStat is item_fasterattackrate, stat 93 (VERIFIED: row 0 of the speed stat table at 0x6ea3d4).
const itemIASStat = 93

// exeClassIndex is the hero class number the exe uses in its tables (Amazon 0, Sorceress 1, Necromancer 2,
// Paladin 3, Barbarian 4, Druid 5, Assassin 6).
func exeClassIndex(h d2enum.Hero) int {
	switch h {
	case d2enum.HeroAmazon:
		return 0
	case d2enum.HeroSorceress:
		return 1
	case d2enum.HeroNecromancer:
		return 2
	case d2enum.HeroPaladin:
		return 3
	case d2enum.HeroBarbarian:
		return 4
	case d2enum.HeroDruid:
		return 5
	case d2enum.HeroAssassin:
		return 6
	}

	return -1
}

// actionClock is the Pipeline.Clock hook: the action events of the hero's attack animation (wave 25, see
// d2skill/actionevents.go). The flags and the frame count are those of the AnimData record of the hero's A1
// mode and weapon class (the key the composite loads), the start frame is the exe's UNIT_GetAttackFrameCount
// table, the rate is the attack rate of the AnimData speed with the worn items' IAS. A sequence skill (Jab)
// plays the rows of its sequence at the same rate. UNVERIFIED: stat 0x44 is taken as 100 (weapon speed
// modifier and skill speed bonuses are not applied), the sequence rate (the exe sets it in mode 0x12 through
// the "other mode" rule, stat 0x45), and the dual wield terms. Mercenaries and heroes without AnimData get no
// clock (the burst then uses d2skill.FallbackBurstGap).
func (e *Engine) actionClock(u d2skill.Unit, sk *d2skill.Skill) *d2skill.ActionClock {
	h, ok := u.(*heroUnit)
	if !ok || h.merc != nil || h.p == nil || e.asset == nil || e.asset.Records == nil {
		return nil
	}

	data := e.asset.Records.Animation.Data
	if data == nil || h.p.Class == d2enum.HeroNone {
		return nil
	}

	wc := strings.ToLower(h.p.WeaponClass())

	recs := data.GetRecords(strings.ToUpper(h.p.Class.GetToken() + "A1" + wc))
	if len(recs) == 0 {
		return nil
	}

	rec := recs[0]

	var (
		flags  []byte
		frames int
		start  int
	)

	if seq := d2skill.SkillSequence(sk); seq != 0 {
		rows, found := d2skill.Sequence(seq, wc)
		if !found {
			return nil
		}

		flags, frames = rows.Flags(), rows.Rows
	} else {
		frames = rec.FramesPerDirection()
		flags = make([]byte, frames)

		for i := range flags {
			flags[i] = byte(rec.Event(i))
		}

		start = d2skill.AttackStartFrame(exeClassIndex(h.p.Class), wc)
	}

	ias, stat44 := 0, d2animspeed.AttackBasePct
	if t := h.totals(); t != nil && t.Stats != nil {
		ias = int(t.Stats.Get(itemIASStat))
		// stat 0x44 starts at 100 and the weapon adds -WSM (weapons.txt speed, ITEMGEN_InitItemBaseStats
		// 0x555b10 VERIFIED). UNVERIFIED here: the skill speed term (Zeal, Fury, Dragon Talon...) is not added.
		stat44 -= t.WeaponSpeed
	}

	rate := d2animspeed.AttackRate(rec.Speed(), stat44, ias, 0, false)

	return d2skill.NewActionClock(flags, start, rate, frames)
}
