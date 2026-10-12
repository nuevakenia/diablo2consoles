package d2npc

import (
	"reflect"
	"testing"
)

func kinds(rows []Option) []Kind {
	var k []Kind
	for _, r := range rows {
		k = append(k, r.Kind)
	}

	return k
}

func quest(bits ...[2]int) QuestFlag {
	set := map[[2]int]bool{}
	for _, b := range bits {
		set[b] = true
	}

	return func(q, b int) bool { return set[[2]int{q, b}] }
}

func TestTableSizes(t *testing.T) {
	if TableSize() != 48 {
		t.Fatalf("menu table has %d rows, the exe has 48 (DAT_00724b8c)", TableSize())
	}

	if EntryCount() != 46 {
		t.Fatalf("state array has %d entries, the exe has 46 (DAT_00724664)", EntryCount())
	}
}

func TestStaticRows(t *testing.T) {
	cases := []struct {
		class int
		want  []Kind
	}{
		{148, []Kind{KindTalk, KindTrade}},             // Akara
		{154, []Kind{KindTalk, KindTradeRepair}},       // Charsi
		{147, []Kind{KindTalk, KindTrade, KindGamble}}, // Gheed
		{252, []Kind{KindTalk, KindHire, KindTrade}},   // Asheara
		{198, []Kind{KindTalk, KindHire}},              // Greiz
		{257, []Kind{KindTradeRepair}},                 // Halbu: the first row wins, no Talk
		{405, []Kind{KindTrade, KindGamble}},           // Jamella
		{514, []Kind{KindTalk, KindGamble}},            // Nihlathak (the server opens gamble for 0x202)
		{150, []Kind{KindTalk}},                        // Kashya: Hire is added at run time
		{515, []Kind{KindTalk}},                        // Qual-Kehk
		{367, []Kind{KindTalk}},                        // Tyrael 2
		{175, []Kind{KindTalk, KindGoWest}},            // Warriv 2
		{264, []Kind{KindTalk, KindSailWest}},          // Meshif 2
		{520, []Kind{KindTalk, KindIdentify}},          // Cain 6
		{511, []Kind{KindTalk, KindTradeRepair}},       // Larzuk
		{512, []Kind{KindTalk, KindTrade, KindGamble}}, // Drehya
		{513, []Kind{KindTalk, KindTrade}},             // Malah
		{255, []Kind{KindTalk, KindTrade}},             // Ormus
		{539, []Kind{KindTalk}},                        // ancient statue
		{521, []Kind{KindTalk}},                        // Tyrael 3
	}

	for _, c := range cases {
		rows, ok := StaticRows(c.class)
		if !ok || !reflect.DeepEqual(kinds(rows), c.want) {
			t.Errorf("class %d: got %v ok=%v, want %v", c.class, kinds(rows), ok, c.want)
		}
	}

	if _, ok := StaticRows(9999); ok {
		t.Error("an unknown class has a row")
	}
}

func TestEveryEntryHasAMenuRow(t *testing.T) {
	// every NPC with a speech pool can be opened (the converse is not true: Halbu's second row has no pool)
	for _, e := range gossipEntries {
		if _, ok := StaticRows(e.Class); !ok {
			t.Errorf("class %d has a speech pool but no menu row", e.Class)
		}
	}
}

func TestReopens(t *testing.T) {
	for _, c := range []int{146, 251, 527, 537, 538, 539} {
		if Reopens(c) {
			t.Errorf("class %d must not reopen", c)
		}
	}

	if !Reopens(148) || !Reopens(147) {
		t.Error("Akara and Gheed reopen")
	}
}

func TestKashyaHireNeedsLevelEight(t *testing.T) {
	for _, tc := range []struct {
		level int
		want  []Kind
	}{
		{1, []Kind{KindTalk}}, {7, []Kind{KindTalk}}, {8, []Kind{KindTalk, KindHire}}, {94, []Kind{KindTalk, KindHire}},
	} {
		rows, _ := Options(ClassKashya, Ctx{HeroLevel: tc.level})
		if !reflect.DeepEqual(kinds(rows), tc.want) {
			t.Errorf("level %d: %v want %v", tc.level, kinds(rows), tc.want)
		}
	}
}

