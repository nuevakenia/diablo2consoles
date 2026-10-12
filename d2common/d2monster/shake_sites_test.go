package d2monster

import "testing"

func TestShakeSitesEnvelope(t *testing.T) {
	for name, s := range map[string]Shake{
		"scene": ShakeSceneEvent, "f65": ShakeDoFunc65, "m150": ShakeMissile150,
		"m325": ShakeMissile325, "f37": ShakeDoFunc37, "f53": ShakeDoFunc53,
	} {
		if !s.Active(0) || s.Magnitude(s.RampMs+s.HoldMs/2) != s.Amp {
			t.Errorf("%s: hold magnitude", name)
		}

		if s.Active(s.RampMs + s.HoldMs + s.DecayMs + 1) {
			t.Errorf("%s: still active after the end", name)
		}
	}

	// ramp and decay are linear
	if got := ShakeSceneEvent.Magnitude(2000); got != 3 {
		t.Errorf("half ramp: %d", got)
	}

	if got := ShakeDoFunc65.Magnitude(2000 + 3000 + 1000); got != 3 {
		t.Errorf("half decay: %d", got)
	}

	// no decay: the hold magnitude is kept until the end
	if got := ShakeMissile150.Magnitude(4000); got != 25 {
		t.Errorf("m150 end: %d", got)
	}
}
