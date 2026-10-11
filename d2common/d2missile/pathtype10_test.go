package d2missile

import (
	"math"
	"testing"
)

// PATH_GetDirectionIndex8 (0x67ba30) plus the first column of 0x6f2ac8: the
// eight compass cases, the collapse of the minor axis, and the zero vector.
func TestPathDirectionIndex8(t *testing.T) {
	cases := []struct {
		name         string
		dx, dy, want int
	}{
		{"+x", 10, 0, 0},
		{"+x+y", 10, 10, 1},
		{"+y", 0, 10, 2},
		{"-x+y", -10, 10, 3},
		{"-x", -10, 0, 4},
		{"-x-y", -10, -10, 5},
		{"-y", 0, -10, 6},
		{"+x-y", 10, -10, 7},
		{"nearly +x collapses the minor axis", 20, 3, 0},
		{"nearly +y with a small x", 3, 20, 2},
		{"shallow slope below 2:1 stays diagonal", 12, 7, 1},
		{"zero vector is -y", 0, 0, 6},
	}

	for _, c := range cases {
		if got := PathDirectionIndex8(100, 100, 100+c.dx, 100+c.dy); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestPathType10TurnTable(t *testing.T) {
	// entries 2..31 repeat -1, 0, +1; entries 0 and 1 are 0 (entry 1 is stale stack in the exe, taken as 0)
	counts := map[int]int{}

	for k := uint32(0); k < 32; k++ {
		counts[pathType10Turn(k)]++
	}

	if pathType10Turn(2) != -1 || pathType10Turn(3) != 0 || pathType10Turn(4) != 1 || pathType10Turn(31) != 1 {
		t.Fatalf("pattern off: %d %d %d %d", pathType10Turn(2), pathType10Turn(3), pathType10Turn(4), pathType10Turn(31))
	}

	if counts[-1] != 10 || counts[1] != 10 || counts[0] != 12 {
		t.Fatalf("distribution %v", counts)
	}
}

func TestPathType10NodesShape(t *testing.T) {
	n := PathType10Nodes(40, 40, 80, 40, 80, 77)
	if len(n) != 38+1 {
		t.Fatalf("life 77 gives 38 steps and 39 nodes, got %d", len(n))
	}

	if n[0] != [2]int{40, 40} {
		t.Fatalf("node 0 is the start, got %v", n[0])
	}

	for i := 1; i < len(n); i++ {
		dx, dy := n[i][0]-n[i-1][0], n[i][1]-n[i-1][1]
		if dx < -2 || dx > 2 || dy < -2 || dy > 2 || (dx == 0 && dy == 0) {
			t.Fatalf("step %d is %d,%d", i, dx, dy)
		}

		// the heading is +x turned by at most one eighth: x never goes backwards
		if dx < 0 {
			t.Fatalf("step %d moves away from the target: %d,%d", i, dx, dy)
		}
	}

	// the same seed repeats, another seed (destination x + index) differs
	if m := PathType10Nodes(40, 40, 80, 40, 80, 77); len(m) != len(n) || m[len(m)-1] != n[len(n)-1] {
		t.Fatal("not deterministic")
	}

	same := true

	for seed := uint32(81); seed < 90; seed++ {
		if PathType10Nodes(40, 40, 80, 40, seed, 77)[38] != n[38] {
			same = false
		}
	}

	if same {
		t.Fatal("the seed must change the path")
	}

	if PathType10Nodes(40, 40, 80, 40, 80, 1) != nil {
		t.Fatal("a life of one frame has no steps")
	}

	if got := len(PathType10Nodes(40, 40, 80, 40, 80, 500)); got != 39 {
		t.Fatalf("life is capped at 0x4d: %d nodes", got)
	}
}

func TestPathType10MissileFollowsNodesAndCapsLife(t *testing.T) {
	w := newWorld()
	s := NewSim(w, nil)
	sp := fireBolt()
	sp.Range = 200

	m, err := s.Create(CreateParams{Spec: sp, Level: 1, X: 40.5, Y: 10.5, DestX: 80, DestY: 10, PathType10: true, PathSeed: 80,
		Owner: Owner{ID: "hero", IsPlayer: true}})
	if err != nil {
		t.Fatal(err)
	}

	if !m.OnWalkPath() || m.Total != 77 {
		t.Fatalf("walk path %v, life %d (want 77)", m.OnWalkPath(), m.Total)
	}

	run(s, w, 20)

	if m.Dead() {
		t.Fatal("died early")
	}

	if m.X <= 40.5 {
		t.Fatalf("did not advance towards +x: %v", m.X)
	}

	// it never gets more than the path length from the start
	if math.Hypot(m.X-40.5, m.Y-10.5) > 38*2*math.Sqrt2 {
		t.Fatalf("left the path: %v,%v", m.X, m.Y)
	}

	run(s, w, 200)

	if !m.Dead() {
		t.Fatal("must expire at the end of the path or its life")
	}
}
