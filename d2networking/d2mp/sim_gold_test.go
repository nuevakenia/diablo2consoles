package d2mp

import "testing"

// Gold picked up by a party member is shared with the party members of the
// same level (d2party.Roster.ShareGold, 0x53e640) and capped by each carry limit.
func TestPickUpGoldPartyShare(t *testing.T) {
	h := newHarness(t, strongHero{}, 40)
	h.join(1, "A", 8)
	h.join(2, "B", 8)
	h.join(3, "C", 8)
	h.sim.Command(1, Command{Type: CmdPartyInvite, Target: 2})
	h.sim.Command(2, Command{Type: CmdPartyAccept})
	h.settle()

	h.sim.pickUpGold(h.sim.pl[1], 100) // two in the party: 50 each
	if a, b, c := h.sim.pl[1].u.Gold, h.sim.pl[2].u.Gold, h.sim.pl[3].u.Gold; a != 50 || b != 50 || c != 0 {
		t.Fatalf("gold a=%d b=%d c=%d want 50 50 0", a, b, c)
	}

	h.sim.pl[2].u.Gold = 80000 - 0 // above the level 8 limit of 80000? exactly full
	h.sim.pickUpGold(h.sim.pl[1], 100)

	if a, b := h.sim.pl[1].u.Gold, h.sim.pl[2].u.Gold; a != 150 || b != 80000 {
		t.Fatalf("full member: a=%d b=%d want 150 80000", a, b)
	}
}
