package d2lightmap

import "testing"

func TestUnitRadiusFromStat(t *testing.T) {
	tests := []struct {
		base, stat, want int
		apply            bool
	}{
		{HeroBaseRadius, 0, 13, true},
		{HeroBaseRadius, 1, 14, true},
		{HeroBaseRadius, -3, 10, true},
		{HeroBaseRadius, -13, 0, false}, // old radius stays
		{HeroBaseRadius, -20, 0, false},
		{HeroBaseRadius, 4, 17, true},
		{HeroBaseRadius, 5, 18, true}, // 0x12 cap
		{HeroBaseRadius, 40, 18, true},
		{0, 2, 2, true}, // monster without own light
	}
	for _, tc := range tests {
		got, ok := UnitRadiusFromStat(tc.base, tc.stat)
		if ok != tc.apply || (ok && got != tc.want) {
			t.Errorf("base %d stat %d: got %d,%v want %d,%v", tc.base, tc.stat, got, ok, tc.want, tc.apply)
		}
	}
}

func TestDecodeLightColor(t *testing.T) {
	tests := []struct {
		in      int32
		r, g, b uint8
	}{
		{0, 255, 255, 255},
		{0x00ff8040, 0xff, 0x80, 0x40},
		{0x000102, 0, 1, 2},
	}
	for _, tc := range tests {
		r, g, b := DecodeLightColor(tc.in)
		if r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("%#x: got %d,%d,%d", tc.in, r, g, b)
		}
	}
}

func TestMissileFlickerRadius(t *testing.T) {
	calls := 0
	roll := func(n int) int { calls++; return n - 1 }

	if _, ok := MissileFlickerRadius(0x100, 5, 3, 2, roll); ok || calls != 0 {
		t.Error("flag 0x100 must skip without drawing")
	}

	if _, ok := MissileFlickerRadius(0, 2, 3, 2, roll); ok || calls != 0 {
		t.Error("radius below Light must skip without drawing")
	}

	if got, ok := MissileFlickerRadius(0, 3, 3, 2, roll); !ok || got != 4 || calls != 1 {
		t.Errorf("got %d,%v calls %d", got, ok, calls)
	}

	if got, ok := MissileFlickerRadius(0, 3, 3, 0, roll); !ok || got != 3 || calls != 1 {
		t.Errorf("flicker 0: got %d,%v calls %d", got, ok, calls)
	}

	if got, ok := MissileFlickerRadius(0, 17, 17, 9, roll); !ok || got != 18 {
		t.Errorf("cap: got %d", got)
	}
}

func TestSetRadiusNow(t *testing.T) {
	s := NewSource(13, 255, 255, 255, 255)
	s.SetRadiusNow(5)

	if s.Radius != 40 || s.Target != 40 {
		t.Fatalf("%d %d", s.Radius, s.Target)
	}

	s.SetRadiusNow(0)

	if s.Radius != 40 {
		t.Error("0 must be ignored")
	}

	s.SetRadiusNow(99)

	if s.Radius != 18*8 {
		t.Error("cap")
	}
}
