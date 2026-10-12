package ebiten

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2config"
)

func TestStartFullscreen(t *testing.T) {
	window := &d2config.Configuration{}
	window.SetFullscreenChoice(false)

	tests := []struct {
		name   string
		cfg    *d2config.Configuration
		always bool
		want   bool
	}{
		{"normal start, nothing chosen", &d2config.Configuration{}, true, true},
		{"normal start, player chose a window", window, true, false},
		{"scripted run keeps the window", &d2config.Configuration{}, false, false},
		{"scripted run with FullScreen set", &d2config.Configuration{FullScreen: true}, false, true},
	}

	for _, tt := range tests {
		if got := startFullscreen(tt.cfg, tt.always); got != tt.want {
			t.Errorf("%s: got %v want %v", tt.name, got, tt.want)
		}
	}
}
