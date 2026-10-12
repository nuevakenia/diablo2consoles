package d2app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
)

// autoCastClass resolves OD2_AUTOCAST_CLASS=<class> to a hero class.
func autoCastClass(name string) (d2enum.Hero, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "barbarian", "bar":
		return d2enum.HeroBarbarian, true
	case "necromancer", "nec":
		return d2enum.HeroNecromancer, true
	case "paladin", "pal":
		return d2enum.HeroPaladin, true
	case "assassin", "ass":
		return d2enum.HeroAssassin, true
	case "sorceress", "sor":
		return d2enum.HeroSorceress, true
	case "amazon", "ama":
		return d2enum.HeroAmazon, true
	case "druid", "dru":
		return d2enum.HeroDruid, true
	}

	return d2enum.HeroNone, false
}

// freshHeroSave implements OD2_AUTOCAST_CLASS=<class>: a brand new hero of that
// class (the same state the character creation screen builds), saved to a
// temporary file so no real save is touched. OD2_AUTOCAST_CLVL=<n> sets the
// character level (default 18, the level the class skill scenario targets).
// The skill scenario (OD2_AUTOCAST) grants whatever skills it casts.
func (a *App) freshHeroSave(class string) (string, error) {
	hero, ok := autoCastClass(class)
	if !ok {
		return "", fmt.Errorf("unknown class %q", class)
	}

	factory, err := d2hero.NewHeroStateFactory(a.asset)
	if err != nil {
		return "", err
	}

	stats := factory.CreateHeroStatsState(hero, a.asset.Records.Character.Stats[hero])

	level := 18
	if n, err := strconv.Atoi(os.Getenv("OD2_AUTOCAST_CLVL")); err == nil && n > 0 {
		level = n
	}

	stats.Level = level

	// OD2_AUTOCAST_SKILLPOINTS=<n> hands the fresh hero n unspent skill points (scripts that skill:spend)
	if n, err := strconv.Atoi(os.Getenv("OD2_AUTOCAST_SKILLPOINTS")); err == nil && n > 0 {
		stats.SkillPoints = n
	}

	// OD2_AUTOCAST_MANA=<n> gives the fresh hero a full pool of n mana: stats.Level is raised after the stats were built
	// at level 1, so without it the hero starts with the level-1 mana (22 of 54 at level 18) and scripts that cast
	// several 15-mana forms are refused for mana (w39, 86k Wearbear step).
	if n, err := strconv.Atoi(os.Getenv("OD2_AUTOCAST_MANA")); err == nil && n > 0 {
		stats.Mana, stats.MaxMana = n, n
	}

	state, err := factory.CreateHeroState("Auto"+hero.String(), hero, stats)
	if err != nil {
		return "", err
	}

	state.FilePath = filepath.Join(os.TempDir(), "od2-autocast-"+strings.ToLower(hero.String())+".od2")

	if err = factory.Save(state); err != nil {
		return "", err
	}

	a.Infof("AUTOCAST_CLASS fresh %v level %d, %d skills -> %s", state.HeroType, level, len(state.Skills), state.FilePath)

	return state.FilePath, nil
}
