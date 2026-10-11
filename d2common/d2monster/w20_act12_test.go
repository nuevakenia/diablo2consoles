package d2monster

import "testing"

type shamanScanWorld struct {
	*fakeBaal
	n      int
	radius int
}

func (w *shamanScanWorld) ShamanCorpseScan(_ *Brain, r int) (Target, int) {
	w.radius = r

	return Target{ID: 9, X: 110, Y: 100}, w.n
}

// The exe scan hands the squared aip4 to the host and raises only when its
// count is non-zero.
func TestFallenShamanUsesExeScan(t *testing.T) {
	p := skills(profile("FallenShaman", 100, 100, 0, 24, 15), "Resurrect", "Shaman Fire")

	w := &shamanScanWorld{fakeBaal: &fakeBaal{fakeWorld: newFake(8, false)}, n: 2}
	Tick(w, brainAt(p))

	if w.radius != 24*24 || w.last() != "cast0" {
		t.Fatalf("radius %d log %v", w.radius, w.log)
	}

	w2 := &shamanScanWorld{fakeBaal: &fakeBaal{fakeWorld: newFake(8, false)}}
	Tick(w2, brainAt(p))

	if w2.last() != "cast1" {
		t.Fatalf("no corpse must fall through to Shaman Fire, log %v", w2.log)
	}
}
