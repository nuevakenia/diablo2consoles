package d2skill

import "testing"

func TestBoneArmorAbsorb(t *testing.T) {
	for _, c := range []struct {
		name                      string
		phys, pool, max           int
		wantPhys, wantPool, wantP int
		ended                     bool
	}{
		{"pool bigger", 100 << 8, 300 << 8, 400 << 8, 0, 200 << 8, 50, false},
		{"pool smaller", 300 << 8, 100 << 8, 400 << 8, 200 << 8, 0, 0, true},
		{"pool equal ends", 100 << 8, 100 << 8, 400 << 8, 0, 0, 0, true},
		{"no physical", 0, 100 << 8, 100 << 8, 0, 100 << 8, 100, false},
		{"no max: untouched", 50, 100, 0, 50, 0, 0, false},
	} {
		p, pl, ended, pct := BoneArmorAbsorb(c.phys, c.pool, c.max)
		if p != c.wantPhys || pl != c.wantPool || pct != c.wantP || ended != c.ended {
			t.Errorf("%s: got %d %d %v %d", c.name, p, pl, ended, pct)
		}
	}
}
