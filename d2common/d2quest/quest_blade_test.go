package d2quest

import "testing"

// The Blade of the Old Religion: the Gidbinn objects (the altar and the blade on it) give the item, taking it moves the
// quest to the hand-over to Ormus. (Nothing gave the item before, so the quest could not be played.)
func TestGidbinnObjectsGiveTheBlade(t *testing.T) {
	for _, id := range []int{ObjectGidbinnAltar, ObjectGidbinn} {
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

		eff := obj(g, id, 78)
		if !effectCode(eff, EffectSpawn, ItemGidbinn) {
			t.Fatalf("object %d: %+v", id, eff)
		}

		pickup(g, ItemGidbinn)

		if q.State != 4 {
			t.Errorf("object %d: state %d after taking the blade, want 4 (%s)", id, q.State, g.Describe(q))
		}
	}
}
