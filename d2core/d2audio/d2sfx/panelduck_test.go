package d2sfx

import "testing"

func TestPanelDuckStep(t *testing.T) {
	v := 100
	for i := 0; i < 25; i++ {
		v = PanelDuckStep(v, true)
	}

	if v != 0 {
		t.Fatalf("closing duck ends at %d, want 0", v)
	}

	if got := PanelDuckStep(3, true); got != 0 {
		t.Errorf("3 -> %d, want 0 (floor)", got)
	}

	if got := PanelDuckStep(0, false); got != 5 {
		t.Errorf("0 open->close = %d, want 5", got)
	}

	if got := PanelDuckStep(98, false); got != 100 {
		t.Errorf("98 -> %d, want 100 (ceiling)", got)
	}

	if got := PanelDuckStep(100, true); got != 95 {
		t.Errorf("100 -> %d, want 95", got)
	}
}

func TestPanelDuckExempt(t *testing.T) {
	for idx, want := range map[int]bool{0: false, 1: true, 15: true, 16: false, 0x33: false, 0x34: true, 0x47: true,
		0x48: false, 4679: true, 0x1230: false, 0x1231: true, 0x125a: true, 0x125b: false} {
		if PanelDuckExempt(idx) != want {
			t.Errorf("PanelDuckExempt(%#x) = %v, want %v", idx, !want, want)
		}
	}
}