func TestReviveRowPlacement(t *testing.T) {
	dead := Ctx{HeroLevel: 30, Expansion: true, MercDead: true, MercCost: 1234}

	// in front of the Hire row when there is one
	rows, _ := Options(ClassAsheara, dead)
	if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindRevive, KindHire, KindTrade}) {
		t.Errorf("Asheara: %v", kinds(rows))
	}

	rows, _ = Options(ClassKashya, dead)
	if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindRevive, KindHire}) {
		t.Errorf("Kashya: %v", kinds(rows))
	}

	// at the end when the NPC has no Hire row (Qual-Kehk, Tyrael) or Kashya's is not there yet
	for _, c := range []int{ClassQualKehk, ClassTyrael2} {
		rows, _ = Options(c, dead)
		if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindRevive}) {
			t.Errorf("class %d: %v", c, kinds(rows))
		}

		if rows[1].Cost != 1234 || rows[1].StringID != StrRevive {
			t.Errorf("class %d: revive row %+v", c, rows[1])
		}
	}

	low := dead
	low.HeroLevel = 3
	rows, _ = Options(ClassKashya, low)

	if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindRevive}) {
		t.Errorf("Kashya at level 3: %v", kinds(rows))
	}

	// no row: nothing dead, classic game, or an NPC that does not revive
	for _, c := range []Ctx{{HeroLevel: 30, Expansion: true}, {HeroLevel: 30, MercDead: true}} {
		rows, _ = Options(ClassGreiz, c)
		if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindHire}) {
			t.Errorf("Greiz %+v: %v", c, kinds(rows))
		}
	}

	rows, _ = Options(ClassAkara, dead)
	if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindTrade}) {
		t.Errorf("Akara got a revive row: %v", kinds(rows))
	}
}

func TestIdentifyRow(t *testing.T) {
	// hidden without unidentified items
	rows, _ := Options(ClassCain2, Ctx{})
	if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk}) {
		t.Errorf("no items: %v", kinds(rows))
	}

	// 100 gold each, with the fee string
	rows, _ = Options(ClassCain2, Ctx{Unidentified: 3})
	if len(rows) != 2 || rows[1].Cost != 300 || rows[1].StringID != StrIdentifyFee {
		t.Errorf("3 items: %+v", rows)
	}

	// free once quest 4 bit 0 or bit 1 is set: the plain label, no fee
	for _, bit := range []int{0, 1} {
		rows, _ = Options(ClassCain5, Ctx{Unidentified: 3, Quest: quest([2]int{4, bit})})
		if len(rows) != 2 || rows[1].Cost != 0 || rows[1].StringID != StrIdentify {
			t.Errorf("quest bit %d: %+v", bit, rows)
		}
	}

	if IdentifyCost(Ctx{Unidentified: 2, Quest: quest([2]int{4, 5})}) != 200 {
		t.Error("another bit of quest 4 does not make it free")
	}
}

func TestRespecGate(t *testing.T) {
	cases := []struct {
		name string
		c    Ctx
		want bool
	}{
		{"nothing set", Ctx{}, false},
		{"bit 1 pending", Ctx{Quest: quest([2]int{0x29, 1})}, true},
		{"bit 0 done wins", Ctx{Quest: quest([2]int{0x29, 1}, [2]int{0x29, 0})}, false},
		{"panel byte 2", Ctx{RespecPanelFlag: true}, true},
		{"panel byte 2 but done", Ctx{RespecPanelFlag: true, Quest: quest([2]int{0x29, 0})}, false},
	}

	for _, c := range cases {
		if got := RespecShown(c.c); got != c.want {
			t.Errorf("%s: %v want %v", c.name, got, c.want)
		}
	}

	// Akara's row is added only when the table has it
	c := Ctx{Quest: quest([2]int{0x29, 1})}
	rows, _ := Options(ClassAkara, c)

	if len(rows) != 2 {
		t.Errorf("respec row without the table string: %v", kinds(rows))
	}

	c.RespecRowPresent = true
	rows, _ = Options(ClassAkara, c)

	if !reflect.DeepEqual(kinds(rows), []Kind{KindTalk, KindTrade, KindRespec}) {
		t.Errorf("respec row: %v", kinds(rows))
	}
}

func TestMercClasses(t *testing.T) {
	for _, c := range []int{150, 198, 252, 515} {
		if !IsMercHirer(c) || !IsMercReviver(c) {
			t.Errorf("class %d", c)
		}
	}

	if IsMercHirer(367) || !IsMercReviver(367) || IsMercReviver(148) {
		t.Error("Tyrael revives but does not hire; Akara does neither")
	}
}

// ---- speech pools ----

