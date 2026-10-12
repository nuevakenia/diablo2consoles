package d2player

import "testing"

func TestHelpTextAnchorsMatchBaselines(t *testing.T) {
	if len(HelpTextAnchors800) != len(HelpTextBaselines800) {
		t.Fatalf("len %d vs %d", len(HelpTextAnchors800), len(HelpTextBaselines800))
	}

	for i, a := range HelpTextAnchors800 {
		if a.Y != HelpTextBaselines800[i] {
			t.Errorf("anchor %d y %#x vs baseline %#x", i, a.Y, HelpTextBaselines800[i])
		}

		if !a.Centered && a.X != 0x75 {
			t.Errorf("anchor %d left x %#x", i, a.X)
		}
	}
}
