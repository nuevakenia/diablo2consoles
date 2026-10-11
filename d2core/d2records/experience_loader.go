package d2records

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2herostats"
)

/*	first column of experience.txt
	Level
	Amazon
	Sorceress
	Necromancer
	Paladin
	Barbarian
	Druid
	Assassin
	ExpRatio

	second row is special case, shows max levels

	MaxLvl
	99
	99
	99
	99
	99
	99
	99
	10

	the rest are the breakpoints records
*/

func experienceLoader(r *RecordManager, d *d2txt.DataDictionary) error {
	breakpoints := make(ExperienceBreakpoints)

	d.Next() // move to the first row, the max level data

	// parse the max level data
	maxLevels := ExperienceMaxLevels{
		d2enum.HeroAmazon:      d.Number("Amazon"),
		d2enum.HeroBarbarian:   d.Number("Barbarian"),
		d2enum.HeroDruid:       d.Number("Druid"),
		d2enum.HeroAssassin:    d.Number("Assassin"),
		d2enum.HeroNecromancer: d.Number("Necromancer"),
		d2enum.HeroPaladin:     d.Number("Paladin"),
		d2enum.HeroSorceress:   d.Number("Sorceress"),
	}

	shift := d.Number("ExpRatio")
	ratios := []int{}

	for d.Next() {
		record := &ExperienceBreakpointRecord{
			Level: d.Number("Level"),
			HeroBreakpoints: map[d2enum.Hero]int{
				d2enum.HeroAmazon:      d.Number("Amazon"),
				d2enum.HeroBarbarian:   d.Number("Barbarian"),
				d2enum.HeroDruid:       d.Number("Druid"),
				d2enum.HeroAssassin:    d.Number("Assassin"),
				d2enum.HeroNecromancer: d.Number("Necromancer"),
				d2enum.HeroPaladin:     d.Number("Paladin"),
				d2enum.HeroSorceress:   d.Number("Sorceress"),
			},
			Ratio: d.Number("ExpRatio"),
		}
		breakpoints[record.Level] = record

		for len(ratios) <= record.Level {
			ratios = append(ratios, 0)
		}

		ratios[record.Level] = record.Ratio
	}

	if d.Err != nil {
		return d.Err
	}

	r.Debugf("Loaded %d ExperienceBreakpoint records", len(breakpoints))

	// Kill experience is scaled by the recipient's ExpRatio (0x57c3a0). Only
	// patch_d2's table has the column; without it nothing is installed.
	if shift > 0 && len(ratios) > 1 && ratios[1] > 0 {
		d2herostats.SetExpRatio(&d2herostats.ExpRatio{Shift: shift, ByLevel: ratios})
	} else {
		d2herostats.SetExpRatio(nil)
	}

	r.Character.MaxLevel = maxLevels
	r.Character.Experience = breakpoints

	return nil
}
