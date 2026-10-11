package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// scanDirector builds a Director whose monstats know the classes the scans match.
func scanDirector(us ...*unit) *Director {
	rec := &d2records.RecordManager{}
	rec.Monster.Stats = d2records.MonStats{
		"knight":  {ID: knightScanClass, BaseKey: "Knight"},
		"over":    {ID: overseerScanClass, BaseKey: "Ally"},
		"zealotA": {ID: priestAllyA, BaseKey: "Zealot"},
		"zealotB": {ID: priestAllyB, BaseKey: "Zealot2"},
		"hide":    {ID: 0x222, BaseKey: "Hidden"},
	}

	d := testDirector(us...)
	d.asset = &d2asset.AssetManager{Records: rec}

	return d
}

func scanUnit(id uint32, x, y int, base string, hp int) *unit {
	u := petUnit(id, x, y, "Melee", "")
	u.m.Stat = &d2records.MonStatRecord{BaseKey: base}
	u.m.Position.Set(float64(x), float64(y))
	u.m.Vitals.HP, u.m.Vitals.MaxHP = hp, 100
	u.b.HPPercent = hp
	u.b.Mode = d2monster.ModeNeutral

	return u
}

func TestScanKnightAllyNearestWithinRadius(t *testing.T) {
	self := scanUnit(1, 0, 0, "Knight", 100)
	d := scanDirector(self, scanUnit(2, 30, 0, "Knight", 100), scanUnit(3, 10, 0, "Knight", 100),
		scanUnit(4, 5, 0, "Other", 100), scanUnit(5, 60, 0, "Knight", 100))

	r := d.FBXScan(self.b, d2monster.FBXScanQuery{Kind: d2monster.FBXScanKnightAlly})
	if !r.Found || r.T.ID != 3 {
		t.Fatalf("nearest knight in 50: %+v", r)
	}

	d.units[3].b.Mode = d2monster.ModeDead

	if r = d.FBXScan(self.b, d2monster.FBXScanQuery{Kind: d2monster.FBXScanKnightAlly}); !r.Found || r.T.ID != 2 {
		t.Fatalf("dead knights are skipped: %+v", r)
	}
}

func TestScanOverseerSlots(t *testing.T) {
	self := scanUnit(1, 0, 0, "Overseer", 100)
	d := scanDirector(self, scanUnit(2, 10, 0, "Ally", 100), scanUnit(3, 15, 0, "Ally", 30),
		scanUnit(4, 40, 0, "Ally", 10), scanUnit(5, 3, 0, "Other", 10))

	q := d2monster.FBXScanQuery{Kind: d2monster.FBXScanOverseer, LifeBelow: 50, AllyRadius2: 576, EnemyRadius2: 400}
	r := d.FBXScan(self.b, q)

	if r.Count != 3 || !r.Found || r.T.ID != 3 || !r.Found2 || r.T2.ID != 2 {
		t.Fatalf("overseer scan: %+v", r)
	}

	// Nihlathak: ally radius 0, enemy radius 625
	q = d2monster.FBXScanQuery{Kind: d2monster.FBXScanOverseer, LifeBelow: 50, AllyRadius2: 0, EnemyRadius2: 625}

	if r = d.FBXScan(self.b, q); r.Found || !r.Found2 || r.T2.ID != 2 {
		t.Fatalf("nihlathak scan: %+v", r)
	}
}

func TestScanPriestAllyLastQualifyingWins(t *testing.T) {
	self := scanUnit(1, 0, 0, "Priest", 100)
	d := scanDirector(self, scanUnit(2, 3, 0, "Zealot", 20), scanUnit(3, 4, 0, "Zealot2", 55),
		scanUnit(4, 5, 0, "Zealot", 90), scanUnit(5, 90, 0, "Zealot", 10))

	r := d.FBXScan(self.b, d2monster.FBXScanQuery{Kind: d2monster.FBXScanPriestAlly, Radius2: 400})
	if !r.Found || r.Count != 2 || r.T.ID != 3 {
		t.Fatalf("priest ally scan: %+v (the last qualifying unit wins)", r)
	}
}

func TestScanDefilerFirstValidTargetInTwentyFive(t *testing.T) {
	self := scanUnit(1, 0, 0, "Defiler", 100)
	hidden := scanUnit(2, 3, 0, "Hidden", 100)
	d := scanDirector(self, hidden, scanUnit(3, 20, 0, "Fallen", 100), scanUnit(4, 5, 0, "Fallen", 100),
		scanUnit(5, 40, 0, "Fallen", 100))

	r := d.FBXScan(self.b, d2monster.FBXScanQuery{Kind: d2monster.FBXScanDefilerCorpse})
	if !r.Found || r.T.ID != 3 {
		t.Fatalf("first valid unit in iteration order, class 0x222 skipped: %+v", r)
	}

	d.SetUnitState(d.units[3].b, 0x6e, true)

	if r = d.FBXScan(self.b, d2monster.FBXScanQuery{Kind: d2monster.FBXScanDefilerCorpse}); !r.Found || r.T.ID != 4 {
		t.Fatalf("state 0x6e units are skipped: %+v", r)
	}
}
