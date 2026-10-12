package d2missile

import "testing"

func TestRingOffsets(t *testing.T) {
	if got := RingOffsets(1); len(got) != 1 || got[0] != [2]int{14, -14} {
		t.Errorf("n=1: %v", got)
	}

	// n=3: first point, one pair of ring 0 (sign pair (-1,-1): (-18,-8), (-8,-18)),
	// then the seven cardinal points of the seven rings (the counter went
	// negative, but the loop only ends after ring 7).
	got := RingOffsets(3)
	if len(got) != 1+2+7 {
		t.Fatalf("n=3: %d points %v", len(got), got)
	}

	if got[1] != [2]int{-18, -8} || got[2] != [2]int{-8, -18} || got[3] != [2]int{20, 0} {
		t.Errorf("n=3 order: %v", got[:4])
	}

	if RingOffsets(41) != nil {
		t.Error("n=41 should be refused ((n-1)/5 > 7)")
	}

	// n=40: four full rings (9 each), two pairs of ring 4, then cardinals only
	if got := RingOffsets(40); len(got) != 44 {
		t.Errorf("n=40: %d points", len(got))
	}
}

func TestDoFuncs9And12(t *testing.T) {
	sub := &Spec{ID: 2, Name: "sub", SrvDoFunc: 1, Range: 5}
	bolt := &Spec{ID: 1, Name: "bolt", SrvDoFunc: 9, Range: 30, Vel: 32, MaxVel: 32, SubMissile: [3]string{"sub"}}
	wall := &Spec{ID: 3, Name: "wall", SrvDoFunc: 12, Range: 30, Vel: 32, MaxVel: 32, SubMissile: [3]string{"sub"}}
	tab := specs{"sub": sub, "bolt": bolt, "wall": wall}

	for _, name := range []string{"bolt", "wall"} {
		w := newWorld()
		s := NewSim(w, tab)
		evs := collect(s)

		_, _ = s.Create(CreateParams{Spec: tab[name], Owner: ownerWith(0), X: 30.5, Y: 0.5, DestX: 90.5, DestY: 0.5})
		stepN(s, w, 10)

		if n := len(creates(*evs, "sub")); n < 3 {
			t.Errorf("%s: %d sub missiles after 10 frames, want one per entered subtile", name, n)
		}
	}

	// function 12 needs an owner
	w := newWorld()
	s := NewSim(w, tab)
	evs := collect(s)
	_, _ = s.Create(CreateParams{Spec: wall, X: 30.5, Y: 0.5, DestX: 90.5, DestY: 0.5})
	stepN(s, w, 5)

	if n := len(creates(*evs, "sub")); n != 0 {
		t.Errorf("ownerless wall maker created %d", n)
	}
}

// SrvDoFunc 8: radius Param1 + max(2, L/Param3), period max(3, Param2 - L/Param3).
func TestMonsterBlizzardNumbers(t *testing.T) {
	shard := &Spec{ID: 2, Name: "shard", SrvDoFunc: 3, Range: 9, CollideType: 3}
	ctr := &Spec{ID: 1, Name: "ctr", SrvDoFunc: 8, Range: 40, SubMissile: [3]string{"shard"},
		Param: [5]int{3, 8, 4}}
	tab := specs{"shard": shard, "ctr": ctr}
	w := newWorld()
	s := NewSim(w, tab)
	evs := collect(s)

	// L=16: q=4, radius 3+4=7, period max(3, 8-4)=4 -> elapsed 0,4,..,36 = 10
	_, _ = s.Create(CreateParams{Spec: ctr, Owner: ownerWith(0), SkillID: 5, Level: 16, X: 40.5, Y: 0.5,
		DestX: 40.5, DestY: 0.5})
	stepN(s, w, 40)

	if n := len(creates(*evs, "shard")); n != 10 {
		t.Errorf("%d shards, want 10", n)
	}

	for _, sh := range creates(*evs, "shard") {
		if x := int(sh.X); x < 34 || x > 46 {
			t.Errorf("shard outside radius 6: %v", sh.X)
		}
	}
}

func TestReflectPairAndCastAtPos(t *testing.T) {
	sub := &Spec{ID: 2, Name: "bolt", SrvDoFunc: 1, Range: 5}
	jav := &Spec{ID: 1, Name: "jav", SrvDoFunc: 22, Range: 20, Vel: 32, MaxVel: 32, SubMissile: [3]string{"bolt"}}
	tab := specs{"bolt": sub, "jav": jav}
	w := newWorld()
	s := NewSim(w, tab)
	evs := collect(s)

	_, _ = s.Create(CreateParams{Spec: jav, Owner: ownerWith(0), X: 30.5, Y: 0.5, DestX: 50.5, DestY: 0.5})
	stepN(s, w, 6)

	bolts := creates(*evs, "bolt")
	if len(bolts) < 2 || len(bolts)%2 != 0 {
		t.Fatalf("%d bolts, want pairs", len(bolts))
	}

	// flying along +x: the perpendicular is (0, +20) and (0, -20) offsets
	if bolts[0].DY*bolts[1].DY >= 0 {
		t.Errorf("pair not on opposite sides: %v %v", bolts[0].DY, bolts[1].DY)
	}
}

