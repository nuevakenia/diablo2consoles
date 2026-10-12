package d2player

import "testing"

func TestIgnoredWhileCasting(t *testing.T) {
	tests := []struct {
		act  WorldAction
		want bool
	}{
		{WorldNone, true},
		{WorldMove, true},   // a walk order never breaks a cast (exe: PLRMODE_CanInterruptCurrentMode)
		{WorldAttack, true}, // the melee swing path is not modelled as an interrupt
		{WorldCastLeft, false},
		{WorldCastRight, false},
		{WorldStandStill, false},
	}

	for _, tt := range tests {
		if got := IgnoredWhileCasting(tt.act); got != tt.want {
			t.Errorf("IgnoredWhileCasting(%v) = %v, want %v", tt.act, got, tt.want)
		}
	}
}
