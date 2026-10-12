package d2inventory

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// What the exe does when a healing or mana potion (pSpell 3) is drunk, read in
// ITEM_ServerUsePotionVariantA 0x5bbf70 and its two scaling helpers
// PLAYER_ScaleValueByClassGroupA 0x62a730 / ...GroupB 0x62a780 (all VERIFIED
// from the disassembly; the stat ids are the ItemStatCost rows):
//
//   - the per-potion amount is the calc column evaluated; for the stats
//     hpregen (0x4a) and manarecovery (0x1a) it is first shifted left by 8.
//   - life stats (hitpoints 6, hpregen 0x4a) go through group A, mana stats
//     (mana 8, manarecovery 0x1a) through group B. The group multiplies the
//     amount by a factor that depends on the drinker's class: see PotionClassScale.
//   - then comes a vitality (life) or energy (mana) roll: r = rand % stat,
//     d = rand % 100, and when d < r/2 the amount is doubled. Both rolls use the
//     drinker's own unit seed (RAND_RollSeedModulo 0x457b60 and
//     RAND_RollSeedBounded 0x457b10, the same LCG, so the seed advances twice).
//     The stat is the unit's total stat 3 (vitality) or 1 (energy).
//   - rejuvenation (pSpell 5, ITEM_ServerUsePotionVariantC 0x5bc680) is a percent
//     of the stat's maximum with none of the above; the golem share applies to
//     both when the stat is life.
//
// The over-time delivery (stat list with the item's state, expiring len frames
// later, merged on a second drink) is Regen in potion.go.

// PotionGroup selects the class factor table of the exe.
type PotionGroup int

// The two tables.
const (
	PotionGroupLife PotionGroup = iota // PLAYER_ScaleValueByClassGroupA
	PotionGroupMana                    // PLAYER_ScaleValueByClassGroupB
)

// ExeClass is the exe's class number: 0 Amazon, 1 Sorceress, 2 Necromancer,
// 3 Paladin, 4 Barbarian, 5 Druid, 6 Assassin.
func ExeClass(h d2enum.Hero) int {
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

// PotionClassScale applies the class factor of a group to an amount. player is
// false for a non-player drinker (a mercenary): life doubles, mana stays. For a
// player the factors are: life x1.5 for Amazon, Paladin and Assassin (v + v/2 with
// an arithmetic shift), x2 for the Barbarian, x1 for the others; mana x1.5 for
// Amazon, Paladin and Assassin, x2 for Sorceress, Necromancer and Druid, x1 for
// the Barbarian.
func PotionClassScale(group PotionGroup, class int, player bool, v int) int {
	if !player {
		if group == PotionGroupLife {
			return v * 2
		}

		return v
	}

	switch class {
	case 0, 3, 6:
		return (v >> 1) + v
	}

	if group == PotionGroupLife {
		if class == 4 {
			return v * 2
		}

		return v
	}

	switch class {
	case 1, 2, 5:
		return v * 2
	}

	return v
}

// PotionDoubles is the vitality / energy roll: with stat > 0 it draws r in
// [0, stat) and d in [0, 100) from the drinker's seed and answers d < r/2. The
// two draws always happen together (in that order); stat <= 0 draws nothing.
func PotionDoubles(roll func(n int) int, stat int) bool {
	if stat <= 0 || roll == nil {
		return false
	}

	r := roll(stat)
	d := roll(100)

	return d < r>>1
}

// ScaledPotion returns the over-time amounts of an effect after the class
// factors and the doubling rolls of the exe. life and mana are the drinker's
// total vitality and energy; roll is the drinker's seeded draw. Rejuvenation
// (instant percent) is not touched.
func ScaledPotion(e PotionEffect, class int, player bool, vitality, energy int, roll func(n int) int) PotionEffect {
	if e.HP > 0 {
		v := PotionClassScale(PotionGroupLife, class, player, int(e.HP))
		if player && PotionDoubles(roll, vitality) {
			v *= 2
		}

		e.HP = float64(v)
	}

	if e.Mana > 0 {
		v := PotionClassScale(PotionGroupMana, class, player, int(e.Mana))
		if player && PotionDoubles(roll, energy) {
			v *= 2
		}

		e.Mana = float64(v)
	}

	return e
}
