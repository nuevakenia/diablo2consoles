package d2hero

import (
	"fmt"
	"math"
	"sort"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2equip"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2statlist"
)

// This file compares an item with what the hero wears where it would go, from real numbers: the stat list is
// computed for the body as it is and for the body with the item in place (what it replaces and what it pushes out
// taken off), and the figures that matter (defense with ethereal and enhanced defense, the damage of the weapon in
// the hand with the hero's strength and dexterity bonus, two handed against one handed, the thrown range of a
// throwing weapon, weapon speed, block) are read off the totals. Nothing is parsed from description text. The
// tooltips of the game format the rows (item_compare.go in d2player). The original game shows no comparison at
// all, so every row here is a feature of this port (UNVERIFIED against the exe); the numbers themselves come from
// the verified stat list.

// Boost is added to the hero's raw attributes for a comparison, so an item the hero cannot use yet is compared by
// what it would give once the requirements are met.
type Boost struct{ Str, Dex, Level int }

// Evaluator computes the totals of a body (the worn items as stored items with file locations).
type Evaluator func(body []StoredItem, b Boost) (d2statlist.Totals, []EquipStatus)

// BodyFigures are the numbers of a body that a comparison reads.
type BodyFigures struct {
	Defense     int
	Str, Dex    int     // strength and dexterity with the item bonuses
	Block       int     // block chance in percent
	WeaponSpeed int     // weapons.txt speed of the weapon in the right hand (higher is slower)
	Damage      float64 // average hit of the weapon in the right hand (the thrown range for a throwing weapon)
	Armed       bool
	Throwing    bool
}

// FiguresOf reads the comparison figures off a stat list result.
func FiguresOf(t *d2statlist.Totals) BodyFigures {
	f := BodyFigures{Defense: t.Defense, Str: t.Str, Dex: t.Dex, Block: t.BlockPct, WeaponSpeed: t.WeaponSpeed, Armed: t.Weapon.Armed}
	f.Damage = float64(t.DamageMin+t.DamageMax) / 2

	if w := t.Weapon; w.Armed && w.Throwing && w.ThrowMax > 0 {
		// the same percent the melee range gets (UNVERIFIED for the thrown range, see WeaponParts)
		bonus := float64(100 + w.BonusPct + t.GearDamagePct)
		if bonus < 10 {
			bonus = 10
		}

		f.Damage = float64(w.ThrowMin+w.ThrowMax) / 2 * bonus / 100
		f.Throwing = true
	}

	return f
}

// Verdict says whether a row is better, worse or the same as the worn item.
type Verdict int

// The verdicts.
const (
	Same Verdict = iota
	Better
	Worse
)

// CompareRow is one line of a comparison (without colour).
type CompareRow struct {
	Name    string
	Text    string // the figures after the name, e.g. "+12 (+8%)"
	Verdict Verdict
}

// CompareInput describes what to compare.
type CompareInput struct {
	Eval Evaluator
	// Worn is everything on the body by logical location.
	Worn map[d2equip.Loc]StoredItem
	Item StoredItem
	Loc  d2equip.Loc
	// Displaced are the locations the item pushes out (a two handed weapon pushes out the shield).
	Displaced []d2equip.Loc
	// FileLoc maps a logical location to the location stored in the file (weapon sets).
	FileLoc func(d2equip.Loc) int
	// Weapons says that the item and the worn item at Loc are both weapons: then the damage is compared too.
	Weapons bool
	// NeedStr, NeedDex and NeedLevel are the item's requirements and HeroLevel the hero's level: what the body
	// without the item lacks of them is added to the attributes of every body compared, so the figures say what the
	// item gives once the requirements are met (the replaced item's own bonuses count: taking it off may leave the
	// hero short).
	NeedStr, NeedDex, NeedLevel, HeroLevel int
	// Sockets and maximum durability of the new and of the worn item (0: none).
	NewSockets, OldSockets, NewMaxDur, OldMaxDur int
}

// Comparison is the result.
type Comparison struct {
	Rows []CompareRow
	// Active is false when the item would not be switched on in the body (a class restriction, a broken item):
	// there are no figure rows then.
	Active                    bool
	Before, After, Bare       BodyFigures
	WeaponBefore, WeaponAfter BodyFigures
	DefenseDelta, DamageDelta float64
	Unequips                  []d2equip.Loc
}

// Primary is the figure that ranks the items for the slot choice (the damage of weapons, else the defense).
func (c *Comparison) Primary() float64 {
	if c.WeaponAfter.Armed || c.WeaponBefore.Armed {
		return c.DamageDelta
	}

	return c.DefenseDelta
}

// PickLowest picks the comparison to show when an item could go to several places (two rings, either hand): the one
// with the largest gain, which is the place of the weakest worn item. It returns -1 for an empty list.
func PickLowest(all []Comparison) int {
	best := -1

	for i := range all {
		if !all[i].Active {
			continue
		}

		if best < 0 || all[i].Primary() > all[best].Primary() {
			best = i
		}
	}

	if best < 0 && len(all) > 0 {
		return 0
	}

	return best
}

