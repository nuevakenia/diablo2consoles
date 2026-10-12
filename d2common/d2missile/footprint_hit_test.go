package d2missile

import "testing"

type sizedTarget struct {
	*fakeTarget
	size int
}

func (t *sizedTarget) Size() int                  { return t.size }
func (t *sizedTarget) SubPos() (float64, float64) { return float64(t.x) + 0.5, float64(t.y) + 0.5 }

// nearWorld serves the footprint lookup the way the engine does: every unit within r cells, plus the old
// per-cell lookup with a radius 1 reach.
type nearWorld struct{ *fakeWorld }

func (w *nearWorld) Targets(x, y int) []Target {
	var out []Target

	for _, t := range w.targets {
		if t.alive && absInt(t.x-x) <= 1 && absInt(t.y-y) <= 1 {
			out = append(out, &sizedTarget{t, 1})
		}
	}

	return out
}

func (w *nearWorld) TargetsWithin(x, y, r int) []Target {
	var out []Target

	for _, t := range w.targets {
		if t.alive && absInt(t.x-x) <= r && absInt(t.y-y) <= r {
			out = append(out, &sizedTarget{t, 1})
		}
	}

	return out
}

// Size 2 missiles hit a size 1 unit on the plus around the node only (the old lookup took the whole 3x3), size 3
// missiles take the 3x3 block, size 1 missiles keep the lookup.
func TestHitCandidatesFootprint(t *testing.T) {
	w := &nearWorld{newWorld()}
	w.targets = []*fakeTarget{newTarget("edge", 11, 10), newTarget("diag", 11, 11), newTarget("far", 12, 10)}
	s := NewSim(w, specs{})

	ids := func(size int) map[string]bool {
		got := map[string]bool{}

		for _, c := range s.hitCandidates(&Missile{Spec: &Spec{Size: size}}, 10, 10) {
			got[c.ID()] = true
		}

		return got
	}

	if g := ids(2); !g["edge"] || g["diag"] || g["far"] {
		t.Errorf("size 2: %v, want only edge", g)
	}

	if g := ids(3); !g["edge"] || !g["diag"] || g["far"] {
		t.Errorf("size 3: %v, want edge and diag", g)
	}

	if g := ids(1); !g["edge"] || !g["diag"] || g["far"] {
		t.Errorf("size 1 keeps the engine lookup: %v", g)
	}
}
