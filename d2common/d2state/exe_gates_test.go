package d2state

import "testing"

func TestStunFor(t *testing.T) {
	cases := []struct {
		name   string
		frames int
		tg     Target
		roll   int
		want   int
		ok     bool
	}{
		{"player cap", 400, Target{}, 99, 250, true},
		{"player short", 30, Target{}, 99, 30, true},
		{"zero", 0, Target{Monster: true}, 99, 0, false},
		{"plain monster cap", 300, Target{Monster: true}, 99, 250, true},
		{"boss immune", 30, Target{Monster: true, Boss: true}, 99, 0, false},
		{"immobile immune", 30, Target{Monster: true, ZeroSpeed: true}, 99, 0, false},
		{"merc long cut", 40, Target{Monster: true, Merc: true}, 99, 13, true},
		{"merc 12 kept", 12, Target{Monster: true, Merc: true}, 99, 12, true},
		{"flag8 roll skips", 30, Target{Monster: true, DataFlag8: true}, 89, 0, false},
		{"flag8 roll lands", 30, Target{Monster: true, DataFlag8: true}, 90, 30, true},
		{"flag8 does not touch players", 30, Target{DataFlag8: true}, 0, 30, true},
	}
	for _, c := range cases {
		got, ok := StunFor(c.frames, c.tg, c.roll)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got %d,%v want %d,%v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestRouteFreezeFor(t *testing.T) {
	cases := []struct {
		tg   Target
		want FreezeRoute
	}{
		{Target{}, RouteChill},
		{Target{Monster: true}, RouteFreeze},
		{Target{Monster: true, Uninterruptable: true}, RouteNone},
		{Target{Monster: true, Boss: true}, RouteChill},
		{Target{Monster: true, DataFlag8: true}, RouteChill},
		{Target{Monster: true, Merc: true}, RouteChill},
	}
	for i, c := range cases {
		if got := RouteFreezeFor(c.tg); got != c.want {
			t.Errorf("case %d: got %d want %d", i, got, c.want)
		}
	}
}

func TestShatterVisual(t *testing.T) {
	if !ShatterVisual(19, true, false) || ShatterVisual(20, true, false) || ShatterVisual(0, false, false) || ShatterVisual(0, true, true) {
		t.Fatal("shatter visual gate")
	}
}

func TestApplyHitTargets(t *testing.T) {
	// boss: stun refused, freeze becomes chill with the chill divisor
	s := New()
	boss := &Target{Monster: true, Boss: true}
	got := s.ApplyHit(0, Hit{StunLen: 50, FreezeLen: 100, Target: boss, HasColdEffect: true, ColdEffect: -50, ChillDiv: 4, FreezeDiv: 2})
	if len(got) != 1 || got[0] != Chill {
		t.Fatalf("boss applied %v", got)
	}

	if in := s.Get(0, Chill); in == nil || in.Until != 25 {
		t.Fatalf("boss chill %+v (want end 25 = 100/ChillDiv)", in)
	}

	if s.Active(0, Stun) || s.Active(0, Freeze) {
		t.Fatal("boss stunned or frozen")
	}

	// player: freeze length goes to chill at -50, no divisor even if supplied
	p := New()
	p.ApplyHit(0, Hit{FreezeLen: 100, Target: &Target{}, HasColdEffect: true, ColdEffect: 0, ChillDiv: 4})

	if in := p.Get(0, Chill); in == nil || in.Until != 100 || p.Stat(0, "velocitypercent") != -50 {
		t.Fatalf("player chill %+v", in)
	}

	// chill length floor of 1 after the divisor
	m := New()
	m.ApplyHit(5, Hit{ColdLen: 3, Target: &Target{Monster: true}, HasColdEffect: true, ColdEffect: -50, ChillDiv: 9})

	if in := m.Get(5, Chill); in == nil || in.Until != 6 {
		t.Fatalf("min length chill %+v", in)
	}

	// shatter visual roll
	r := New()
	r.ApplyHit(0, Hit{ColdLen: 10, Target: &Target{Monster: true}, HasColdEffect: true, ColdEffect: -50, Roll100: func() int { return 5 }})

	if !r.ShatterVisual() {
		t.Fatal("expected shatter visual on")
	}

	r.ApplyHit(1, Hit{ColdLen: 10, Target: &Target{Monster: true}, HasColdEffect: true, ColdEffect: -50, Roll100: func() int { return 50 }})

	if r.ShatterVisual() {
		t.Fatal("expected shatter visual off")
	}
}
