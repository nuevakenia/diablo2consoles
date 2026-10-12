package d2mapentity

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

func TestStatTransparency(t *testing.T) {
	tests := []struct {
		name     string
		player   bool
		stat     int
		ethereal bool
		want     bool
	}{
		{"player zero", true, 0, false, false},
		{"player any", true, 7, false, true},
		{"player negative", true, -1, false, true},
		{"monster zero", false, 0, true, false},
		{"monster low", false, 5, false, true},
		{"monster 15", false, 15, false, true},
		{"monster high needs ethereal", false, 16, false, false},
		{"monster high with ethereal", false, 16, true, true},
	}

	for _, tc := range tests {
		if got := StatTransparency(tc.player, tc.stat, tc.ethereal); got != tc.want {
			t.Errorf("%s: got %v", tc.name, got)
		}
	}
}

func TestItemTransparentEffect(t *testing.T) {
	if e, ok := ItemTransparentEffect(true, 3); !ok || e != d2enum.DrawEffectModulate {
		t.Errorf("torch: %v %v", e, ok)
	}

	if _, ok := ItemTransparentEffect(false, 3); ok {
		t.Error("transparent=0 must not apply")
	}

	if _, ok := ItemTransparentEffect(true, 99); ok {
		t.Error("out of range")
	}

	m := allLayersTransparent(nil)
	if len(m) != int(d2enum.CompositeTypeMax) || m[d2enum.CompositeTypeShield] != d2enum.DrawEffectPctTransparency50 {
		t.Error("all layers")
	}
}
