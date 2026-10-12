package d2hero

import "testing"

// Wiki round 3 (w41): death penalties. Exe: PLAYER_ServerCreateCorpse 0x57d6f0
// moves ALL inventory gold (stat 0xe) into piles of at most 2e9 (0x557fe0, at
// most 32 piles) and sets it to 0; the stash is untouched. The corpse xp stat
// is 75% of the penalty value.

func TestW41DeathPenaltyTable(t *testing.T) {
	for d, pct := range []int{0, 5, 10} {
		if got := DeathExpPenaltyPercent(d); got != pct {
			t.Errorf("difficulty %d penalty %d%%, want %d%%", d, got, pct)
		}
	}

	// Hell: lose 10% of the level span, but stay inside the level.
	lost, now := ExpPenalty(10, 80, 1_005_000, 1_000_000, 1_010_000)
	if lost != 1000 || now != 1_004_000 {
		t.Errorf("hell penalty lost %d new %d", lost, now)
	}

	lost, now = ExpPenalty(10, 80, 1_000_300, 1_000_000, 1_010_000)
	if now != 1_000_001 || lost != 299 {
		t.Errorf("penalty near level start lost %d new %d, want to stay at start+1", lost, now)
	}

	if lost, _ := ExpPenalty(10, 1, 100, 0, 500); lost != 0 {
		t.Error("level 1 never loses experience")
	}

	if lost, _ := ExpPenalty(0, 50, 5000, 4000, 6000); lost != 0 {
		t.Error("normal difficulty has no penalty")
	}
}

func TestW41DeathDropsAllCarriedGold(t *testing.T) {
	var s DeathState

	out := s.Die(DeathInput{Difficulty: 2, Level: 60, Experience: 5000, ExpStart: 4000, ExpNext: 14000, Gold: 600000})
	if out.GoldDropped != 600000 {
		t.Errorf("gold dropped %d, want all 600000", out.GoldDropped)
	}

	if out.Corpse == nil || out.Corpse.Experience != 750 {
		t.Errorf("corpse %+v, want experience stat 75%% of 1000", out.Corpse)
	}
}
