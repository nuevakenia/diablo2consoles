package d2client

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
)

func TestShotWobbleNodes(t *testing.T) {
	s := clientShot{from: d2skill.SubPoint{X: 10, Y: 10}, to: d2skill.SubPoint{X: 30, Y: 10}, pathType10: true, pathSeed: 30}
	a := shotWobbleNodes(s, 0)
	b := shotWobbleNodes(s, 1000)

	if len(a) != 0x4d/2+1 || len(a) != len(b) {
		t.Fatalf("nodes %d %d", len(a), len(b))
	}

	if a[0] != [2]int{10, 10} {
		t.Fatalf("start %v", a[0])
	}

	s.pathSeed = 31
	c := shotWobbleNodes(s, 0)
	same := true

	for i := range a {
		if a[i] != c[i] {
			same = false
		}
	}

	if same {
		t.Fatal("seed does not change the path")
	}
}
