package drlgpop

import "testing"

// place_tentacle_ns / place_tentacle_ew (monplace rows 10 and 11) start from class 0x105 with a depth by level and the
// flags 4 / 0 (FUN_0054c470, byte table 0x54c9a0).
func TestResolveTentacle(t *testing.T) {
	nm := realNames(t)
	base := nm.MonstatsCount + nm.SuperCount

	for _, c := range []struct{ level, depth int }{
		{0x4b, 0}, {0x4c, 1}, {0x4d, 1}, {0x4e, 1}, {0x4f, 0}, {0x56, 0}, {0x5b, 0}, {0x5c, 2}, {0x5d, 2}, {0x5e, 0}, {1, 0},
	} {
		ns := ResolveMonster(base+10, &Ctx{LevelID: c.level}, nm)
		ew := ResolveMonster(base+11, &Ctx{LevelID: c.level}, nm)

		if ns.Kind != MonTentacle || ns.Class != 0x105 || ns.Depth != c.depth || ns.Mode != 4 {
			t.Errorf("level %#x ns: %+v", c.level, ns)
		}

		if ew.Kind != MonTentacle || ew.Class != 0x105 || ew.Depth != c.depth || ew.Mode != 0 {
			t.Errorf("level %#x ew: %+v", c.level, ew)
		}
	}
}