func TestPoolsMatchTheNotes(t *testing.T) {
	cases := []struct {
		class, count, intro, variant int
		variantClass                 int
		noIntro                      bool
	}{
		{148, 13, 11, 12, 1, false}, // Akara: intro 11, sorceress intro 12, gossip 13..23
		{154, 9, 36, 37, 4, false},  // Charsi: barbarian intro
		{147, 9, 45, 46, 2, false},  // Gheed: necromancer intro
		{150, 12, 24, 25, 0, false}, // Kashya: amazon intro
		{155, 11, 0, 1, 3, false},   // Warriv: paladin intro
		{146, 7, 54, 54, 7, true},   // Cain 1: no introduction row
		{512, 12, 20014, 20014, 7, false},
	}

	for _, c := range cases {
		e, ok := EntryFor(c.class)
		if !ok {
			t.Fatalf("class %d has no entry", c.class)
		}

		if len(e.Records) != c.count || e.Records[0].Msg != c.intro || e.Records[1].Msg != c.variant ||
			e.Records[1].Class != c.variantClass || e.NoIntro != c.noIntro {
			t.Errorf("class %d: %d records, intro %d variant %d/%d noIntro=%v", c.class, len(e.Records),
				e.Records[0].Msg, e.Records[1].Msg, e.Records[1].Class, e.NoIntro)
		}
	}
}

func TestEveryMessageHasAKey(t *testing.T) {
	for _, e := range gossipEntries {
		for _, r := range e.Records {
			if MessageKey(r.Msg) == "" {
				t.Errorf("class %d message %d has no string key", e.Class, r.Msg)
			}
		}
	}

	if MessageKey(13) != "AkaraGossip1" || MessageKey(20003) != "CainAct5IntroGossip1" {
		t.Errorf("keys: %q %q", MessageKey(13), MessageKey(20003))
	}
}

func TestRecordEligibility(t *testing.T) {
	plain := Record{Msg: 1, Class: AnyClass}
	sorc := Record{Msg: 2, Class: 1}
	gated := Record{Msg: 3, Class: AnyClass, Gated: true, Quest: 6, Want: false}
	done := Record{Msg: 4, Class: AnyClass, Gated: true, Quest: 4, Want: true}
	q := quest([2]int{4, 0})

	for _, c := range []struct {
		r    Record
		hero int
		want bool
	}{
		{plain, 0, true}, {plain, 6, true}, {sorc, 1, true}, {sorc, 0, false},
		{gated, 0, true}, // quest 6 not done, wanted not done
		{done, 0, true},  // quest 4 done, wanted done
		{gated, 0, true},
	} {
		if got := c.r.Eligible(c.hero, q); got != c.want {
			t.Errorf("%+v hero %d: %v want %v", c.r, c.hero, got, c.want)
		}
	}

	if (Record{Class: AnyClass, Gated: true, Quest: 6, Want: true}).Eligible(0, q) {
		t.Error("quest 6 is not done")
	}

	if !(Record{Class: AnyClass, Gated: true, Quest: 6, Want: false}).Eligible(0, nil) {
		t.Error("a nil record reads every flag as clear")
	}
}

// scripted returns the given rolls in order, then 0.
func scripted(rolls ...int) func(int) int {
	i := 0

	return func(n int) int {
		if i >= len(rolls) {
			return 0
		}

		r := rolls[i] % n
		i++

		return r
	}
}

func TestPickGossip(t *testing.T) {
	akara, _ := EntryFor(ClassAkara)

	// rolls 0 and 1 are the introductions and are skipped; 5 is a plain line
	if got := PickGossip(akara, 0, nil, scripted(0, 1, 5)); got != 5 {
		t.Errorf("got %d want 5", got)
	}

	// record 2 is sorceress-only for Akara: other classes roll again, the sorceress keeps it
	if got := PickGossip(akara, 0, nil, scripted(2, 7)); got != 7 {
		t.Errorf("amazon got %d want 7", got)
	}

	if got := PickGossip(akara, 1, nil, scripted(2)); got != 2 {
		t.Errorf("sorceress got %d want 2", got)
	}

	// record 12 (message 23) needs quest 6 not done
	if got := PickGossip(akara, 0, quest([2]int{6, 0}), scripted(12, 4)); got != 4 {
		t.Errorf("quest 6 done: got %d want 4", got)
	}

	if got := PickGossip(akara, 0, nil, scripted(12)); got != 12 {
		t.Errorf("quest 6 open: got %d want 12", got)
	}

	// ten failed rolls end on index 2 even when it does not fit
	if got := PickGossip(akara, 0, nil, scripted(0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2)); got != 2 {
		t.Errorf("fallback got %d want 2", got)
	}

	// the tenth roll still counts, the eleventh does not
	if got := PickGossip(akara, 0, nil, scripted(0, 0, 0, 0, 0, 0, 0, 0, 0, 6)); got != 6 {
		t.Errorf("tenth roll got %d want 6", got)
	}

	// an NPC with no pool ends on 2 as well
	navi, _ := EntryFor(266)
	if got := PickGossip(navi, 0, nil, scripted(0)); got != 2 {
		t.Errorf("empty pool got %d", got)
	}
}

