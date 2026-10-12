package d2hero

import (
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
)

// compareFixture is the real level 94 Sorceress of the sample save with the real tables (D2_TABLES,
// D2S_SAMPLE_BODY; skipped without) and an evaluator that runs the very activation pass and stat list the game
// uses for the hero's totals.
func compareFixture(t *testing.T) (f *HeroStateFactory, c *HeroContainers, worn map[d2equip.Loc]StoredItem, eval Evaluator) {
	t.Helper()

	f, c, h, _, _ := equipFixture(t)

	worn = map[d2equip.Loc]StoredItem{}
	for i := range c.Equipped {
		worn[c.LogicalLoc(c.Equipped[i].X)] = c.Equipped[i]
	}

	eval = func(body []StoredItem, b Boost) (d2statlist.Totals, []EquipStatus) {
		cc := *c
		cc.Equipped = body
		hh := h
		hh.Str += b.Str
		hh.Dex += b.Dex
		hh.Level += b.Level

		return totalsOf(f, &cc, hh)
	}

	return f, c, worn, eval
}

func fileLoc(l d2equip.Loc) int { return int(l) }

func stat(code string, def int, props ...d2statlist.Prop) StoredItem {
	return StoredItem{Code: code, Identified: true, Stat: &d2statlist.Item{Code: code, Defense: def, Props: props}}
}

func rowOf(c Comparison, name string) (CompareRow, bool) {
	for _, r := range c.Rows {
		if r.Name == name {
			return r, true
		}
	}

	return CompareRow{}, false
}

func TestCompareArmourFromRealNumbers(t *testing.T) {
	f, c, worn, eval := compareFixture(t)

	torso := worn[d2equip.LocTorso]
	base := f.statBases[torso.Code]

	ed100 := d2statlist.Prop{ID: d2statlist.StatArmorPct, Value: 100}

	tests := []struct {
		name string
		item StoredItem
		want Verdict
		// wantDelta is checked against an independent computation when set
		check func(t *testing.T, got Comparison)
	}{
		{"lower base armour", stat("xtp", 240), Worse, nil},
		{"higher defense", stat(torso.Code, 2000), Better, nil},
		{"enhanced defense doubles the base", stat(torso.Code, 3000, ed100), Better, func(t *testing.T, got Comparison) {
			// base 3000 raised by 100% is 6000; the worn torso's own defense is whatever the stat list says
			if got.After.Defense-got.Bare.Defense != 6000 {
				t.Errorf("item defense %d, want 6000 (3000 + 100%%)", got.After.Defense-got.Bare.Defense)
			}
		}},
		{"same armour", stat(torso.Code, base.MinAC), Worse, nil}, // the worn one carries properties: just compare the delta below
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(CompareInput{Eval: eval, Worn: worn, Item: tc.item, Loc: d2equip.LocTorso, FileLoc: fileLoc})

			if !got.Active {
				t.Fatalf("not active")
			}

			r, ok := rowOf(got, "Defense")
			if !ok {
				t.Fatalf("no defense row: %+v", got.Rows)
			}

			if tc.name != "same armour" && r.Verdict != tc.want {
				t.Errorf("verdict %v, want %v (row %+v)", r.Verdict, tc.want, r)
			}

			// the delta is the totals difference of two real stat list runs
			full, _ := eval(append(withoutLoc(c, d2equip.LocTorso), withX(tc.item, int(d2equip.LocTorso))), Boost{})
			before, _ := eval(c.Equipped, Boost{})

			if want := full.Defense - before.Defense; int(got.DefenseDelta) != want {
				t.Errorf("delta %v, stat list says %d", got.DefenseDelta, want)
			}

			if tc.check != nil {
				tc.check(t, got)
			}
		})
	}

	// the worn item against itself is equal: a white "same" row and no verdict
	got := Compare(CompareInput{Eval: eval, Worn: worn, Item: torso, Loc: d2equip.LocTorso, FileLoc: fileLoc})
	if r, ok := rowOf(got, "Defense"); !ok || r.Verdict != Same || r.Text != "same" {
		t.Errorf("worn against itself: %+v", got.Rows)
	}
}

func withoutLoc(c *HeroContainers, l d2equip.Loc) []StoredItem {
	var out []StoredItem

	for _, s := range c.Equipped {
		if c.LogicalLoc(s.X) != l {
			out = append(out, s)
		}
	}

	return out
}

func withX(s StoredItem, x int) StoredItem {
	s.X = x

	return s
}

