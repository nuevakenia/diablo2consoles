package d2skill

import "testing"

func init() {
	rows = append(rows,
		row{"skill": "IronGolem5", "Id": "720", "charclass": "nec", "srvstfunc": "20", "srvdofunc": "57", "summon": "irongolem",
			"pettype": "golem", "petmax": "1", "summode": "S1", "calc1": "10", "manashift": "8"},
		row{"skill": "Clay Golem5", "Id": "721", "charclass": "nec", "srvdofunc": "56", "summon": "claygolem",
			"pettype": "golem", "petmax": "1", "summode": "S1", "calc1": "10", "manashift": "8"},
	)
}

func TestFilterMatchTotemAura(t *testing.T) {
	const totem = 65795 // aurafilter of Oak Sage Aura, Wolverine Aura and Barbs Aura

	tests := []struct {
		name string
		u    FilterFacts
		want bool
	}{
		{"hero", FilterFacts{Kind: KindPlayer, Allied: true}, true},
		{"minion", FilterFacts{Kind: KindMonster, Allied: true}, true},
		{"hostile monster", FilterFacts{Kind: KindMonster}, false},
		{"hostile player", FilterFacts{Kind: KindPlayer}, false},
		{"monster with the 0x100 record bit", FilterFacts{Kind: KindMonster, Allied: true, NoAuraMonster: true}, false},
		{"hero in a town room", FilterFacts{Kind: KindPlayer, Allied: true, InTownRoom: true}, false},
		{"dead minion", FilterFacts{Kind: KindMonster, Allied: true, Dead: true}, false},
		{"dying minion", FilterFacts{Kind: KindMonster, Allied: true, Dying: true}, false},
		{"missile", FilterFacts{Kind: KindMissile, Allied: true}, false},
		{"object", FilterFacts{Kind: KindObject, Allied: true}, false},
	}

	for _, tt := range tests {
		if got := FilterMatch(totem, tt.u); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFilterMatchBits(t *testing.T) {
	tests := []struct {
		name   string
		filter int
		u      FilterFacts
		want   bool
	}{
		{"default filter takes monsters", 0, FilterFacts{Kind: KindMonster, Flag2: true, Flag3: true}, true},
		{"default filter takes players", 0, FilterFacts{Kind: KindPlayer, Flag2: true, Flag3: true}, true},
		{"default filter takes objects: no", 0, FilterFacts{Kind: KindObject}, false},
		{"sanctuary takes undead", 59270, FilterFacts{Kind: KindMonster, Undead: true, Hostile: true, LineClear: true, Flag2: true, Flag3: true}, true},
		{"sanctuary skips the living", 59270, FilterFacts{Kind: KindMonster, Hostile: true, LineClear: true, Flag2: true, Flag3: true}, false},
		{"sanctuary skips undead bosses", 59270, FilterFacts{Kind: KindMonster, Undead: true, Boss: true, Hostile: true, LineClear: true}, false},
		{"dead only takes corpses", FilterMonsters | FilterDeadOnly, FilterFacts{Kind: KindMonster, Dead: true}, true},
		{"dead only refuses the living", FilterMonsters | FilterDeadOnly, FilterFacts{Kind: KindMonster}, false},
		{"alive filter refuses a corpse", FilterMonsters, FilterFacts{Kind: KindMonster, Dead: true}, false},
		{"flag 2 needed", FilterMonsters | FilterNeedFlag2, FilterFacts{Kind: KindMonster}, false},
		{"flag 2 present", FilterMonsters | FilterNeedFlag2, FilterFacts{Kind: KindMonster, Flag2: true}, true},
		{"flag 3 needed", FilterMonsters | FilterNeedFlag3, FilterFacts{Kind: KindMonster}, false},
		{"stat flag 80 excluded", FilterMonsters | FilterNoStatFlag80, FilterFacts{Kind: KindMonster, StatFlag80: true}, false},
		{"state 0x56 excluded", FilterMonsters | FilterNoState56, FilterFacts{Kind: KindMonster, HasState56: true}, false},
		{"melee needs reach", FilterMonsters | FilterMelee, FilterFacts{Kind: KindMonster}, false},
		{"line of sight", FilterMonsters | FilterLineOfSight, FilterFacts{Kind: KindMonster}, false},
		{"missile with bit 2 never passes", FilterMissiles, FilterFacts{Kind: KindMissile, MissileBit2: true}, false},
		{"missile passes", FilterMissiles, FilterFacts{Kind: KindMissile}, true},
		{"item", FilterItems, FilterFacts{Kind: KindItem}, true},
		{"item type bit missing", FilterMonsters, FilterFacts{Kind: KindItem}, false},
	}

	for _, tt := range tests {
		if got := FilterMatch(tt.filter, tt.u); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestSummonLevel(t *testing.T) {
	tests := []struct{ caster, skill, want int }{
		{0, 5, 0},
		{1, 1, 1},    // 0 + 1, capped at the caster
		{10, 1, 8},   // 7 + 1
		{10, 20, 10}, // capped at the caster level
		{30, 5, 27},  // 22 + 5
		{99, 20, 94},
		{4, 0, 3},
		{1, 0, 1}, // never below 1
	}

	for _, tt := range tests {
		if got := SummonLevel(tt.caster, tt.skill); got != tt.want {
			t.Errorf("SummonLevel(%d, %d) = %d, want %d", tt.caster, tt.skill, got, tt.want)
		}
	}
}

func TestCorpseRules(t *testing.T) {
	ok := CorpseFacts{Monster: true, Dead: true, Selectable: true, WalkSpeed: 8}

	tests := []struct {
		name           string
		f              CorpseFacts
		explode, raise bool
		raiseDying     bool
	}{
		{"plain corpse", ok, true, true, true},
		{"not a monster", CorpseFacts{Dead: true, Selectable: true, WalkSpeed: 8}, false, false, false},
		{"alive", CorpseFacts{Monster: true, Selectable: true, WalkSpeed: 8}, false, false, false},
		{"state mask 0x150", CorpseFacts{Monster: true, Dead: true, Selectable: true, WalkSpeed: 8, InStateMask150: true}, false, false, false},
		{"not selectable", CorpseFacts{Monster: true, Dead: true, WalkSpeed: 8}, false, false, false},
		{"stationary leaves no raisable corpse", CorpseFacts{Monster: true, Dead: true, Selectable: true}, true, false, false},
		{"first mode", CorpseFacts{Monster: true, Dying: true, Selectable: true, WalkSpeed: 8}, false, false, true},
	}

	for _, tt := range tests {
		if got := CorpseExplodable(tt.f); got != tt.explode {
			t.Errorf("%s: explodable %v, want %v", tt.name, got, tt.explode)
		}

		if got := CorpseRaisable(tt.f, false); got != tt.raise {
			t.Errorf("%s: raisable %v, want %v", tt.name, got, tt.raise)
		}

		if got := CorpseRaisable(tt.f, true); got != (tt.raise || tt.raiseDying) {
			t.Errorf("%s: raisable with dying %v, want %v", tt.name, got, tt.raise || tt.raiseDying)
		}
	}
}

func TestRaiseSkeletonRefusesBadCorpse(t *testing.T) {
	cf := newClassFixture(map[string]int{"Raise Skeleton": 7})
	id := cf.id("Raise Skeleton")
	base := Target{X: 5, Y: 5, Corpse: true, CX: 5, CY: 5, CorpseID: "c1", CorpseHP: 80}

	still := base
	still.CorpseInfo = &CorpseFacts{Monster: true, Dead: true, Selectable: true} // no walk speed

	if r := cf.p.Do(cf.u, id, still); r.OK || r.Reason != ReasonNoCorpse {
		t.Errorf("stationary corpse: %+v", r)
	}

	good := base
	good.CorpseInfo = &CorpseFacts{Monster: true, Dead: true, Selectable: true, WalkSpeed: 8}

	r := cf.p.Do(cf.u, id, good)
	if !r.OK {
		t.Fatalf("good corpse: %+v", r)
	}

	if e := effectOf(t, r, "summon"); e.Summon.CorpseID != "c1" {
		t.Errorf("order %+v", e.Summon)
	}
}

func TestGolemSummonLevel(t *testing.T) {
	cf := newClassFixture(map[string]int{"Clay Golem5": 4})
	cf.u.level = 20

	_, r := cf.cast("Clay Golem5", 5, 5)

	e := effectOf(t, r, "summon")
	if e.Summon.Level != 19 { // 20*3/4 + 4
		t.Errorf("golem level %d, want 19", e.Summon.Level)
	}
}

func TestIronGolemNeedsAGroundItem(t *testing.T) {
	cf := newClassFixture(map[string]int{"IronGolem5": 3})
	cf.u.level = 20
	id := cf.id("IronGolem5")

	// no item: refused
	none := Target{X: 5, Y: 5}
	if r := cf.p.Do(cf.u, id, none); r.OK || r.Reason != ReasonTarget {
		t.Errorf("without an item: %+v", r)
	}

	// an item the golem cannot use
	bad := Target{X: 5, Y: 5, GroundItem: &GroundItem{ID: "i1", X: 6, Y: 7}}
	if r := cf.p.Do(cf.u, id, bad); r.OK {
		t.Errorf("unusable item accepted: %+v", r)
	}

	// the item at (9, 4): the golem stands there and the item is the order's
	good := Target{X: 5, Y: 5, GroundItem: &GroundItem{ID: "i2", X: 9, Y: 4, Valid: true}}

	r := cf.p.Do(cf.u, id, good)
	if !r.OK {
		t.Fatalf("with an item: %+v", r)
	}

	o := effectOf(t, r, "summon").Summon
	if o.ItemID != "i2" || o.X != 9 || o.Y != 4 || !o.AtPoint || o.Level != 18 { // 15 + 3
		t.Errorf("iron golem order %+v", o)
	}
}

func TestIronGolemStartGate(t *testing.T) {
	cf := newClassFixture(map[string]int{"IronGolem5": 3})
	id := cf.id("IronGolem5")

	tg := Target{X: 5, Y: 5, Item: &ItemTarget{OnGround: true, GolemItem: true, FlagOK: true, InRoom: true},
		GroundItem: &GroundItem{ID: "i", X: 5, Y: 5, Valid: true}}
	if st := cf.p.Start(cf.u, id, tg); !st.OK {
		t.Errorf("start with a ground item: %+v", st)
	}

	tg.Item.Owned = true
	if st := cf.p.Start(cf.u, id, tg); st.OK {
		t.Errorf("start with an owned item: %+v", st)
	}
}