func TestIntroIndex(t *testing.T) {
	akara, _ := EntryFor(ClassAkara)
	malah, _ := EntryFor(ClassMalah)
	nih, _ := EntryFor(ClassNihlathak)
	qual, _ := EntryFor(ClassQualKehk)

	cases := []struct {
		e    *Entry
		hero int
		want int
	}{
		{akara, 0, 0}, {akara, 1, 1}, {akara, 4, 0},
		{malah, 4, 15}, {malah, 1, 1}, {malah, 0, 0},
		{nih, 2, 11}, {nih, 6, 1}, {nih, 0, 0}, // record 1 of Nihlathak is for the assassin
		{qual, 3, 10}, {qual, 5, 1}, {qual, 0, 0}, // record 1 of Qual-Kehk is for the druid
	}

	for _, c := range cases {
		if got := IntroIndex(c.e, c.hero); got != c.want {
			t.Errorf("class %d hero %d: %d want %d", c.e.Class, c.hero, got, c.want)
		}
	}

	// the lines they point at name the class
	if malah.Records[15].Class != 4 || nih.Records[11].Class != 2 || qual.Records[10].Class != 3 {
		t.Error("the class intro records are not for those classes")
	}
}

func TestSeed(t *testing.T) {
	s := Seed{V: 3<<32 | 2}
	if got := s.Below(10); got != 3 { // 0xD58D218D % 10
		t.Errorf("roll %d", got)
	}

	if s.V != 0xd58d218d {
		t.Errorf("seed %#x", s.V)
	}

	if (&Seed{V: 12345}).Below(0) != 0 {
		t.Error("n < 1")
	}

	// a power of two masks the low bits
	s2 := Seed{V: 3<<32 | 2}
	if got := s2.Below(8); got != 0xd58d218d&7 {
		t.Errorf("pow2 roll %d", got)
	}
}

func TestTalkRows(t *testing.T) {
	cases := []struct {
		class int
		cube  bool
		want  []TalkRow
	}{
		{ClassAkara, false, []TalkRow{TalkIntroduction, TalkGossip}},
		{ClassCain1, false, []TalkRow{TalkGossip}}, // no introduction
		{ClassJerhyn, false, []TalkRow{TalkIntroduction, TalkGossip, TalkMerchants}},
		{ClassCain2, true, []TalkRow{TalkGossip, TalkCube}},
		{ClassCain2, false, []TalkRow{TalkGossip}},
		{ClassCain4, true, []TalkRow{TalkIntroduction, TalkGossip, TalkCube}},
		{ClassCain6, true, []TalkRow{TalkIntroduction, TalkGossip, TalkCube}},
		{ClassAkara, true, []TalkRow{TalkIntroduction, TalkGossip}},
	}

	for _, c := range cases {
		if got := TalkRows(c.class, c.cube); !reflect.DeepEqual(got, c.want) {
			t.Errorf("class %d cube=%v: %v want %v", c.class, c.cube, got, c.want)
		}
	}
}

// ---- greeting gate ----

func TestGreetingPlaysOncePerRoom(t *testing.T) {
	g := NewGate()

	if play, mode := g.OpenGreeting(ClassAkara); !play || mode != GreetNormal {
		t.Fatalf("first open: %v %v", play, mode)
	}

	if play, _ := g.OpenGreeting(ClassAkara); play {
		t.Fatal("the greeting played twice in one room")
	}

	// another NPC is independent
	if play, _ := g.OpenGreeting(147); !play {
		t.Fatal("Gheed's greeting is separate")
	}

	g.RoomChanged(func(*Entry) int { return 2 })

	if play, _ := g.OpenGreeting(ClassAkara); !play {
		t.Fatal("a room change allows the greeting again")
	}

	// a class outside the array always plays
	for i := 0; i < 3; i++ {
		if play, mode := g.OpenGreeting(9999); !play || mode != GreetNormal {
			t.Fatal("unlisted class")
		}
	}
}

