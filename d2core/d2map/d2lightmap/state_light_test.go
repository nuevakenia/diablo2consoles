package d2lightmap

import "testing"

func TestNewStateLight(t *testing.T) {
	if _, ok := NewStateLight(0, 3, 1, 2, 3); ok {
		t.Error("radius 0 has no light")
	}

	if _, ok := NewStateLight(5, 0, 1, 2, 3); ok {
		t.Error("init radius 0 creates nothing")
	}

	s, ok := NewStateLight(6, 2, 10, 20, 30)
	if !ok || s.Radius != 16 || s.Target != 48 || s.Intensity != 0xff || s.G != 20 {
		t.Fatalf("bad source %+v", s)
	}

	// grows one subtile (8 units) per tick until the target
	for i := 0; i < 4; i++ {
		s.Step()
	}

	if s.Radius != 48 {
		t.Errorf("radius after 4 ticks %d", s.Radius)
	}

	s.Step()

	if s.Radius != 48 {
		t.Error("must stop at the target")
	}

	// creation clamp 0x12
	big, _ := NewStateLight(40, 30, 0, 0, 0)
	if big.Radius != 0x12*8 || big.Target != 0x12*8 {
		t.Errorf("clamp: %+v", big)
	}
}
