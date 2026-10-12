package d2statlist

import "testing"

// weaponParts is the percent free weapon of the skill descriptor (MISSILE_BuildDamageDescriptor 0x64ca60):
// base or two handed range, the weapon's own enhanced damage, ethereal, the added damage stats, the thrown range
// and the attribute bonus; the barbarian's 1or2handed rule follows INV_ClassifyHandWeaponMode (0x63e3e0).
func TestWeaponParts(t *testing.T) {
	prop := func(id int, v int64) Prop { return Prop{ID: id, Value: v} }
	bow := WeaponBase{TwoHanded: true, TwoMin: 10, TwoMax: 20, DexBonus: 100}
	jav := WeaponBase{Min: 1, Max: 5, MisMin: 6, MisMax: 14, StrBonus: 75, DexBonus: 75}
	bsw := WeaponBase{Min: 7, Max: 19, TwoMin: 20, TwoMax: 28, TwoHanded: true, OneOrTwo: true, StrBonus: 100}

	cases := []struct {
		name     string
		right    Item
		left     bool
		class    int
		added    [2]int64 // stats 21 / 22
		str, dex int
		want     WeaponParts
	}{
		{"bow range", Item{Weapon: &bow}, false, 0, [2]int64{}, 0, 50,
			WeaponParts{Armed: true, Min: 10, Max: 20, BonusPct: 50}},
		{"bow enhanced damage and added damage", Item{Weapon: &bow, Props: []Prop{prop(StatMinDmgPct, 100), prop(StatMaxDmgPct, 50)}},
			false, 0, [2]int64{3, 6}, 0, 0, WeaponParts{Armed: true, Min: 23, Max: 36}},
		{"ethereal bow", Item{Weapon: &bow, Ethereal: true}, false, 0, [2]int64{}, 0, 0,
			WeaponParts{Armed: true, Min: 15, Max: 30}},
		{"javelin thrown range with ED", Item{Weapon: &jav, Props: []Prop{prop(StatMinDmgPct, 100), prop(StatMaxDmgPct, 100)}},
			false, 0, [2]int64{2, 2}, 40, 20,
			WeaponParts{Armed: true, Min: 4, Max: 12, ThrowMin: 14, ThrowMax: 30, Throwing: true, BonusPct: 45}},
		{"bastard sword is two handed for a sorceress", Item{Weapon: &bsw}, false, 1, [2]int64{}, 0, 0,
			WeaponParts{Armed: true, Min: 20, Max: 28}},
		{"barbarian with an empty off hand uses the two handed range", Item{Weapon: &bsw}, false, 4, [2]int64{}, 0, 0,
			WeaponParts{Armed: true, Min: 20, Max: 28}},
		{"barbarian with a shield uses the one handed range", Item{Weapon: &bsw}, true, 4, [2]int64{}, 0, 0,
			WeaponParts{Armed: true, Min: 7, Max: 19}},
	}

	for _, tc := range cases {
		it := tc.right
		it.Slot = SlotRightHand
		active := []Item{it}

		if tc.left {
			active = append(active, Item{Slot: SlotLeftHand, BaseBlock: 30})
		}

		list := &List{}
		list.Add(StatMinDamage, 0, tc.added[0])
		list.Add(StatMaxDamage, 0, tc.added[1])

		got := weaponParts(active, list, tc.class, tc.str, tc.dex)
		if got != tc.want {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// Without a weapon the parts are the bare hand range plus the added damage.
func TestWeaponPartsUnarmed(t *testing.T) {
	list := &List{}
	list.Add(StatMaxDamage, 0, 4)

	got := weaponParts(nil, list, 0, 30, 10)
	if got.Armed || got.Min != 1 || got.Max != 6 || got.BonusPct != 0 {
		t.Errorf("unarmed parts %+v", got)
	}
}

// weaponDamageClass still finishes the range as before: the parts times (100 + attribute bonus + stat 25).
func TestWeaponDamageClassMatchesParts(t *testing.T) {
	bow := WeaponBase{TwoHanded: true, TwoMin: 10, TwoMax: 20, DexBonus: 100}
	list := &List{}
	list.Add(StatDamagePct, 0, 30)

	items := []Item{{Slot: SlotRightHand, Weapon: &bow, Props: []Prop{{ID: StatMaxDmgPct, Value: 100}}}}

	min, max := weaponDamageClass(items, list, 0, 0, 20)
	if min != 10*150/100 || max != 40*150/100 {
		t.Errorf("range %d-%d, want 15-60", min, max)
	}
}
