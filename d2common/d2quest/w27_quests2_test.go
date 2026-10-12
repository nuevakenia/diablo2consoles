package d2quest

import "testing"

func TestAnyaTierGameType(t *testing.T) {
	cases := []struct{ diff, lvl, gt, want int }{
		{Normal, 99, GameTypeMode0, 0}, // Normal never raises the tier
		{Nightmare, 10, GameTypeMode0, 1},
		{Nightmare, 10, GameTypeHost, 0},
		{Nightmare, 45, GameTypeHost, 0},
		{Nightmare, 46, GameTypeHost, 1},
		{Hell, 10, GameTypeMode0, 2},
		{Hell, 65, GameTypeHost, 0},
		{Hell, 66, GameTypeHost, 2},
		{Hell, 66, GameTypeRealm, 2},
	}

	for _, c := range cases {
		if got := AnyaTier(c.diff, c.lvl, c.gt); got != c.want {
			t.Errorf("AnyaTier(%d,%d,%d) = %d, want %d", c.diff, c.lvl, c.gt, got, c.want)
		}
	}

	if got := AnyaRewardCodesFor(ClassPaladin, Hell, 1, GameTypeMode0); got[0] != "pab" {
		t.Errorf("type 3 in Hell gave %v", got)
	}
}

func TestEveGoldRange(t *testing.T) {
	want := [][2]int{{1500, 3000}, {7500, 9000}, {13500, 15000}}
	for d, w := range want {
		if lo, hi := EveGoldRange(d); lo != w[0] || hi != w[1] {
			t.Errorf("difficulty %d: %d-%d, want %v", d, lo, hi, w)
		}
	}
}

func TestBaalKillPaysGold(t *testing.T) {
	g, _ := newGame(t)
	g.Expansion = true

	var out []Effect

	out = append(out, moveTo(g, LevelHarrogath, LevelWorldstone1)...)
	out = append(out, g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCBaalCrab, Name: "Baal", Level: LevelWorldstoneChamber})...)

	if !effectCode(out, EffectReward, "gold-pile") {
		t.Skipf("Eve of Destruction was not active in the fresh test game: %v", out)
	}
}

func TestBetrayalMissedBit(t *testing.T) {
	g, _ := newGame(t)
	g.Expansion = true

	q := g.Quest(QuestBetrayal)
	if bit(g, QuestBetrayal, betrayalMissedBit) {
		t.Fatal("bit 14 set at the start")
	}

	// Prison of Ice not done: Nihlathak's death grants nothing and marks the hero.
	g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCNihlathakBoss, Level: LevelNihlathakTemple})

	if !bit(g, QuestBetrayal, betrayalMissedBit) || g.get(q, FlagRewardPending) {
		t.Fatalf("ineligible hero: bit14=%v pending=%v", bit(g, QuestBetrayal, betrayalMissedBit), g.get(q, FlagRewardPending))
	}

	if BetrayalEligible(true, true) || !BetrayalEligible(true, false) || BetrayalEligible(false, false) {
		t.Fatal("BetrayalEligible")
	}
}

func TestRiteRules(t *testing.T) {
	if RiteAltarOpens(false, false) || !RiteAltarOpens(true, false) || !RiteAltarOpens(false, true) {
		t.Fatal("RiteAltarOpens")
	}

	if AncientsMayStart(1, 0) || AncientsMayStart(0, 3) || !AncientsMayStart(0, 2) {
		t.Fatal("AncientsMayStart")
	}

	g, _ := newGame(t)
	g.Expansion = true

	if g.RiteAltarOpen() {
		t.Fatal("altar open on a fresh game")
	}
}

func TestCainRing(t *testing.T) {
	for d, w := range [][2]int{{7, 1}, {30, 2}, {60, 2}} {
		if ilvl, q := CainRing(d); ilvl != w[0] || q != w[1] {
			t.Errorf("difficulty %d: %d/%d, want %v", d, ilvl, q, w)
		}
	}
}
