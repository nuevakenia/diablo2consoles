package d2missile

import "testing"

func vineSpecs() specs {
	return specs{
		"plague vines": {ID: 474, Name: "plague vines", SrvDoFunc: 26, Vel: 7, MaxVel: 7, Range: 20,
			Param: [5]int{9}, SubMissile: [3]string{"plague vines trail"}, CanSlow: true},
		"plague vines trail": {ID: 475, Name: "plague vines trail", SrvDoFunc: 1, SrvHitFunc: 50, Range: 100, LevRange: 25,
			CollideType: 3, LastCollide: true, Size: 3, AlwaysExplode: true, SHitPar: [3]int{15}},
		"orphan":         {ID: 1, Name: "orphan", SrvDoFunc: 26, Vel: 7, Range: 20},
		"recycler delay": {ID: 498, Name: "recycler delay", SrvDoFunc: 29, Range: 47, Param: [5]int{45}},
		"vine recycler":  {ID: 540, Name: "vine recycler", SrvDoFunc: 33, Range: 47, Param: [5]int{45}},
	}
}

// SrvDoFunc 26 (0x5ad4e0): the trail is dropped whenever the elapsed life is a multiple of Param1 (9), the first frame
// included, on the missile's own subtile and stationary; the owner, skill and level are the parent's.
func TestVineTrailDropsEveryParam1Frames(t *testing.T) {
	tab := vineSpecs()
	w := newWorld()
	s := NewSim(w, tab)
	evs := collect(s)

	_, err := s.Create(CreateParams{Spec: tab["plague vines"], Owner: ownerWith(0), SkillID: 222, Level: 7,
		X: 40.5, Y: 0.5, DestX: 50, DestY: 0.5})
	if err != nil {
		t.Fatal(err)
	}

	stepN(s, w, 19)

	trails := creates(*evs, "plague vines trail")
	// life 20: elapsed 0, 9 and 18 are multiples of 9 (the move runs after the check)
	if len(trails) != 3 {
		t.Fatalf("%d trails, want 3", len(trails))
	}

	for i, tr := range trails {
		if tr.Velocity != 0 || tr.SkillID != 222 || tr.Level != 7 {
			t.Errorf("trail %d velocity %d skill %d level %d", i, tr.Velocity, tr.SkillID, tr.Level)
		}

		if i > 0 && tr.X <= trails[i-1].X {
			t.Errorf("trail %d did not follow the vine: %v after %v", i, tr.X, trails[i-1].X)
		}

		if tr.Total != 100+25*7 {
			t.Errorf("trail %d lives %d frames", i, tr.Total)
		}
	}
}

func TestVineTrailWithoutSubMissileVanishes(t *testing.T) {
	tab := vineSpecs()
	w := newWorld()
	s := NewSim(w, tab)
	evs := collect(s)

	m, _ := s.Create(CreateParams{Spec: tab["orphan"], Owner: ownerWith(0), SkillID: 1, Level: 1, X: 40.5, Y: 0.5, DestX: 50, DestY: 0.5})
	stepN(s, w, 1)

	if !m.Dead() || count(*evs, EventVanish) != 1 {
		t.Fatalf("dead %v, vanish events %d", m.Dead(), count(*evs, EventVanish))
	}
}

// SrvHitFunc 50 (0x5aa340): damage (2) while the elapsed life is below the total minus sHitPar1, nothing (0) in the
// last sHitPar1 frames, destroy (1) with no target.
func TestVineTrailHitWindow(t *testing.T) {
	tab := vineSpecs()
	w := newWorld()
	s := NewSim(w, tab)

	m, _ := s.Create(CreateParams{Spec: tab["plague vines trail"], Owner: ownerWith(0), SkillID: 222, Level: 1,
		X: 40.5, Y: 0.5, DestX: 40.5, DestY: 0.5, Stationary: true})
	tg := newTarget("mob", 40, 0)

	cases := []struct {
		elapsed int
		t       Target
		want    int
	}{
		{0, tg, resDamage},
		{109, tg, resDamage},
		{110, tg, 0},
		{124, tg, 0},
		{50, nil, resKill},
	}

	for _, c := range cases {
		m.Life = m.Total - c.elapsed

		if got := vineTrailHit(m, c.t); got != c.want {
			t.Errorf("elapsed %d: hit function returned %d, want %d", c.elapsed, got, c.want)
		}
	}

	if m.Total != 125 {
		t.Fatalf("total life %d", m.Total)
	}
}

// SrvDoFunc 29 / 33 (0x5ad910, 0x5ada80): the owner gets the percent back exactly when the elapsed life equals Param1.
func TestRecyclerDelayRestoresOnce(t *testing.T) {
	for _, tc := range []struct {
		name string
		mana bool
	}{{"recycler delay", false}, {"vine recycler", true}} {
		tab := vineSpecs()
		w := newWorld()
		s := NewSim(w, tab)

		type call struct {
			mana bool
			pct  int
			at   int
		}

		var calls []call

		own := ownerWith(0)
		own.Restore = func(mana bool, pct int) bool {
			calls = append(calls, call{mana, pct, w.frame})

			return true
		}

		_, _ = s.Create(CreateParams{Spec: tab[tc.name], Owner: own, SkillID: 307, Level: 3, X: 10.5, Y: 0.5, DestX: 10.5, DestY: 0.5,
			Stationary: true, RestorePct: 9})
		stepN(s, w, 46)

		if len(calls) != 1 || calls[0].mana != tc.mana || calls[0].pct != 9 {
			t.Fatalf("%s: restore calls %+v", tc.name, calls)
		}

		// Elapsed life is 45 on the 46th step, before the move.
		if calls[0].at != 46 {
			t.Errorf("%s: restored on frame %d, want 46", tc.name, calls[0].at)
		}
	}
}

func TestRecyclerDelaySkipsADeadOwner(t *testing.T) {
	tab := vineSpecs()
	w := newWorld()
	s := NewSim(w, tab)
	called := false

	own := ownerWith(0)
	own.Gone = func() bool { return true }
	own.Restore = func(bool, int) bool { called = true; return true }

	m, _ := s.Create(CreateParams{Spec: tab["recycler delay"], Owner: own, SkillID: 307, Level: 1, X: 10.5, Y: 0.5, DestX: 10.5, DestY: 0.5,
		Stationary: true, RestorePct: 9})
	stepN(s, w, 60)

	if called {
		t.Fatal("a dead owner must not be healed")
	}

	if !m.Dead() {
		t.Fatal("the missile still runs out its life")
	}
}

func TestVineFuncsAreListed(t *testing.T) {
	for _, id := range []int{26, 29, 33} {
		if !DoFuncModelled(id) {
			t.Errorf("do function %d not listed", id)
		}
	}

	if !HitFuncModelled(50) {
		t.Error("hit function 50 not listed")
	}
}
