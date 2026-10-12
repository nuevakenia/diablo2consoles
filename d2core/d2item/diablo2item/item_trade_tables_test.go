package diablo2item

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2trade"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func priceFactory() *ItemFactory {
	f := testDropFactory()
	rec := f.asset.Records
	rec.Item.All["cap"].Cost = 1000
	rec.Item.Stats = d2records.ItemStatCosts{
		"str": {Index: 0, CostMultiply: 55, CostAdd: 125},
		"hp":  {Index: 7, ValShift: 8, CostMultiply: 20, CostAdd: 0},
	}
	rec.Skill.Details = d2records.SkillDetails{3: {CostMult: 256, CostAdd: 181, Reqlevel: 6}}
	rec.Item.Books = d2records.Books{"Tome": {BookSpellCode: "tbk", CostPerCharge: 80}}
	rec.Item.All["tbk"] = &d2records.ItemCommonRecord{Code: "tbk", Type: "book", Cost: 450}

	return f
}

// Rolled stat writes feed the price code: base stat writes ('S') and set
// bonus tiers are left out, equal stats are merged.
func TestPriceStatsFromRolledWrites(t *testing.T) {
	f := priceFactory()
	it := &Item{factory: f, CommonCode: "cap", quality: d2drop.QualityMagic, rolled: &d2drop.Rolled{Writes: []d2drop.StatWrite{
		{Kind: 'S', Stat: 31, Value: 40},
		{Kind: 'L', Stat: 0, Value: 3},
		{Kind: 'L', Stat: 0, Value: 2},
		{Kind: 'L', Stat: 7, Value: 10 << 8},
		{Kind: 'L', Stat: 0, Value: 9, List: setListFirst},
	}}}

	got := it.priceStats()
	if len(got) != 2 || got[0] != (d2trade.StatEntry{ID: 0, Value: 5}) || got[1] != (d2trade.StatEntry{ID: 7, Value: 10 << 8}) {
		t.Fatalf("got %+v", got)
	}
}

// The identified magic item costs more than the base: stat prices come from
// the ItemStatCost columns.
func TestTradeItemIncludesStatPrices(t *testing.T) {
	f := priceFactory()
	it := &Item{factory: f, CommonCode: "cap", quality: d2drop.QualityMagic,
		attributes: &itemAttributes{identitified: true},
		rolled:     &d2drop.Rolled{Writes: []d2drop.StatWrite{{Kind: 'L', Stat: 0, Value: 5}}}}

	ti := it.TradeItem()
	if len(ti.Stats) != 1 || ti.Tables == nil {
		t.Fatalf("stats not attached: %+v", ti)
	}

	p := d2trade.Params{Mode: d2trade.ModeBuy, NPC: d2trade.NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024}}
	if got := d2trade.ItemPrice(ti, p); got != 1000+393 {
		t.Fatalf("price %d", got)
	}

	it.attributes.identitified = false

	if got := d2trade.ItemPrice(it.TradeItem(), p); got != 1000 {
		t.Fatalf("unidentified price %d", got)
	}
}

// A tome costs its base plus the per charge price of its scroll times the
// quantity (at least one).
func TestTradeItemTomePrice(t *testing.T) {
	f := priceFactory()
	it := &Item{factory: f, CommonCode: "tbk", quality: d2drop.QualityNormal, attributes: &itemAttributes{currentStackSize: 3}}

	ti := it.TradeItem()
	if !ti.IsTome || ti.TomeScrollCost != 80 {
		t.Fatalf("tome not detected: %+v", ti)
	}

	p := d2trade.Params{Mode: d2trade.ModeBuy, NPC: d2trade.NPC{PlayerPays: 1024, VendorPays: 1024, Repair: 1024}}
	if got := d2trade.ItemPrice(ti, p); got != 450+3*80 {
		t.Fatalf("price %d", got)
	}
}
