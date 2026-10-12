package d2lightmap

import "testing"

func TestClampSourceRadius(t *testing.T) {
	for in, want := range map[int]int{1: 1, 8: 8, 0x11: 0x11, 0x12: 0x12, 0x13: 0x12, 100: 0x12} {
		got, ok := ClampSourceRadius(in)
		if !ok || got != want {
			t.Errorf("%d: %d %v want %d", in, got, ok, want)
		}
	}

	for _, in := range []int{0, -1} {
		if _, ok := ClampSourceRadius(in); ok {
			t.Errorf("%d must create no source", in)
		}
	}
}

func TestUnitLightRadius(t *testing.T) {
	if UnitLightRadius(3, 5) != 5 || UnitLightRadius(6, 5) != 6 || UnitLightRadius(0, 0) != 0 {
		t.Error("larger of the two")
	}
}

func TestHoverIntensity(t *testing.T) {
	cases := map[uint8]uint8{0: 0x40, 0x10: 0x40, 0x20: 0x40, 0x21: 0x42, 0x40: 0x80, 0x7e: 0xfc, 0x7f: 0xfe, 0x80: 0xff, 0xff: 0xff}
	for in, want := range cases {
		if got := HoverIntensity(in); got != want {
			t.Errorf("%#x: %#x want %#x", in, got, want)
		}
	}
}
