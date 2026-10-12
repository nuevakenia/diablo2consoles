package d2gamescreen

import (
	"errors"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2herostats"
)

// commandCubeTake is "cubetake": everything in the Horadric Cube goes back into the inventory (the Horadric Staff has to be
// carried to Tal Rasha's orifice, not left in the cube).
func (v *Game) commandCubeTake(_ []string) error {
	if v.gameControls == nil {
		return errors.New("not in a game")
	}

	v.Infof("CUBE took %d item(s) out of the cube", v.gameControls.CubeTakeAll())
	v.syncQuestItems()

	return nil
}

// commandDumpJunk is "dumpjunk": drops the loot that is not a quest item, potion, scroll or key (the pack is full).
func (v *Game) commandDumpJunk(_ []string) error {
	if v.gameControls == nil {
		return errors.New("not in a game")
	}

	keep := append([]string{}, questItemCodes...)
	keep = append(keep, "box", "msf", "vip", "leg", "hdm", "hfh", "hst")

	v.Infof("DUMPJUNK removed %d item(s)", v.gameControls.DumpJunk(keep))

	return nil
}

// commandAutoBuild is "autobuild": spends the unspent points by the OD2_AUTOBUILD plan (a quest reward skill point
// does not come with a level-up).
func (v *Game) commandAutoBuild(_ []string) error {
	if v.gameControls == nil {
		return errors.New("not in a game")
	}

	v.gameControls.AutoBuild()

	return nil
}

// The hero's level follows his experience: the monster kills only add
// experience points (d2monsters), the level-up itself (the new level, one skill
// point and five stat points, full life and mana) happens here.

const heroLevelUpSound = "cursor_level_up" // Sounds.txt: cursor\levelup.wav

// advanceHeroLevel promotes the hero while his experience reaches the next
// level's breakpoint (several levels at once when a big kill allows it).
func (v *Game) advanceHeroLevel() {
	p := v.localPlayer
	if p == nil || p.Stats == nil || v.gameControls == nil || p.IsDead() {
		return
	}

	st := p.Stats
	recs := v.asset.Records
	maxLevel := recs.GetMaxLevelByHero(p.Class)

	if st.NextLevelExp == 0 { // heroes made before the breakpoint was filled in
		st.NextLevelExp = recs.GetExperienceBreakpoint(p.Class, st.Level)
	}

	// experience is capped at the threshold of row MaxLvl-1 (VERIFIED 0x0057c510)
	tbl := d2herostats.NewExpTable(maxLevel, func(l int) int64 { return int64(recs.GetExperienceBreakpoint(p.Class, l)) })
	if maxLevel >= 2 && tbl.Threshold[maxLevel-1] > 0 {
		capped, _ := tbl.ApplyExperience(st.Level, int64(st.Experience))
		st.Experience = int(capped)
	}

	gained := levelsGained(st.Experience, st.Level, maxLevel, func(l int) int { return recs.GetExperienceBreakpoint(p.Class, l) })

	if gained == 0 {
		return
	}

	v.gameControls.GrantLevels(gained)
	v.gameControls.AutoBuild() // OD2_AUTOBUILD: a script's plan for the new points (no effect when unset)

	if st.Recalc != nil {
		st.Recalc() // life and mana maxima follow the level
	}

	st.Health, st.Mana = st.MaxHealth, st.MaxMana // a level-up refills both

	v.playSoundAt(heroLevelUpSound, p.GetPosition(), "levelup")
	v.Infof("HERO LEVEL UP +%d level=%d exp=%d next=%d skillpoints=%d statpoints=%d life=%d mana=%d", gained, st.Level,
		st.Experience, st.NextLevelExp, st.SkillPoints, st.StatsPoints, st.MaxHealth, st.MaxMana)

	_ = v.OnPlayerSave()
}

// levelsGained counts the levels a hero of the given level and experience has
// earned: breakpoint(l) is the experience needed to leave level l (0 or less:
// no further level).
func levelsGained(exp, level, maxLevel int, breakpoint func(level int) int) int {
	gained := 0

	for lvl := level; lvl < maxLevel; lvl++ {
		need := breakpoint(lvl)
		if need <= 0 || exp < need {
			break
		}

		gained++
	}

	return gained
}
