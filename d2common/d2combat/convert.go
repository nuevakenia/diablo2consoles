package d2combat

// ConvertPhysical moves pct percent of the physical damage into an element, the last step of
// COMBAT_BuildAttackerDamage (0x5794e0, VERIFIED; the damage struct bytes +0x65 class and +0x68 percent).
// Skills fill the pair from their EType column and calc4 (Berserk: magic, calc4 = 100; Concentrate and Frenzy:
// magic, calc4 = the Berserk level, read in wave 23 from SRVST_Bash 0x5d6910 / SRVST_039 0x5d82a0 /
// SKILL_FrenzyBuildHitAndFinalize 0x5d75b0). Class 10 picks one of the classes 1..5 at random; classes 1 fire,
// 2 lightning, 3 magic, 4 cold (length at least 50), 5 poison (an eighth of the amount, length at least 50),
// 11 fire with burn length 50, 12 cold with freeze length 50. Other classes only remove the physical part.
func ConvertPhysical(d *Damage, r Roller, class byte, pct int32) {
	if class == 0 {
		return
	}

	moved := MulDiv(d.Physical, pct, 100)
	d.Physical -= moved

	if d.Physical < 0 {
		d.Physical = 0
	}

	if class == ConvertRandom {
		class = byte(r.Roll(5)) + 1
	}

	switch class {
	case 1:
		d.Fire += moved
	case 2:
		d.Lightning += moved
	case 3:
		d.Magic += moved
	case 4:
		d.Cold += moved
		if d.ColdLen < 50 {
			d.ColdLen = 50
		}
	case 5:
		d.Poison += moved / 8
		if d.PoisonLen < 50 {
			d.PoisonLen = 50
		}
	case 11:
		d.Fire += moved
		if d.BurnLen < 50 {
			d.BurnLen = 50
		}
	case 12:
		d.Cold += moved
		if d.FreezeLen < 50 {
			d.FreezeLen = 50
		}
	}
}
