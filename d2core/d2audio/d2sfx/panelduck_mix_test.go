package d2sfx

import "testing"

func TestPanelDuckAppliesInMix(t *testing.T) {
	rows := []Row{row(0, nil), row(1, func(r *Row) { r.Loop = true }), row(20, func(r *Row) { r.Loop = true })}
	h := newHarness(rows, 4)
	h.b.Play(Request{Index: 20})
	h.b.Play(Request{Index: 1})
	h.b.SetPanelOpen(true)

	for i := 0; i < 30; i++ {
		h.clock.t++
		h.b.Advance()
	}

	if h.b.PanelDuck() != 0 {
		t.Fatalf("panel duck %d, want 0", h.b.PanelDuck())
	}

	if v := h.players[0].vol; v != 0 {
		t.Errorf("ordinary sound under an open menu has volume %v, want 0", v)
	}

	if v := h.players[1].vol; v != 1 {
		t.Errorf("exempt sound ducked to %v, want 1", v)
	}
}
