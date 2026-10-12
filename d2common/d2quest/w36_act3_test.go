package d2quest

import "testing"

func TestW36OrificeNeedsTheRealTomb(t *testing.T) {
	g, _ := newGame(t)
	q := g.Quest(QuestHoradricStaff)

	g.RealTomb = 70
	g.Items[ItemHoradricStaff] = 1

	// a decoy tomb does not take the staff
	obj(g, ObjectOrifice, 68)

	if g.get(q, FlagRewardGranted) || g.Items[ItemHoradricStaff] != 1 {
		t.Fatalf("a decoy tomb took the staff: %s", g.Describe(q))
	}

	eff := obj(g, ObjectOrifice, 70)
	if !g.get(q, FlagRewardGranted) || !hasEffectPortal(eff, LevelDurielLair) {
		t.Fatalf("the real tomb: %s %+v", g.Describe(q), eff)
	}
}

func TestW36LamEsenRewardValue(t *testing.T) {
	g, _ := newGame(t)

	moveTo(g, 1, LevelKurastDocktown)
	moveTo(g, LevelKurastDocktown, LevelLowerKurast)
	moveTo(g, LevelLowerKurast, LevelKurastDocktown)
	talk(g, NPCAlkor)
	moveTo(g, LevelKurastDocktown, LevelRuinedTemple)
	pickup(g, ItemLamEsenTome)
	moveTo(g, LevelRuinedTemple, LevelKurastDocktown)

	_, eff := talk(g, NPCAlkor)

	n := 0

	for _, e := range eff {
		if e.Kind == EffectReward && e.Code == "stat-points" {
			n += e.Value
		}
	}

	if n != 5 {
		t.Errorf("Lam Esen's Tome gave %d stat points, want 5 (once)", n)
	}

	pageIs(t, g, QuestLamEsen, 13, "paid by Alkor")
}
