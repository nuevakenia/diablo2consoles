package d2maprenderer

import "testing"

func TestTileAnimFrame(t *testing.T) {
	tests := []struct{ ticks, speed, frames, want int }{
		{0, 0, 10, 0},
		{3, 0, 10, 0}, // 240/256
		{4, 0, 10, 1}, // 320/256
		{32, 0, 10, 0},
		{33, 0, 10, 0},
		{8, 0x100, 5, 3},
		{5, 0x100, 5, 0},
		{7, 0, 0, 0},
	}

	for _, tc := range tests {
		if got := tileAnimFrame(tc.ticks, tc.speed, tc.frames); got != tc.want {
			t.Errorf("%+v: got %d", tc, got)
		}
	}
}
