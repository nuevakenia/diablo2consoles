package d2skill

import "testing"

func TestInfernoLife(t *testing.T) {
	for in, want := range map[int]int{-3: 0, 0: 0, 1: 1, 2: 2, 19: 19, 254: 254, 255: 255, 300: 255} {
		if got := InfernoLife(in); got != want {
			t.Errorf("InfernoLife(%d) = %d, want %d", in, got, want)
		}
	}
}

// The real "mon inferno sentry" row: calc1 = ln34/2 + skill('Wake of Fire Sentry'.blvl); at level 6 with 4 points in
// Wake of Fire. Only checked to be within the clamp so the flame gets a life.
func TestInfernoSentryRowHasALife(t *testing.T) {
	reg := loadRealRegistry(t)

	ms := reg.ByName("mon inferno sentry")
	if ms == nil || ms.SrvDoFunc != 95 || ms.SrvMissileA == "" {
		t.Fatalf("skill %+v", ms)
	}

	u := newUnit(reg, map[string]int{"Wake of Fire Sentry": 4})
	u.base = u.levels

	life := InfernoLife(NewEnv(ms, 6, u, reg).eval(ms.Calc[1]))
	if life < 1 || life > 255 {
		t.Errorf("life %d", life)
	}

	bs := reg.ByName("BoltSentry")
	if bs == nil || bs.SrvDoFunc != 17 {
		t.Fatalf("BoltSentry %+v", bs)
	}

	// calc1 = ln12 + skill('Shock Field'.blvl)/3 = 5 + 6/3
	u2 := newUnit(reg, map[string]int{"Shock Field": 6})
	u2.base = u2.levels

	if n := NewEnv(bs, 3, u2, reg).eval(bs.Calc[1]); n != 7 {
		t.Errorf("bolts = %d, want 7", n)
	}
}