func TestW23HitFuncs(t *testing.T) {
	sub := &Spec{ID: 2, Name: "sub", SrvDoFunc: 1, Range: 5}
	tab := specs{"sub": sub}

	// 15: one hit sub missile, missile flies on (0)
	s := NewSim(newWorld(), tab)
	evs := collect(s)
	sp := &Spec{ID: 1, Name: "m", SrvHitFunc: 15, HitSubMissile: [4]string{"sub"}}

	if ret, ok := s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0), X: 30.5, Y: 0.5}, nil); !ok || ret != 0 ||
		len(creates(*evs, "sub")) != 1 {
		t.Errorf("hit 15: %d %v", ret, ok)
	}

	// 38: ring count (level-1)*SHitPar2 + SHitPar1, with path type 10
	s = NewSim(newWorld(), tab)
	evs = collect(s)
	sp = &Spec{ID: 1, Name: "m", SrvHitFunc: 38, HitSubMissile: [4]string{"sub"}, SHitPar: [3]int{1, 2}}

	// level 3: n = 2*2+1 = 5 -> first + 2 pairs + cardinals
	if ret, _ := s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0), Level: 3, X: 30.5, Y: 0.5}, nil); ret != resKill {
		t.Errorf("hit 38 ret %d", ret)
	}

	if n, want := len(creates(*evs, "sub")), len(RingOffsets(5)); n != want {
		t.Errorf("hit 38: %d ring missiles, want %d", n, want)
	}

	// 45 with SHitPar1 0: nothing but an end
	s = NewSim(newWorld(), tab)
	evs = collect(s)
	sp = &Spec{ID: 1, Name: "m", SrvHitFunc: 45, HitSubMissile: [4]string{"sub"}}

	if ret, _ := s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0), X: 30.5, Y: 0.5}, nil); ret != resKill ||
		len(creates(*evs, "sub")) != 0 {
		t.Errorf("hit 45 empty: %d", ret)
	}

	// 16: state with at least 5 frames
	s = NewSim(newWorld(), tab)
	evs = collect(s)
	sp = &Spec{ID: 1, Name: "m", SrvHitFunc: 16}
	m := &Missile{Spec: sp, Owner: ownerWith(0), Skill: SkillInfo{AuraTargetState: "slowed", Calc4: 2}}
	ret, _ := s.hitFunc(m, &fakeTarget{id: "x", alive: true})

	var got *Event

	for i := range *evs {
		if (*evs)[i].Kind == EventState {
			got = &(*evs)[i]
		}
	}

	if ret != 0 || got == nil || got.Frames != 5 || got.Name != "slowed" {
		t.Errorf("hit 16: ret %d event %+v", ret, got)
	}

	// 24 and 44: area damage, missile ends
	for _, id := range []int{24, 44} {
		s = NewSim(newWorld(), tab)
		evs = collect(s)
		sp = &Spec{ID: 1, Name: "m", SrvHitFunc: id, SHitPar: [3]int{4}}

		if ret, _ := s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0), X: 30.5, Y: 0.5}, nil); ret != resKill ||
			count(*evs, EventArea) != 1 {
			t.Errorf("hit %d: ret %d events %v", id, ret, kinds(*evs))
		}
	}

	// 31: heals the owner
	healed := 0
	own := ownerWith(0)
	own.Heal = func(a int) { healed += a }
	s = NewSim(newWorld(), tab)
	sp = &Spec{ID: 1, Name: "m", SrvHitFunc: 31, CollideKill: true}
	m = &Missile{Spec: sp, Owner: own, Damage: DamageDesc{}}

	if ret, _ := s.hitFunc(m, &fakeTarget{id: "x", alive: true}); ret != resKill|resDamage {
		t.Errorf("hit 31 ret %d", ret)
	}
}

func TestModelledListGrew(t *testing.T) {
	for _, d := range []int{8, 9, 11, 12, 21, 22, 32} {
		if !DoFuncModelled(d) {
			t.Errorf("do %d", d)
		}
	}

	for _, h := range []int{15, 16, 19, 24, 31, 38, 43, 44, 45} {
		if !HitFuncModelled(h) {
			t.Errorf("hit %d", h)
		}
	}
}

type manaTarget struct {
	fakeTarget
	mana int
}

func (m *manaTarget) Mana() int     { return m.mana }
func (m *manaTarget) SetMana(v int) { m.mana = v }

func TestBurstDrainRadial(t *testing.T) {
	sub := &Spec{ID: 2, Name: "sub", SrvDoFunc: 1, Range: 5}
	tab := specs{"sub": sub}

	s := NewSim(newWorld(), tab)
	evs := collect(s)
	sp := &Spec{ID: 1, Name: "m", SrvHitFunc: 52, HitSubMissile: [4]string{"sub"}, SHitPar: [3]int{2}}
	_, _ = s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0), X: 30.5, Y: 0.5}, nil)

	if n := len(creates(*evs, "sub")); n != 4 {
		t.Errorf("hit 52 step 2: %d missiles, want 4", n)
	}

	tg := &manaTarget{fakeTarget{id: "p", player: true, alive: true}, 200}
	sp = &Spec{ID: 1, Name: "m", SrvHitFunc: 55, SHitPar: [3]int{25}}

	if ret, _ := s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0)}, tg); ret != resDamage || tg.mana != 150 {
		t.Errorf("hit 55: ret %d mana %d", ret, tg.mana)
	}

	s = NewSim(newWorld(), tab)
	evs = collect(s)
	sp = &Spec{ID: 1, Name: "m", SrvHitFunc: 58, HitSubMissile: [4]string{"sub"}, SHitPar: [3]int{3}}
	_, _ = s.hitFunc(&Missile{Spec: sp, Owner: ownerWith(0), X: 30.5, Y: 0.5}, nil)

	if c := creates(*evs, "sub"); len(c) != 1 {
		t.Errorf("hit 58: %d", len(c))
	}
}