// Compare computes the rows.
func Compare(in CompareInput) Comparison {
	skip := map[d2equip.Loc]bool{in.Loc: true}
	for _, l := range in.Displaced {
		skip[l] = true
	}

	body := func(drop map[d2equip.Loc]bool, add *StoredItem, at d2equip.Loc) []StoredItem {
		var out []StoredItem

		for l, s := range in.Worn {
			if !drop[l] {
				s.X = in.FileLoc(l)
				out = append(out, s)
			}
		}

		if add != nil {
			s := *add
			s.X = in.FileLoc(at)
			out = append(out, s)
		}

		sort.Slice(out, func(i, j int) bool { return out[i].X < out[j].X })

		return out
	}

	var boost Boost

	figures := func(b []StoredItem) (BodyFigures, []EquipStatus) {
		t, st := in.Eval(b, boost)

		return FiguresOf(&t), st
	}

	var c Comparison

	c.Bare, _ = figures(body(skip, nil, 0))
	boost = Boost{
		Str: maxI(0, in.NeedStr-c.Bare.Str), Dex: maxI(0, in.NeedDex-c.Bare.Dex), Level: maxI(0, in.NeedLevel-in.HeroLevel),
	}

	c.Bare, _ = figures(body(skip, nil, 0))
	c.Before, _ = figures(body(nil, nil, 0))

	var status []EquipStatus

	c.After, status = figures(body(skip, &in.Item, in.Loc))
	c.Active = true

	for _, s := range status {
		if s.Loc == in.Loc && !s.Active {
			c.Active = false
		}
	}

	c.Unequips = append(c.Unequips, in.Displaced...)

	if !c.Active {
		return c
	}

	c.DefenseDelta = float64(c.After.Defense - c.Before.Defense)

	if in.Weapons {
		if in.Loc == d2equip.LocRightHand {
			// the weapon hand: the totals of the two real bodies hold the weapon (the shield in the other hand
			// decides whether a barbarian wields a one-or-two-handed weapon in two hands)
			c.WeaponBefore, c.WeaponAfter = c.Before, c.After
		} else {
			// a weapon in the off hand (dual wield) is not in the totals: both weapons are put alone in the right
			// hand of the otherwise unchanged body, so the attribute bonus, the two handed range and the thrown
			// range compare like with like
			hands := map[d2equip.Loc]bool{d2equip.LocRightHand: true, d2equip.LocLeftHand: true}
			only := func(s StoredItem) BodyFigures {
				f, _ := figures(body(hands, &s, d2equip.LocRightHand))

				return f
			}

			c.WeaponBefore, c.WeaponAfter = only(in.Worn[in.Loc]), only(in.Item)
		}

		c.DamageDelta = c.WeaponAfter.Damage - c.WeaponBefore.Damage
	}

	c.Rows = rows(&c, &in)

	return c
}

func rows(c *Comparison, in *CompareInput) []CompareRow {
	var out []CompareRow

	if in.Weapons && c.WeaponBefore.Armed && c.WeaponAfter.Armed {
		out = append(out, numberRow("Damage", c.WeaponAfter.Damage, c.WeaponBefore.Damage, true))

		if d := c.WeaponAfter.WeaponSpeed - c.WeaponBefore.WeaponSpeed; d != 0 {
			r := CompareRow{Name: "Attack Speed", Verdict: Worse, Text: fmt.Sprintf("%d slower", d)}
			if d < 0 {
				r.Verdict, r.Text = Better, fmt.Sprintf("%d faster", -d)
			}

			out = append(out, r)
		}
	}

	// the defense counts when the item (or what it replaces) has any
	if c.After.Defense != c.Bare.Defense || c.Before.Defense != c.Bare.Defense {
		out = append(out, numberRow("Defense", float64(c.After.Defense), float64(c.Before.Defense), false))
	}

	if c.After.Block != c.Before.Block {
		out = append(out, numberRow("Chance to Block", float64(c.After.Block), float64(c.Before.Block), false))
	}

	if in.NewSockets != in.OldSockets && (in.NewSockets > 0 || in.OldSockets > 0) {
		out = append(out, numberRow("Sockets", float64(in.NewSockets), float64(in.OldSockets), false))
	}

	if in.NewMaxDur != in.OldMaxDur && in.NewMaxDur > 0 && in.OldMaxDur > 0 {
		out = append(out, numberRow("Max Durability", float64(in.NewMaxDur), float64(in.OldMaxDur), false))
	}

	return out
}

// numberRow compares a figure where higher is better; decimals shows one decimal.
func numberRow(name string, newV, oldV float64, decimals bool) CompareRow {
	r := CompareRow{Name: name}
	d := newV - oldV

	if math.Abs(d) < 0.05 {
		r.Text = "same"

		return r
	}

	if d > 0 {
		r.Verdict = Better
	} else {
		r.Verdict = Worse
	}

	format := "%+.0f"
	if decimals {
		format = "%+.1f"
	}

	r.Text = fmt.Sprintf(format, d)
	if oldV > 0 {
		r.Text += fmt.Sprintf(" (%+.0f%%)", d*100/oldV)
	}

	return r
}

func maxI(a, b int) int {
	if a > b {
		return a
	}

	return b
}