func TestCompareWeapons(t *testing.T) {
	f, _, worn, eval := compareFixture(t)

	wornW := worn[d2equip.LocRightHand]
	if f.statBases[wornW.Code].Weapon == nil {
		t.Fatalf("right hand is not a weapon: %s", wornW.Code)
	}

	// every weapon below is compared with the worn right hand weapon; the sign of the damage row must follow the
	// average of the hero's real hit and the speed row the weapons.txt speed column
	for _, code := range []string{"7ws", "lbb", "tkf", "9cl", "7gd", "amb"} {
		w := f.statBases[code].Weapon
		if w == nil {
			t.Logf("%s: no such weapon in the tables", code)

			continue
		}

		t.Run(code, func(t *testing.T) {
			got := Compare(CompareInput{
				Eval: eval, Worn: worn, Item: stat(code, 0), Loc: d2equip.LocRightHand, FileLoc: fileLoc, Weapons: true,
			})

			if !got.Active {
				t.Skipf("%s is not active for this hero", code)
			}

			r, ok := rowOf(got, "Damage")
			if !ok {
				t.Fatalf("no damage row: %+v", got)
			}

			switch {
			case got.DamageDelta > 0.05 && r.Verdict != Better, got.DamageDelta < -0.05 && r.Verdict != Worse:
				t.Errorf("damage delta %v with verdict %v", got.DamageDelta, r.Verdict)
			}

			// a weapon with a thrown range is compared by it
			if got.WeaponAfter.Throwing != (w.MisMax > 0) {
				t.Errorf("throwing %v, the table's missile damage is %d-%d", got.WeaponAfter.Throwing, w.MisMin, w.MisMax)
			}

			if w.MisMax > 0 && got.WeaponAfter.Damage <= 0 {
				t.Errorf("a throwing weapon must be compared by its thrown range, got %v", got.WeaponAfter.Damage)
			}

			// attack speed: a lower weapons.txt speed is faster, the row says so
			d := f.statBases[code].Speed - f.statBases[wornW.Code].Speed
			sp, has := rowOf(got, "Attack Speed")

			switch {
			case d == 0 && has:
				t.Errorf("equal speed must have no row, got %+v", sp)
			case d < 0 && (!has || sp.Verdict != Better || !strings.HasSuffix(sp.Text, "faster")):
				t.Errorf("speed %d should be a faster row, got %+v", d, sp)
			case d > 0 && (!has || sp.Verdict != Worse || !strings.HasSuffix(sp.Text, "slower")):
				t.Errorf("speed %d should be a slower row, got %+v", d, sp)
			}

			t.Logf("%s: damage %.1f -> %.1f (%+v) speed %d -> %d rows=%v", code, got.WeaponBefore.Damage, got.WeaponAfter.Damage,
				r, got.WeaponBefore.WeaponSpeed, got.WeaponAfter.WeaponSpeed, got.Rows)
		})
	}
}

func TestCompareDamageBonusesAndEthereal(t *testing.T) {
	_, _, worn, eval := compareFixture(t)

	// +100% enhanced damage (stats 17 and 18) doubles the weapon's own range: the same weapon with it is better,
	// one with a negative percent is worse
	ed := func(p int64) StoredItem {
		return stat("fla", 0, d2statlist.Prop{ID: 17, Value: p}, d2statlist.Prop{ID: 18, Value: p})
	}

	plain := Compare(CompareInput{Eval: eval, Worn: worn, Item: stat("fla", 0), Loc: d2equip.LocRightHand, FileLoc: fileLoc, Weapons: true})
	more := Compare(CompareInput{Eval: eval, Worn: worn, Item: ed(100), Loc: d2equip.LocRightHand, FileLoc: fileLoc, Weapons: true})
	less := Compare(CompareInput{Eval: eval, Worn: worn, Item: ed(-50), Loc: d2equip.LocRightHand, FileLoc: fileLoc, Weapons: true})

	if !(more.WeaponAfter.Damage > plain.WeaponAfter.Damage && plain.WeaponAfter.Damage > less.WeaponAfter.Damage) {
		t.Errorf("damage by enhanced damage: %v %v %v", more.WeaponAfter.Damage, plain.WeaponAfter.Damage, less.WeaponAfter.Damage)
	}

	if r, _ := rowOf(more, "Damage"); r.Verdict != Better {
		t.Errorf("+100%% damage: %+v", r)
	}

	if r, _ := rowOf(less, "Damage"); r.Verdict != Worse {
		t.Errorf("-50%% damage: %+v", r)
	}
}

func TestCompareTwoHandedPushesOutTheShield(t *testing.T) {
	f, _, worn, eval := compareFixture(t)

	shield := worn[d2equip.LocLeftHand]
	if shield.Code == "" {
		t.Skip("the sample hero wears no shield")
	}

	// a two handed weapon into the right hand pushes the shield out: the defense row falls by the shield's
	for code, b := range f.equipBases {
		if !b.Weapon || !b.TwoHanded || f.statBases[code].Weapon == nil || f.statBases[code].Weapon.Ranged {
			continue
		}

		got := Compare(CompareInput{
			Eval: eval, Worn: worn, Item: stat(code, 0), Loc: d2equip.LocRightHand, FileLoc: fileLoc, Weapons: true,
			Displaced: []d2equip.Loc{d2equip.LocLeftHand},
		})

		if !got.Active {
			continue
		}

		r, ok := rowOf(got, "Defense")
		if !ok || r.Verdict != Worse {
			t.Errorf("%s: pushing the shield out must lower the defense: %+v", code, got.Rows)
		}

		if len(got.Unequips) != 1 || got.Unequips[0] != d2equip.LocLeftHand {
			t.Errorf("unequips %v", got.Unequips)
		}

		return
	}

	t.Skip("no two handed melee weapon the hero can use")
}

