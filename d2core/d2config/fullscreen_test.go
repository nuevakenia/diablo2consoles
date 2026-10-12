package d2config

import (
	"encoding/json"
	"testing"
)

func TestFullscreenDefaultsToOn(t *testing.T) {
	var c Configuration // an old config.json: no option, FullScreen false
	if !c.WantFullscreen() {
		t.Fatal("a config without the option must start full screen")
	}
}

func TestFullscreenChoicePersists(t *testing.T) {
	var c Configuration

	c.SetFullscreenChoice(false)

	data, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	var back Configuration
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}

	if back.WantFullscreen() || back.FullScreen {
		t.Fatalf("window choice lost after a save and load: %+v", back)
	}

	back.SetFullscreenChoice(true)

	if !back.WantFullscreen() || !back.FullScreen || back.OptionValue(OptFullscreen) != "ON" {
		t.Fatalf("full screen choice not stored: %+v", back)
	}
}
