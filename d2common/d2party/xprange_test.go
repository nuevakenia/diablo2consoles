package d2party

import "testing"

func partyOf(t *testing.T) *Roster {
	t.Helper()

	r := afield()
	if err := r.Invite("a", "b"); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Accept("b"); err != nil {
		t.Fatal(err)
	}

	return r
}

func ids(s []XPShare) map[string]bool {
	m := map[string]bool{}
	for _, x := range s {
		m[x.ID] = true
	}

	return m
}

// Kill experience goes to the party members within 80 subtiles (squared
// distance 0x1900, inclusive) that are not dead bodies.
func TestKillXPRange(t *testing.T) {
	tests := []struct {
		name    string
		bx, by  int
		body    bool
		wantB   bool
		unknown bool
	}{
		{name: "next to the killer", bx: 10, by: 10, wantB: true},
		{name: "exactly at the limit", bx: 80, by: 0, wantB: true},
		{name: "one subtile beyond", bx: 81, by: 0},
		{name: "diagonal inside (56^2*2 = 6272)", bx: 56, by: 56, wantB: true},
		{name: "diagonal outside (57^2*2 = 6498)", bx: 57, by: 57},
		{name: "dead body", bx: 5, by: 5, body: true},
		{name: "position unknown: the area decides", unknown: true, wantB: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := partyOf(t)
			r.SetPos("a", 0, 0)

			if !tc.unknown {
				r.SetPos("b", tc.bx, tc.by)
			}

			r.SetPlayerBody("b", tc.body)

			got := ids(r.ShareKillXP("a", 1000, 30, 99))
			if got["b"] != tc.wantB || !got["a"] {
				t.Fatalf("recipients %v, b wanted %v", got, tc.wantB)
			}
		})
	}
}
