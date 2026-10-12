package d2player

import "testing"

func TestNextNPCMenuRow(t *testing.T) {
	cases := []struct {
		name      string
		cur, n, d int
		want      int
	}{
		{"down from none", -1, 4, 1, 0},
		{"up from none", -1, 4, -1, 3},
		{"down", 1, 4, 1, 2},
		{"wrap down", 3, 4, 1, 0},
		{"wrap up", 0, 4, -1, 3},
		{"stale focus", 9, 4, 1, 0},
		{"empty", 0, 0, 1, -1},
	}

	for _, c := range cases {
		if got := NextNPCMenuRow(c.cur, c.n, c.d); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}
