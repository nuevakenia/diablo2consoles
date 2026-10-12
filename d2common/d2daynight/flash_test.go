package d2daynight

import "testing"

func TestCos512(t *testing.T) {
	if Cos512(0) != 1 || Cos512(0x80) > 1e-9 || Cos512(0x100) != -1 {
		t.Errorf("%v %v %v", Cos512(0), Cos512(0x80), Cos512(0x100))
	}
}

func TestFlashLevel8(t *testing.T) {
	if FlashLevel8Intensity(-1) != 80 || FlashLevel8Intensity(0) != 80 {
		t.Error("start is 80")
	}

	prev := 81

	for n := 0; n < FlashLevel8Ticks; n++ {
		v := FlashLevel8Intensity(n)
		if v > prev || v < 0 {
			t.Fatalf("tick %d: %d after %d", n, v, prev)
		}

		prev = v
	}

	if FlashLevel8Intensity(29) > 5 {
		t.Errorf("end %d", FlashLevel8Intensity(29))
	}
}

func TestScreenEffectEnvelope(t *testing.T) {
	e := NewScreenEffect(10, 20, 0) // fade-out defaults to 0x19

	var got []int

	for done := false; !done; {
		var v int

		v, done = e.Step(200)
		got = append(got, v)
	}

	if got[0] != 200 {
		t.Errorf("first tick %d (previous phase 0)", got[0])
	}

	if got[5] >= got[1] || got[9] >= got[5] {
		t.Errorf("fade-in should fall: %v", got[:10])
	}

	for i := 10; i < 30; i++ {
		if got[i] != 0 {
			t.Fatalf("hold tick %d = %d", i, got[i])
		}
	}

	if got[30] > got[len(got)-1] || got[len(got)-2] < 150 {
		t.Errorf("fade-out should climb back: %v", got[30:])
	}

	if len(got) != 10+20+0x19+1 {
		t.Errorf("length %d", len(got))
	}
}
