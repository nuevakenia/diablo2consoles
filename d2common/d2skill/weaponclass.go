package d2skill

import "strings"

// AttackWeaponClasser is implemented by units that know the weapon class code of the weapon they attack with
// (weapons.txt wclass / the COF weapon class: "hth", "bow", "1hs", "xbw"...). UNIT_GetAttackWeaponClass
// (0x623e60) returns its index; the fan, strafe and guided arrow do functions only compare it with 1.
type AttackWeaponClasser interface {
	AttackWeaponClass() string
}

// WeaponClassBow is the index of the bow class in the exe's weapon class table (VERIFIED: 0x6472a0 treats class 1
// as the bow and 7 as the crossbow; the rest follow the COF order hth, bow, 1hs, 1ht, stf, 2hs, 2ht, xbw...).
const WeaponClassBow = 1

// weaponClassIndex maps a weapon class code to the exe's index (ITEM_GetWeaponClassId 0x62a190 through the code
// table at 0x740f30). Unknown codes give -1.
func weaponClassIndex(code string) int {
	for i, c := range []string{"hth", "bow", "1hs", "1ht", "stf", "2hs", "2ht", "xbw", "1js", "1jt", "1ss", "1st", "ht1", "ht2"} {
		if strings.EqualFold(c, code) {
			return i
		}
	}

	return -1
}

// UsesMissileB is the weapon test of Multiple Shot (SRVDO_008 0x5d9fe0), Guided Arrow (SRVDO_010 0x5da2f0) and Strafe
// (SRVDO_012 0x5da6c0), VERIFIED: srvmissileb replaces srvmissilea when the attack weapon class is not 1 (the bow)
// and the skill has a srvmissileb, so a crossbow, any other weapon and bare hands take it. A unit that does not
// report a weapon class (monsters, fakes) keeps the old rule: its ranged missile name says "bolt".
func UsesMissileB(u Unit, sk *Skill) bool {
	if sk.SrvMissileB == "" {
		return false
	}

	if wc, ok := u.(AttackWeaponClasser); ok {
		if code := wc.AttackWeaponClass(); code != "" {
			return weaponClassIndex(code) != WeaponClassBow
		}
	}

	return strings.Contains(u.RangedWeaponMissile(), "bolt")
}
