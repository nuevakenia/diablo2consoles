package d2gamescreen

import "testing"

func TestKindLines(t *testing.T) {
	joined := joinKindLines([]string{"Hardcore", " ", "Expansion Character"})

	a, b := splitKindLines(joined)
	if a == "" || b == "" || a == b {
		t.Fatalf("two lines expected, got %q and %q", a, b)
	}

	one, two := splitKindLines(joinKindLines([]string{"Expansion Character"}))
	if one == "" || two != "" {
		t.Fatalf("one line expected, got %q and %q", one, two)
	}

	if x, y := splitKindLines(joinKindLines(nil)); x != "" || y != "" {
		t.Fatalf("classic softcore has no kind line, got %q %q", x, y)
	}
}
