package d2config

import "testing"

func TestCinematicsOptions(t *testing.T) {
	c := DefaultConfig()

	if !c.Cinematics() {
		t.Fatal("cinematics default to ON")
	}

	if c.IntroSeen() {
		t.Fatal("the opening movie was not seen yet")
	}

	if err := c.SetOption(OptCinematics, 1); err != nil || c.Cinematics() {
		t.Fatalf("OFF: %v %v", err, c.Cinematics())
	}

	if err := c.SetOption(OptIntroSeen, 1); err != nil || !c.IntroSeen() {
		t.Fatalf("seen: %v %v", err, c.IntroSeen())
	}

	if c.OptionValue(OptCinematics) != "OFF" {
		t.Fatalf("value %q", c.OptionValue(OptCinematics))
	}
}
