package d2party

import "testing"

// Wiki cross-check (wave 30): party experience. Wiki (recalled, WIKI-ONLY):
// shares are proportional to character level and a party earns a bonus per
// extra member. Engine: exe-verified 0x0057c6b0, bonus 89/256 per extra member.
func TestWikiPartyExperienceSplit(t *testing.T) {
	if got := SplitKillXP(1000, []int{50}); got[0] != 1000 {
		t.Errorf("solo %v", got)
	}

	got := SplitKillXP(1000, []int{50, 50})
	// total = 1000 + 1000*89/256 = 1347 -> 673 each (float truncation)
	if got[0] < 672 || got[0] > 674 || got[0] != got[1] {
		t.Errorf("two equal members %v", got)
	}

	// a member twice the level of the other takes twice the share.
	got = SplitKillXP(900, []int{20, 40})
	if d := got[1] - 2*got[0]; d < -2 || d > 2 {
		t.Errorf("level weighting %v", got)
	}
}
