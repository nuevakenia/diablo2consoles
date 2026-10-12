package d2quest

import "testing"

// Five freed barbarians that die fail the quest (exe 0x585f30: the killed counter at +0xa8 reaches 5); a door that
// freed nobody (Freed < 0) adds nothing.
func TestRescueDeadBarbariansBlockTheGoal(t *testing.T) {
	g, _ := newGame(t)
	q := g.Quest(QuestRescue)

	g.ForceDone(QuestSiege)

	moveTo(g, 1, LevelHarrogath)
	talk(g, NPCQualKehk)
	moveTo(g, LevelHarrogath, LevelFrigidHighlands)

	for i := 0; i < 5; i++ {
		g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCWussie, Level: LevelFrigidHighlands})
	}

	for i := 0; i < 3; i++ {
		g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCPrisonDoor, Level: LevelFrigidHighlands, Freed: 5})
	}

	if g.get(q, FlagRewardPending) {
		t.Fatalf("five dead barbarians must fail the quest: %s", g.Describe(q))
	}
}

func TestRescueTenFreedDoNotFinishTheGoal(t *testing.T) {
	g, _ := newGame(t)
	q := g.Quest(QuestRescue)

	g.ForceDone(QuestSiege)

	moveTo(g, 1, LevelHarrogath)
	talk(g, NPCQualKehk)
	moveTo(g, LevelHarrogath, LevelFrigidHighlands)

	for _, n := range []int{5, 5, -1} {
		g.Dispatch(Event{Kind: EvMonsterKilled, Monster: NPCPrisonDoor, Level: LevelFrigidHighlands, Freed: n})
	}

	if g.get(q, FlagRewardPending) {
		t.Fatalf("ten freed barbarians must not finish the goal: %s", g.Describe(q))
	}
}

// The Act 5 quests open one after the other (D2MOO nSeqId 32..35 and the SeqCallbacks; Siege, Rescue, Prison of Ice,
// Betrayal, Rite): a quest before its turn offers nothing.
func TestAct5QuestsOpenInOrder(t *testing.T) {
	chain := []int{QuestSiege, QuestRescue, QuestPrison, QuestBetrayal, QuestRite}

	g, _ := newGame(t)
	g.Start()

	for i, id := range chain {
		q := g.Quest(id)
		if i == 0 {
			if q.State != 1 {
				t.Fatalf("the Siege must be open at the start: %s", g.Describe(q))
			}

			continue
		}

		if q.State != 0 {
			t.Fatalf("%s is open before %s is done: %s", q.Name, g.Quest(chain[i-1]).Name, g.Describe(q))
		}
	}

	for i, id := range chain[:len(chain)-1] {
		g.ForceDone(id)

		if next := g.Quest(chain[i+1]); next.State != 1 {
			t.Fatalf("%s did not open after %s: %s", next.Name, g.Quest(id).Name, g.Describe(next))
		}
	}

	// a quest out of turn: Qual-Kehk has no Rescue line before the Siege is done
	g2, _ := newGame(t)
	moveTo(g2, 1, LevelHarrogath)

	if msgs, _ := talk(g2, NPCQualKehk); contains2(msgs, 20096) {
		t.Fatalf("Qual-Kehk offered the Rescue before the Siege was done: %v", msgs)
	}
}