func TestCompareRequirementsBoost(t *testing.T) {
	f, _, worn, eval := compareFixture(t)

	// an armour piece the hero is too weak for: not active as is, active (and compared) with the boost
	for code, b := range f.equipBases {
		if b.Weapon || b.Type != "tors" || b.ReqStr <= 150 {
			continue
		}

		it := stat(code, 900)
		in := CompareInput{Eval: eval, Worn: worn, Item: it, Loc: d2equip.LocTorso, FileLoc: fileLoc}

		if got := Compare(in); got.Active || len(got.Rows) != 0 {
			t.Fatalf("%s needs %d strength: the hero (122) cannot use it, got %+v", code, b.ReqStr, got)
		}

		in.NeedStr, in.HeroLevel = b.ReqStr, 94

		got := Compare(in)
		if !got.Active {
			t.Fatalf("%s with the strength boost must be active", code)
		}

		if _, ok := rowOf(got, "Defense"); !ok {
			t.Errorf("%s: no defense row with the boost", code)
		}

		return
	}

	t.Skip("no heavy body armour in the tables")
}

func TestCompareSocketsDurabilityBlock(t *testing.T) {
	_, _, worn, eval := compareFixture(t)

	got := Compare(CompareInput{
		Eval: eval, Worn: worn, Item: stat("uit", 0), Loc: d2equip.LocLeftHand, FileLoc: fileLoc,
		NewSockets: 3, OldSockets: 1, NewMaxDur: 50, OldMaxDur: 60,
	})

	if r, ok := rowOf(got, "Sockets"); !ok || r.Verdict != Better || r.Text != "+2 (+200%)" {
		t.Errorf("sockets: %+v", r)
	}

	if r, ok := rowOf(got, "Max Durability"); !ok || r.Verdict != Worse || !strings.HasPrefix(r.Text, "-10") {
		t.Errorf("durability: %+v", r)
	}

	// equal sockets and durability: no rows at all
	got = Compare(CompareInput{Eval: eval, Worn: worn, Item: worn[d2equip.LocLeftHand], Loc: d2equip.LocLeftHand, FileLoc: fileLoc,
		NewSockets: 2, OldSockets: 2, NewMaxDur: 60, OldMaxDur: 60})

	for _, n := range []string{"Sockets", "Max Durability"} {
		if _, ok := rowOf(got, n); ok {
			t.Errorf("%s row for equal values", n)
		}
	}
}

func TestCompareDualWieldOffHand(t *testing.T) {
	_, _, worn, eval := compareFixture(t)

	// an assassin or barbarian wears a weapon in each hand: the one in the off hand is not in the totals, so both
	// are compared alone in the main hand
	body := map[d2equip.Loc]StoredItem{}
	for l, s := range worn {
		body[l] = s
	}

	body[d2equip.LocLeftHand] = stat("lbb", 0)

	got := Compare(CompareInput{Eval: eval, Worn: body, Item: stat("fla", 0), Loc: d2equip.LocLeftHand, FileLoc: fileLoc, Weapons: true})
	if !got.Active || !got.WeaponBefore.Armed || !got.WeaponAfter.Armed {
		t.Fatalf("both weapons must be measured: %+v", got)
	}

	if r, ok := rowOf(got, "Damage"); !ok || r.Verdict != Better {
		t.Errorf("a flail against a long battle bow: %+v (before %v after %v)", got.Rows, got.WeaponBefore.Damage, got.WeaponAfter.Damage)
	}
}

func TestComparePickLowerRing(t *testing.T) {
	_, _, worn, eval := compareFixture(t)

	// two rings, one giving 300 defense and one 20: a new ring giving 100 is compared with (and replaces) the lower
	rings := map[d2equip.Loc]StoredItem{d2equip.LocLeftRing: stat("rin", 0, d2statlist.Prop{ID: 31, Value: 300}),
		d2equip.LocRightRing: stat("rin", 0, d2statlist.Prop{ID: 31, Value: 20})}

	body := map[d2equip.Loc]StoredItem{}
	for l, s := range worn {
		body[l] = s
	}

	delete(body, d2equip.LocLeftRing)
	delete(body, d2equip.LocRightRing)

	for l, s := range rings {
		body[l] = s
	}

	cand := stat("rin", 0, d2statlist.Prop{ID: 31, Value: 100})

	var all []Comparison

	for _, l := range []d2equip.Loc{d2equip.LocLeftRing, d2equip.LocRightRing} {
		all = append(all, Compare(CompareInput{Eval: eval, Worn: body, Item: cand, Loc: l, FileLoc: fileLoc}))
	}

	i := PickLowest(all)
	if i != 1 {
		t.Fatalf("picked ring %d, want the 20 defense one (1): deltas %v %v", i, all[0].DefenseDelta, all[1].DefenseDelta)
	}

	if r, _ := rowOf(all[i], "Defense"); r.Verdict != Better {
		t.Errorf("a 100 ring against the 20 ring is better: %+v", r)
	}
}
