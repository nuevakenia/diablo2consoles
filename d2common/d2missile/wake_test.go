package d2missile

import "testing"

func TestWakeVectorIsAQuarterTurn(t *testing.T) {
	// sentry at (10,10) firing at (14,12): direction (4,2); field 0x28 = -dy, 0x2c = dx
	a, b := WakeVector(10, 10, 14, 12)
	if int32(a) != -2 || int32(b) != 4 {
		t.Errorf("vector (%d,%d), want (-2,4)", int32(a), int32(b))
	}
}

func TestWakeMakerDropsTwoFlamesPerNewCell(t *testing.T) {
	sub := &Spec{ID: 133, Name: "wakeofdestruction", SrvDoFunc: 1, Vel: 8, MaxVel: 8, Range: 20, CollideType: 3, Size: 1}
	mk := &Spec{ID: 132, Name: "wakemaker", SrvDoFunc: 31, Vel: 10, MaxVel: 10, Range: 25, CollideType: 0, Size: 2,
		SubMissile: [3]string{"wakeofdestruction"}}
	w := newWorld()
	s := NewSim(w, specs{"wakeofdestruction": sub})
	evs := collect(s)

	owner := Owner{ID: "sentry", Roller: fakeRoller{0}}
	a, b := WakeVector(10, 10, 20, 10)

	_, err := s.Create(CreateParams{Spec: mk, Level: 1, X: 10.5, Y: 10.5, DestX: 20.5, DestY: 10.5, Owner: owner,
		Data28: a, Data2C: b})
	if err != nil {
		t.Fatal(err)
	}

	stepN(s, w, 12)

	var up, down int

	for _, e := range *evs {
		if e.Kind == EventCreate && e.Missile.Spec.Name == "wakeofdestruction" {
			// the line runs along x, so the flames fly along +y and -y
			switch {
			case e.Missile.destY > e.Missile.Y:
				down++
			case e.Missile.destY < e.Missile.Y:
				up++
			}
		}
	}

	if up == 0 || up != down {
		t.Errorf("flames up %d down %d, want equal and non-zero", up, down)
	}
}

func TestWakeMakerWithoutOwnerEnds(t *testing.T) {
	sub := &Spec{ID: 133, Name: "wakeofdestruction", SrvDoFunc: 1, Vel: 8, MaxVel: 8, Range: 20, Size: 1}
	mk := &Spec{ID: 132, Name: "wakemaker", SrvDoFunc: 31, Vel: 10, MaxVel: 10, Range: 25, Size: 2,
		SubMissile: [3]string{"wakeofdestruction"}}
	w := newWorld()
	s := NewSim(w, specs{"wakeofdestruction": sub})

	m, _ := s.Create(CreateParams{Spec: mk, Level: 1, X: 10.5, Y: 10.5, DestX: 20.5, DestY: 10.5})
	stepN(s, w, 2)

	if !m.Dead() {
		t.Error("a wake maker without an owner must end")
	}
}
