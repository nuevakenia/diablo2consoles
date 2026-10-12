package d2skill

import "testing"

func TestInfernoFlameLife(t *testing.T) {
	if InfernoFlameLife(0) != 1 || InfernoFlameLife(-3) != 1 || InfernoFlameLife(12) != 12 {
		t.Fatal("life is calc1, at least one frame")
	}

	reg := loadRealRegistry(t)
	sk := reg.ByName("Inferno")

	// ln12 = Param1 20 + 3 per level, halved
	for lvl, want := range map[int]int{1: 10, 20: 38} {
		if got := InfernoFlameLife(realEnv(reg, "Inferno", lvl, nil).eval(sk.Calc[1])); got != want {
			t.Errorf("level %d: %d, want %d", lvl, got, want)
		}
	}
}
