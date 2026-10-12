package d2trade

import "testing"

func TestCanRepairAt(t *testing.T) {
	for _, id := range []int{154, 178, 253, 257, 511} {
		if !CanRepairAt(id) {
			t.Errorf("%d should repair", id)
		}
	}

	for _, id := range []int{148, 255, 405, 0} {
		if CanRepairAt(id) {
			t.Errorf("%d should not repair", id)
		}
	}
}

func TestSellable(t *testing.T) {
	if !Sellable(SellInfo{}) {
		t.Fatal("plain item must sell")
	}

	for _, s := range []SellInfo{{NoSellFlag: true}, {Quest: true}, {QuestType: true}} {
		if Sellable(s) {
			t.Errorf("%+v must not sell", s)
		}
	}
}

func TestResellable(t *testing.T) {
	if !Resellable(ResaleInfo{}) {
		t.Fatal("plain item joins the stock")
	}

	cases := []ResaleInfo{
		{Cracked: true}, {Broken: true}, {PlayerPart: true}, {Personalized: true}, {Ethereal: true},
		{HasContents: true}, {SingleCopy: true}, {GambleMode: true}, {VendorSells: true},
	}
	for _, c := range cases {
		if Resellable(c) {
			t.Errorf("%+v must not be resold", c)
		}
	}
}

func TestResalePaidIsTheLower(t *testing.T) {
	if ResalePaid(100, 80) != 80 || ResalePaid(100, 120) != 100 {
		t.Fatal("lower price wins")
	}
}

func TestDeductGold(t *testing.T) {
	cases := []struct {
		inv, stash, cost int
		wantInv, wantSt  int
		ok               bool
	}{
		{100, 50, 30, 70, 50, true},
		{100, 50, 100, 0, 50, true},
		{100, 50, 120, 0, 30, true},
		{100, 50, 150, 0, 0, true},
		{100, 50, 151, 100, 50, false},
		{0, 0, 0, 0, 0, true},
	}
	for _, c := range cases {
		inv, st, ok := DeductGold(c.inv, c.stash, c.cost)
		if inv != c.wantInv || st != c.wantSt || ok != c.ok {
			t.Errorf("%+v got %d %d %v", c, inv, st, ok)
		}
	}
}

func TestPartialRepair(t *testing.T) {
	// price 100 for 50 missing points: 2 gold per point; 20 gold -> 10 points
	dur, spent, ok := PartialRepair(100, 10, 60, 20)
	if !ok || dur != 20 || spent != 20 {
		t.Fatalf("got %d %d %v", dur, spent, ok)
	}

	// enough for more than the item needs is capped at max
	if dur, _, ok := PartialRepair(100, 10, 60, 120); !ok || dur != 60 {
		t.Fatalf("cap: %d %v", dur, ok)
	}

	// one point costs 2 gold, 1 gold buys nothing; no gold buys nothing
	if _, _, ok := PartialRepair(100, 10, 60, 1); ok {
		t.Fatal("1 gold must not repair")
	}

	if _, _, ok := PartialRepair(100, 10, 60, 0); ok {
		t.Fatal("0 gold must not repair")
	}

	// a full item has nothing to repair
	if _, _, ok := PartialRepair(100, 60, 60, 500); ok {
		t.Fatal("full item")
	}
}
