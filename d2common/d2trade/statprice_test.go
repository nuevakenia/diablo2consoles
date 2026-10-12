package d2trade

import "testing"

func testTables() *Tables {
	return &Tables{
		Stats: map[int]StatCost{
			0:   {Mult: 55, Add: 125},
			107: {Encode: 1, Mult: 256, Add: 181},
			195: {Encode: 2, Mult: 256, Add: 190},
			204: {Encode: 3, Mult: 256, Add: 401},
			1:   {ValShift: 8, Mult: 20, Add: 0},
		},
		Skills: map[int]SkillCost{
			7: {Mult: 256, Add: 0, ReqLevel: 12},
			3: {Mult: 256, Add: 181, ReqLevel: 1},
		},
	}
}

func TestStatPropertyDefault(t *testing.T) {
	da, db, dc := StatPropertyDelta(testTables(), []StatEntry{{ID: 0, Value: 5}}, 1000, 125, 1000, 1)
	// 55*1000*5/1024 = 268, +125; 55*125*5/1024 = 33, +125
	if da != 393 || db != 158 || dc != 393 {
		t.Fatalf("got %d %d %d", da, db, dc)
	}
}

func TestStatPropertyBigBranchDividesFirst(t *testing.T) {
	// 20000*10 >= 0x10000: (200000>>10)*55 + 125 = 195*55+125
	da, _, _ := StatPropertyDelta(testTables(), []StatEntry{{ID: 0, Value: 10}}, 20000, 100, 100, 1)
	if want := 195*55 + 125; da != want {
		t.Fatalf("got %d want %d", da, want)
	}
}

func TestStatPropertyValShift(t *testing.T) {
	// stored 5<<8 with ValShift 8 is 5
	da, _, _ := StatPropertyDelta(testTables(), []StatEntry{{ID: 1, Value: 5 << 8}}, 1024, 1024, 1024, 1)
	if want := 20 * 5; da != want {
		t.Fatalf("got %d want %d", da, want)
	}
}

func TestStatPropertySkillEncodeSellQuarter(t *testing.T) {
	// encode 1: the sell side divides by 4096 (VERIFIED quirk of 0x629010)
	da, db, dc := StatPropertyDelta(testTables(), []StatEntry{{ID: 107, Param: 3, Value: 1}}, 1000, 1000, 1000, 1)
	if da != 250+181 || db != 62+181 || dc != 250+181 {
		t.Fatalf("got %d %d %d", da, db, dc)
	}
}

func TestStatPropertyChanceToCastUsesLevel(t *testing.T) {
	// param = skill 3 << 6 | level 4; value is ignored apart from being non-zero
	e := StatEntry{ID: 195, Param: 3<<ParamShift | 4, Value: 1}
	da, _, _ := StatPropertyDelta(testTables(), []StatEntry{e}, 1000, 1000, 1000, 1)
	if want := 256*1000*4/1024 + 181; da != want {
		t.Fatalf("got %d want %d", da, want)
	}
}

func TestStatPropertyDividesByStack(t *testing.T) {
	da, _, _ := StatPropertyDelta(testTables(), []StatEntry{{ID: 0, Value: 5}}, 1000, 125, 1000, 4)
	if da != 393/4 {
		t.Fatalf("got %d", da)
	}
}

func TestStatPropertyUnknownRowsSkipped(t *testing.T) {
	da, db, dc := StatPropertyDelta(testTables(), []StatEntry{{ID: 999, Value: 5}, {ID: 107, Param: 99, Value: 1}}, 100, 100, 100, 1)
	if da != 0 || db != 0 || dc != 0 {
		t.Fatalf("got %d %d %d", da, db, dc)
	}
}

func TestSingleSkillDelta(t *testing.T) {
	// level 3 -> weight 5
	da, db, dc := SingleSkillDelta(testTables(), []StatEntry{{Param: 3, Value: 3}}, 1000, 1000, 1000, 1)
	if da != (250+181)*5 || db != (62+181)*5 || dc != (250+181)*5 {
		t.Fatalf("got %d %d %d", da, db, dc)
	}
}

func TestSingleSkillBigBranchDividesMultiplier(t *testing.T) {
	// A >= 0x10000: (mult>>10)*A + add, mult 256 -> 0
	da, _, _ := SingleSkillDelta(testTables(), []StatEntry{{Param: 3, Value: 1}}, 70000, 100, 100, 1)
	if da != 181 {
		t.Fatalf("got %d", da)
	}
}

func TestSocketedDelta(t *testing.T) {
	if got := SocketedDelta([]int{1001, 50}); got != 500+25 {
		t.Fatalf("got %d", got)
	}
}

func TestRechargeCost(t *testing.T) {
	// skill 7 level 5 charged 2 of 10: lvl 5+2+(12/6)*2 = 11,
	// 110000 >= 0x10000 so the big branch: per = (110000>>10)*256 = 27392, (10-2)*27392/10 = 21913
	e := StatEntry{ID: StatChargedSkill, Param: 7<<ParamShift | 5, Value: 2 | 10<<8}
	if got := RechargeCost(testTables(), []StatEntry{e}); got != 21913 {
		t.Fatalf("got %d", got)
	}

	full := StatEntry{ID: StatChargedSkill, Param: 7<<ParamShift | 5, Value: 10 | 10<<8}
	if got := RechargeCost(testTables(), []StatEntry{full}); got != 0 {
		t.Fatalf("full item got %d", got)
	}
}

// ItemPrice with Tables must equal ItemPrice with the same amounts given as
// fixed StatPrice / SocketPrice.
func TestItemPriceTablesMatchFixedAmounts(t *testing.T) {
	npc := NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024, MaxBuy: [3]int{1 << 30, 1 << 30, 1 << 30}}
	p := Params{Mode: ModeBuy, NPC: npc}
	base := Item{BaseCost: 1000, Quantity: 1, Identified: true, Quality: QualityRare}

	withTables := base
	withTables.Tables = testTables()
	withTables.Stats = []StatEntry{{ID: 0, Value: 5}}
	withTables.SocketedCosts = []int{200}

	fixed := base
	fixed.StatPrice = [3]int{393, 393, 393}
	fixed.SocketPrice = [3]int{100, 100, 100}

	if x, y := ItemPrice(&withTables, p), ItemPrice(&fixed, p); x != y || x <= 1000 {
		t.Fatalf("tables %d fixed %d", x, y)
	}
}

func TestItemPriceStaffModsGate(t *testing.T) {
	npc := NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024}
	p := Params{Mode: ModeBuy, NPC: npc}
	it := Item{BaseCost: 1000, Quantity: 1, Identified: true, Quality: QualityMagic, Tables: testTables(),
		SingleSkills: []StatEntry{{Param: 3, Value: 3}}}

	without := ItemPrice(&it, p)
	it.StaffMods = true

	if with := ItemPrice(&it, p); with != without+(250+181)*5 {
		t.Fatalf("with %d without %d", with, without)
	}
}
