package d2groundexp

import "testing"

func TestExpiryFrame(t *testing.T) {
	const now = 1000

	cases := []struct {
		name string
		k    Kind
		want int
	}{
		{"quest never", Kind{Quest: true, Quality: 7}, 0},
		{"normal", Kind{Quality: 2}, now + NormalFrames},
		{"magic", Kind{Quality: 4}, now + MagicFrames},
		{"set", Kind{Quality: 5}, now + SpecialFrames},
		{"unique", Kind{Quality: 7}, now + SpecialFrames},
		{"tempered", Kind{Quality: 9}, now + SpecialFrames},
		{"gem", Kind{Quality: 2, SocketFiller: true}, now + MagicFrames},
		{"small gold", Kind{Gold: true, GoldAmount: 10000}, now + NormalFrames},
		{"big gold", Kind{Gold: true, GoldAmount: 10001}, now + SpecialFrames},
	}

	for _, c := range cases {
		if got := ExpiryFrame(now, c.k); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}

func TestPurgeTests(t *testing.T) {
	if !LiveExpired(100, 100) || SavedExpired(100, 100) || LiveExpired(0, 5) || SavedExpired(0, 5) {
		t.Fatal("boundary rules")
	}

	if !IsPurgeFrame(3000) || IsPurgeFrame(3001) {
		t.Fatal("period")
	}
}
