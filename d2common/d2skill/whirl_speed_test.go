package d2skill

import (
	"math"
	"testing"
)

func TestWhirlTickDistance(t *testing.T) {
	if math.Abs(WhirlTickDistance-0.375) > 1e-9 {
		t.Errorf("walk velocity 6 covers %v subtile per frame, want 0.375", WhirlTickDistance)
	}

	if WhirlStepCost(1, 0) != 1 || WhirlStepCost(0, -1) != 1 || math.Abs(WhirlStepCost(1, -1)-math.Sqrt2) > 1e-12 {
		t.Error("step cost")
	}

	// 40 orthogonal cells take 107 frames at that speed
	frames := 0

	for budget, cells := 0.0, 0; cells < 40; frames++ {
		budget += WhirlTickDistance

		for budget >= 1 && cells < 40 {
			budget--
			cells++
		}
	}

	if frames != 107 {
		t.Errorf("40 cells in %d frames", frames)
	}
}

func TestChargeVelocity(t *testing.T) {
	// run velocity 9, Param1 150, velocity percentage 100: 2.5 times the run velocity
	if got := ChargeVelocity(9, 150, 100); got != 9*256*250/100 {
		t.Errorf("velocity %d", got)
	}

	// the percentage is floored at 50
	if ChargeVelocity(9, 150, 10) != ChargeVelocity(9, 150, 50) {
		t.Error("floor")
	}

	if got := ChargeSubtilesPerTick(ChargeVelocity(9, 150, 100)); math.Abs(got-1.40625) > 1e-9 {
		t.Errorf("%v subtiles per frame", got)
	}
}
