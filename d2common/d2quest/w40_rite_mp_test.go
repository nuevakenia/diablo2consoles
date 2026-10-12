package d2quest

import "testing"

func TestRiteAltarStep(t *testing.T) {
	tests := []struct {
		name    string
		node    RiteNode
		portals int
		want    RiteAltarResult
	}{
		{"inactive node, no portal", RiteNode{State: 1}, 0, RiteAltarResult{Speak: true, NewState: 1}},
		{"inactive node, portal", RiteNode{State: 1}, 1, RiteAltarResult{Speak: true, ResetAncients: true, NewState: 1}},
		{"active, state 1 lifts to 2 and refreshes the log", RiteNode{Active: true, State: 1}, 0,
			RiteAltarResult{Speak: true, NewState: 2, RefreshLog: true}},
		{"active, state 3, log already refreshed", RiteNode{Active: true, State: 3, Sub: 3}, 2,
			RiteAltarResult{Speak: true, ResetAncients: true, NewState: 3}},
		{"active, state 4 does nothing", RiteNode{Active: true, State: 4}, 1, RiteAltarResult{NewState: 4}},
	}

	for _, tc := range tests {
		if got := RiteAltarStep(tc.node, tc.portals); got != tc.want {
			t.Errorf("%s: got %+v want %+v", tc.name, got, tc.want)
		}
	}
}

func TestRiteLogRefreshes(t *testing.T) {
	bits := func(set ...int) func(int) bool {
		return func(b int) bool {
			for _, s := range set {
				if s == b {
					return true
				}
			}

			return false
		}
	}

	if RiteLogRefreshes(bits()) || RiteLogRefreshes(bits(FlagRewardGranted)) || RiteLogRefreshes(bits(FlagPrimaryGoal)) {
		t.Error("the callback needs a done bit and a goal bit")
	}

	if !RiteLogRefreshes(bits(FlagRewardGranted, FlagPrimaryGoal)) || !RiteLogRefreshes(bits(FlagCompletedEarly, FlagCompletedNow)) {
		t.Error("done + goal refreshes the log")
	}
}

func TestRiteDeathOutcomes(t *testing.T) {
	heroes := []RiteHero{
		{ID: "killer", Level: 30, Area: LevelSummit},
		{ID: "low", Level: 10, Area: LevelSummit},
		{ID: "friend", Level: 45, Area: 109, Party: "p"},          // Harrogath, party of the summit hero
		{ID: "partner", Level: 45, Area: LevelSummit, Party: "p"}, // pays and spreads to its party
		{ID: "town", Level: 45, Area: 1, Party: "p"},              // Act 1: out of the party's reach
		{ID: "away", Level: 50, Area: 40},
		{ID: "old", Level: 50, Area: LevelSummit, Done: true},
	}

	got := RiteDeathOutcomes("killer", Normal, heroes)
	want := map[string]RiteOutcome{
		"killer": RiteReward, "low": RiteGoalOnly, "friend": RiteReward, "partner": RiteReward,
		"town": RiteGoalOnly, "away": RiteGoalOnly,
	}

	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: got %v want %v", id, got[id], w)
		}
	}

	if _, ok := got["old"]; ok {
		t.Error("a hero that was done already is left alone")
	}

	// Nightmare needs level 40: the 30 killer no longer qualifies and only gets the goal mark
	nm := RiteDeathOutcomes("killer", Nightmare, heroes)
	if nm["killer"] != RiteGoalOnly || nm["partner"] != RiteReward {
		t.Errorf("nightmare gate: %v", nm)
	}
}

func TestRiteDeathKillerElsewhere(t *testing.T) {
	// the killer is rewarded wherever it stands (0x58a7b0 tests no area for it) and does not spread to its party
	h := []RiteHero{
		{ID: "k", Level: 25, Area: 110, Party: "p"},
		{ID: "m", Level: 25, Area: 109, Party: "p"},
	}

	got := RiteDeathOutcomes("k", Normal, h)
	if got["k"] != RiteReward || got["m"] != RiteGoalOnly {
		t.Errorf("killer outside the summit: %v", got)
	}
}

func TestRiteShared(t *testing.T) {
	g, _ := newGame(t)
	g.Hero.Level = 50

	q := g.Quest(QuestRite)
	if q == nil {
		t.Fatal("no Rite quest")
	}

	if !g.RiteShared(RiteReward) || !g.get(q, FlagRewardGranted) || !g.get(q, FlagPrimaryGoal) {
		t.Fatal("a shared reward sets bits 0 and 13")
	}

	if g.RiteShared(RiteReward) {
		t.Error("the reward is paid once")
	}

	fx := g.TakeEffects()
	n := 0

	for _, e := range fx {
		if e.Kind == EffectReward && e.Code == "experience" && e.Value == RiteExperience(Normal) {
			n++
		}
	}

	if n != 1 {
		t.Errorf("experience effects: %d", n)
	}

	g2, _ := newGame(t)
	g2.Hero.Level = 5

	if g2.RiteShared(RiteReward) {
		t.Error("below the level gate nothing is paid")
	}

	if !g2.RiteShared(RiteGoalOnly) || !g2.get(g2.Quest(QuestRite), FlagCompletedNow) {
		t.Error("the goal mark is bit 14")
	}

	if g2.RiteShared(RiteGoalOnly) == false && g2.get(g2.Quest(QuestRite), FlagRewardGranted) {
		t.Error("goal mark must not grant")
	}
}

func TestRiteStartDelay(t *testing.T) {
	if RiteStatueWakeFrames*1000/RiteFramesPerSecond != 800 {
		t.Error("20 frames at 25 fps are 0.8 s")
	}
}
