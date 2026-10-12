package d2ui

import "testing"

func TestColorTokenForIndex(t *testing.T) {
	tests := []struct {
		idx  int
		want ColorToken
	}{
		{0, ColorTokenWhite}, {2, ColorTokenGreen}, {3, ColorTokenBlue}, {4, ColorTokenGold},
		{5, ColorTokenGrey}, {8, ColorTokenOrange}, {9, ColorTokenYellow}, {10, ColorTokenDarkGreen},
		{-1, ColorTokenWhite}, {11, ColorTokenWhite}, {99, ColorTokenWhite},
	}

	for _, tc := range tests {
		if got := ColorTokenForIndex(tc.idx); got != tc.want {
			t.Errorf("index %d: got %q want %q", tc.idx, got, tc.want)
		}
	}
}
