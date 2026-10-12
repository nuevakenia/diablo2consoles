package d2resource

import "testing"

func TestClassicPaths(t *testing.T) {
	if b, x := CreditsPaths(true); b != CreditsBackground || x != CreditsText {
		t.Fatal("expansion credits")
	}

	if b, x := CreditsPaths(false); b != CreditsBackgroundClassic || x != CreditsTextClassic || b == CreditsBackground {
		t.Fatal("classic credits")
	}

	if CinematicsBackgroundFor(true) == CinematicsBackgroundFor(false) {
		t.Fatal("cinematics background must differ")
	}
}
