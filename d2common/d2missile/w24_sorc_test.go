package d2missile

import "testing"

// Wave 24 sorceress re-audit (VERIFIED against Game.exe 1.14b): a fire that stands still and has neither
// LastCollide nor NextHit (firewall, meteorfire, armageddonfire) hits a unit standing in it on every frame.
// MISSILE_SrvDoStandardMove 0x5abd00 tests the units under the missile each tick and MISSILE_ProcessHitOrExpire
// 0x5aba10 only dedupes through LastCollide (0x64b700) and the NextHit state 0x56.
func TestStandingFireHitsEveryFrame(t *testing.T) {
	hero := Owner{ID: "hero", IsPlayer: true, Roller: fakeRoller{}}

	for _, tc := range []struct {
		name         string
		lastCollide  bool
		nextHit      bool
		wantHits     int
		wantHitsDesc string
	}{
		{"firewall", false, false, 20, "one per frame"},
		{"lastcollide", true, false, 1, "one per target"},
		{"nexthit", false, true, 4, "one per NextDelay (5)"},
	} {
		w := newWorld()
		z := newTarget("z", 10, 0)
		w.targets = append(w.targets, z)
		s := NewSim(w, nil)

		fire := &Spec{ID: 1, Name: "firewall", SrvDoFunc: 5, Range: 90, CollideType: 3, LastCollide: tc.lastCollide,
			NextHit: tc.nextHit, NextDelay: 5}
		_, err := s.Create(CreateParams{Spec: fire, Level: 1, X: 10, Y: 0, DestX: 10, DestY: 0, Owner: hero,
			Stationary: true, Damage: DamageDesc{PhysMin: 1000, PhysMax: 1001}})
		if err != nil {
			t.Fatal(err)
		}

		hits := 0

		s.OnEvent = func(e Event) {
			if e.Kind == EventHit {
				hits++
			}
		}

		for i := 0; i < 20; i++ {
			w.frame++
			s.Step()
		}

		if hits != tc.wantHits {
			t.Errorf("%s: %d hits in 20 frames, want %d (%s)", tc.name, hits, tc.wantHits, tc.wantHitsDesc)
		}
	}
}

// SrvDoFunc 15 (0x5acba0): a Frozen Orb whose owner is gone, or without SubMissile1, ends at once (return 2),
// before the period test; with its owner it fires one bolt per Param1 frames.
func TestFrozenOrbNeedsOwnerAndSubMissile(t *testing.T) {
	bolt := &Spec{ID: 2, Name: "frozenorbbolt", Range: 25, CollideType: 3, Vel: 18}
	orb := &Spec{ID: 1, Name: "frozenorb", SrvDoFunc: 15, Range: 30, Vel: 10, Param: [5]int{1, 19}, SubMissile: [3]string{"frozenorbbolt"}}
	tbl := specs{"frozenorbbolt": bolt}

	gone := false
	hero := Owner{ID: "hero", IsPlayer: true, Roller: fakeRoller{}, Gone: func() bool { return gone }}

	w := newWorld()
	s := NewSim(w, tbl)
	created := 0

	s.OnEvent = func(e Event) {
		if e.Kind == EventCreate && e.Missile.Spec.Name == "frozenorbbolt" {
			created++
		}
	}

	m, err := s.Create(CreateParams{Spec: orb, Level: 1, X: 0, Y: 0, DestX: 30, DestY: 0, Owner: hero})
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		w.frame++
		s.Step()
	}

	if created != 5 {
		t.Errorf("Param1 1 must fire a bolt per frame: %d in 5 frames", created)
	}

	gone = true
	w.frame++
	s.Step()

	if !m.Dead() || created != 5 {
		t.Errorf("owner gone: orb dead=%v, bolts %d (want it ended with no new bolt)", m.Dead(), created)
	}

	// no SubMissile1: the orb ends on its first tick
	w2 := newWorld()
	s2 := NewSim(w2, tbl)
	orb2 := *orb
	orb2.SubMissile = [3]string{}
	m2, _ := s2.Create(CreateParams{Spec: &orb2, Level: 1, DestX: 30, Owner: Owner{ID: "hero", IsPlayer: true, Roller: fakeRoller{}}})
	w2.frame++
	s2.Step()

	if !m2.Dead() {
		t.Error("orb without SubMissile1 must end")
	}
}
