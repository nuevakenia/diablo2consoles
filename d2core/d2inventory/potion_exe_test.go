package d2inventory

import (
	"math"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestExeClassOrder(t *testing.T) {
	want := map[d2enum.Hero]int{
		d2enum.HeroAmazon: 0, d2enum.HeroSorceress: 1, d2enum.HeroNecromancer: 2, d2enum.HeroPaladin: 3,
		d2enum.HeroBarbarian: 4, d2enum.HeroDruid: 5, d2enum.HeroAssassin: 6,
	}
	for h, c := range want {
		if got := ExeClass(h); got != c {
			t.Errorf("class of %v = %d, want %d", h, got, c)
		}
	}
}

// golden: PLAYER_ScaleValueByClassGroupA/B 0x62a730 / 0x62a780 on an amount of 320
func TestPotionClassScale(t *testing.T) {
	type row struct{ life, mana int }

	want := map[int]row{
		0: {480, 480}, 3: {480, 480}, 6: {480, 480}, // x1.5
		1: {320, 640}, 2: {320, 640}, 5: {320, 640}, // mana x2
		4: {640, 320}, // barbarian life x2
	}
	for class, w := range want {
		if got := PotionClassScale(PotionGroupLife, class, true, 320); got != w.life {
			t.Errorf("class %d life %d, want %d", class, got, w.life)
		}

		if got := PotionClassScale(PotionGroupMana, class, true, 320); got != w.mana {
			t.Errorf("class %d mana %d, want %d", class, got, w.mana)
		}
	}

	if PotionClassScale(PotionGroupLife, 0, false, 320) != 640 || PotionClassScale(PotionGroupMana, 0, false, 320) != 320 {
		t.Error("non-player drinker: life x2, mana x1")
	}

	// the 1.5 factor is v + (v >> 1): an odd amount rounds down
	if got := PotionClassScale(PotionGroupLife, 0, true, 45); got != 67 {
		t.Errorf("45 x1.5 = %d, want 67", got)
	}
}

func TestPotionDoubles(t *testing.T) {
	draws := []int{}
	script := func(vals ...int) func(int) int {
		i := 0

		return func(n int) int {
			draws = append(draws, n)
			v := vals[i%len(vals)]
			i++

			return v
		}
	}

	if PotionDoubles(script(0, 0), 0) || len(draws) != 0 {
		t.Error("no stat: no draw")
	}

	// stat 100: r = 80 -> threshold 40; d = 39 doubles, 40 does not
	if !PotionDoubles(script(80, 39), 100) || draws[0] != 100 || draws[1] != 100 {
		t.Errorf("draws %v", draws)
	}

	if PotionDoubles(script(80, 40), 100) {
		t.Error("d == r/2 does not double")
	}
}

func TestScaledPotion(t *testing.T) {
	never := func(n int) int { return 0 }
	e := ScaledPotion(PotionEffect{HP: 320, Seconds: 10}, 4, true, 100, 50, never)

	if e.HP != 640 || e.Seconds != 10 {
		t.Errorf("barbarian greater healing %+v", e)
	}

	always := func(n int) int {
		if n == 100 {
			return 0
		}

		return n - 1
	}
	e = ScaledPotion(PotionEffect{Mana: 250, Seconds: 5}, 1, true, 100, 60, always)

	if e.Mana != 1000 { // sorceress x2, then doubled by the roll (d 0 < 59/2)
		t.Errorf("sorceress greater mana %+v", e)
	}

	rejuv := ScaledPotion(PotionEffect{InstantHPPercent: 35, InstantManaPercent: 35}, 4, true, 100, 100, always)
	if rejuv.InstantHPPercent != 35 || rejuv.InstantManaPercent != 35 {
		t.Errorf("rejuvenation is a percent of the maximum and stays as is: %+v", rejuv)
	}
}

// ITEM_ServerUsePotionVariantA 0x5bbf70: a second potion adds the points left to
// the new ones and spreads them over remaining + new length.
func TestRegenMergeSumsDuration(t *testing.T) {
	var r Regen

	r.Add(PotionEffect{HP: 100, Seconds: 10})
	r.Tick(4) // 60 points and 6 s left

	r.Add(PotionEffect{HP: 100, Seconds: 10}) // 160 points over 16 s: 10 per second

	hp, _ := r.Tick(1)
	if math.Abs(hp-10) > 1e-9 {
		t.Errorf("merged rate %v, want 10", hp)
	}

	total := hp

	for i := 0; i < 40 && r.Active(); i++ {
		h, _ := r.Tick(1)
		total += h
	}

	if math.Abs(total-160) > 1e-9 {
		t.Errorf("merged total %v, want 160", total)
	}

	// life and mana run separate clocks
	var m Regen

	m.Add(PotionEffect{HP: 30, Seconds: 3})
	m.Add(PotionEffect{Mana: 20, Seconds: 2})

	if hp, mana := m.Tick(2); math.Abs(hp-20) > 1e-9 || math.Abs(mana-20) > 1e-9 {
		t.Errorf("separate pools: %v %v", hp, mana)
	}
}