func TestReturnGreeting(t *testing.T) {
	g := NewGate()
	g.MarkReturn(ClassAkara, 9999) // unknown ids are ignored

	if !g.ReturnPending(ClassAkara) {
		t.Fatal("not marked")
	}

	if play, mode := g.OpenGreeting(ClassAkara); !play || mode != GreetReturn {
		t.Fatalf("return greeting: %v %v", play, mode)
	}

	if g.ReturnPending(ClassAkara) {
		t.Fatal("the flag survives use")
	}

	// the flag is spent even when the greeting is not allowed (second open in the room)
	g.MarkReturn(ClassAkara)

	if play, mode := g.OpenGreeting(ClassAkara); play || mode != GreetReturn {
		t.Fatalf("blocked open: %v %v", play, mode)
	}

	if g.ReturnPending(ClassAkara) {
		t.Fatal("the flag survives a blocked open")
	}

	g.MarkReturn(ClassAkara)
	g.Reset()

	if g.ReturnPending(ClassAkara) {
		t.Fatal("Reset keeps the flag")
	}
}

func TestGossipRerollOnlyForNPCsThatSpoke(t *testing.T) {
	g := NewGate()
	calls := map[int]int{}
	roll := func(e *Entry) int { calls[e.Class]++; return 5 }

	g.Spoke(ClassAkara, roll) // the first speech rolls everybody once

	total := 0
	for _, n := range calls {
		total += n
	}

	if total != EntryCount() || calls[ClassAkara] != 1 {
		t.Fatalf("first speech rolled %d entries", total)
	}

	if g.Chosen(ClassAkara) != 5 || g.Chosen(147) != 5 {
		t.Fatal("chosen not stored")
	}

	g.Spoke(147, roll) // no second full roll

	if calls[ClassAkara] != 1 {
		t.Fatal("rolled everybody twice")
	}

	g.RoomChanged(roll)

	if calls[ClassAkara] != 2 || calls[147] != 2 || calls[154] != 1 {
		t.Fatalf("room change rolled %v", calls)
	}

	g.RoomChanged(roll)

	if calls[ClassAkara] != 2 {
		t.Fatal("rolled again without a speech")
	}
}

// ---- healing ----

func TestHealers(t *testing.T) {
	for _, c := range []int{148, 513, 176, 178, 405, 255} {
		if !IsHealer(c) {
			t.Errorf("class %d should heal", c)
		}
	}

	for _, c := range []int{147, 154, 177, 199, 253, 511, 512, 150} {
		if IsHealer(c) {
			t.Errorf("class %d should not heal", c)
		}
	}
}

func TestRestore(t *testing.T) {
	v := Vitals{Life: 10, MaxLife: 300, Mana: 5, MaxMana: 80, Stamina: 20, MaxStamina: 100}

	out, changed := Restore(v)
	if !changed || out.Life != 300 || out.Mana != 80 || out.Stamina != 100 {
		t.Errorf("got %+v changed=%v", out, changed)
	}

	// full pools: nothing changes
	if _, changed := Restore(out); changed {
		t.Error("full hero reported a change")
	}

	// above the maximum (a potion buff that ended): never lowered
	over := Vitals{Life: 400, MaxLife: 300, Mana: 90, MaxMana: 80, Stamina: 120, MaxStamina: 100}
	if got, changed := Restore(over); changed || got != over {
		t.Errorf("over-full pools were touched: %+v", got)
	}

	// a single pool
	if got, changed := Restore(Vitals{Life: 300, MaxLife: 300, Mana: 1, MaxMana: 2, Stamina: 100, MaxStamina: 100}); !changed || got.Mana != 2 {
		t.Errorf("mana only: %+v", got)
	}
}

func TestHealingCost(t *testing.T) {
	cases := []struct {
		level                        int
		life, maxLife, mana, maxMana uint32
		want                         int
	}{
		{10, 100 << 8, 100 << 8, 50 << 8, 50 << 8, 0},
		{10, 60 << 8, 100 << 8, 50 << 8, 50 << 8, 100},   // 10 * 40 >> 2
		{20, 60 << 8, 100 << 8, 10 << 8, 50 << 8, 400},   // 20 * (40+40) >> 2
		{3, 100<<8 - 255, 100 << 8, 50 << 8, 50 << 8, 0}, // less than one point lost
		{1, 99 << 8, 100 << 8, 50 << 8, 50 << 8, 0},      // 1 * 1 >> 2
		{4, 99 << 8, 100 << 8, 50 << 8, 50 << 8, 1},
		{99, 0, 1000 << 8, 0, 0, 24750},
	}

	for _, c := range cases {
		if got := HealingCost(c.level, c.life, c.maxLife, c.mana, c.maxMana); got != c.want {
			t.Errorf("%+v: %d", c, got)
		}
	}
}
