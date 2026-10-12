package d2quest

import "testing"

// The Blade of the Old Religion (D2MOO ACT3Q3, exe 0x5b6ea0): the Gidbinn object wakes the fetish boss (class 407);
// the boss's death drops the blade where it fell; taking it moves the quest to the hand-over to Ormus. The altar (251)
// is scenery and does nothing; a killed fetish that is not the woken boss drops nothing.
func TestGidbinnObjectWakesTheBossWhichDropsTheBlade(t *testing.T) {
	g, _ := newGame(t)
	q := g.Quest(QuestBlade)

	moveTo(g, 1, LevelKurastDocktown)

	// VERIFIED shape (D2MOO ACT3Q3_Callback03_ChangedLevel): Hratli offers the quest after the Flayer Jungle was entered
	if msgs, _ := talk(g, NPCHratli); contains2(msgs, 571) {
		t.Fatalf("Hratli offered the Blade before the Flayer Jungle: %v", msgs)
	}

	moveTo(g, LevelKurastDocktown, LevelFlayerJungle)
	moveTo(g, LevelFlayerJungle, LevelKurastDocktown)

	msgs, _ := talk(g, NPCHratli)
	if !contains2(msgs, 571) {
		t.Fatalf("Hratli: %v %s", msgs, g.Describe(q))
	}

	if eff := obj(g, ObjectGidbinnAltar, 78); effectCode(eff, EffectSpawn, GidbinnBossCode) {
		t.Fatalf("the altar woke the boss: %+v", eff)
	}

	if eff := kill(g, NPCGidbinnBoss, 78); effectCode(eff, EffectSpawn, GidbinnDropCode) {
		t.Fatalf("a fetish killed before the object was used dropped the blade: %+v", eff)
	}

	eff := obj(g, ObjectGidbinn, 78)
	if !effectCode(eff, EffectSpawn, GidbinnBossCode) || effectCode(eff, EffectSpawn, ItemGidbinn) {
		t.Fatalf("the object must wake the boss and give no item: %+v", eff)
	}

	if eff := obj(g, ObjectGidbinn, 78); effectCode(eff, EffectSpawn, GidbinnBossCode) {
		t.Fatalf("the boss woke twice: %+v", eff)
	}

	if eff := kill(g, NPCGidbinnBoss, 78); !effectCode(eff, EffectSpawn, GidbinnDropCode) {
		t.Fatalf("the boss's death must drop the blade: %+v", eff)
	}

	pickup(g, ItemGidbinn)

	if q.State != 4 {
		t.Errorf("state %d after taking the blade, want 4 (%s)", q.State, g.Describe(q))
	}
}
