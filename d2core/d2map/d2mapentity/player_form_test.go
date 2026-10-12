package d2mapentity

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
)

// The form composites are monster composites: they have no town, throw or kick modes.
func TestFormModeFoldsPlayerOnlyModes(t *testing.T) {
	cases := map[d2enum.PlayerAnimationMode]d2enum.PlayerAnimationMode{
		d2enum.PlayerAnimationModeTownNeutral: d2enum.PlayerAnimationModeNeutral,
		d2enum.PlayerAnimationModeTownWalk:    d2enum.PlayerAnimationModeWalk,
		d2enum.PlayerAnimationModeThrow:       d2enum.PlayerAnimationModeAttack1,
		d2enum.PlayerAnimationModeKick:        d2enum.PlayerAnimationModeAttack1,
		d2enum.PlayerAnimationModeSkill2:      d2enum.PlayerAnimationModeAttack2,
		d2enum.PlayerAnimationModeRun:         d2enum.PlayerAnimationModeRun,
		d2enum.PlayerAnimationModeBlock:       d2enum.PlayerAnimationModeBlock,
		d2enum.PlayerAnimationModeDeath:       d2enum.PlayerAnimationModeDeath,
		d2enum.PlayerAnimationModeDead:        d2enum.PlayerAnimationModeDead,
	}

	for in, want := range cases {
		if got := FormMode(in); got != want {
			t.Errorf("FormMode(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestModeForOnlyInForm(t *testing.T) {
	p := &Player{}
	if p.modeFor(d2enum.PlayerAnimationModeTownWalk) != d2enum.PlayerAnimationModeTownWalk || p.InForm() {
		t.Fatal("human shape must not remap modes")
	}

	p.formToken = "40"
	if p.modeFor(d2enum.PlayerAnimationModeTownWalk) != d2enum.PlayerAnimationModeWalk || !p.InForm() {
		t.Fatal("a form must remap town modes")
	}
}
