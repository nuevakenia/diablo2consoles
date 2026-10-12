package d2monsters

import "testing"

func TestRiteHostPortalsAndReset(t *testing.T) {
	d := &Director{}
	d.RiteSetPortals(2)

	if d.PortalCount() != 2 {
		t.Fatalf("portal count %d", d.PortalCount())
	}

	d.rite.Started = true
	d.rite.Statues = [3]int{3, 3, 3}

	if !d.RiteStarted() {
		t.Fatal("started")
	}

	d.RiteReset()

	if d.RiteStarted() || d.rite.Statues != [3]int{} || d.PortalCount() != 2 {
		t.Errorf("reset clears the fight, not the portal counter: %+v", *d.rite)
	}
}
