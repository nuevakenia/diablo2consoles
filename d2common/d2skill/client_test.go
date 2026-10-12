package d2skill

import "testing"

func TestMultiShotStep(t *testing.T) {
	cases := []struct {
		name         string
		caster, tgt  SubPoint
		wantX, wantY int
	}{
		{"east", SubPoint{0, 0}, SubPoint{10, 0}, 0, -1},
		{"south", SubPoint{0, 0}, SubPoint{0, 10}, 1, 0},
		{"west", SubPoint{0, 0}, SubPoint{-8, 0}, 0, 1},
		{"short east", SubPoint{5, 5}, SubPoint{6, 5}, 0, -1},
		{"diagonal", SubPoint{0, 0}, SubPoint{7, 7}, 1, -1},
	}
	for _, c := range cases {
		x, y := MultiShotStep(c.caster, c.tgt)
		if x != c.wantX || y != c.wantY {
			t.Errorf("%s: step (%d,%d), want (%d,%d)", c.name, x, y, c.wantX, c.wantY)
		}
	}
}

func TestMultiShotPlan(t *testing.T) {
	// firing east at (10,0): step (0,-1); 5 arrows start at y = 0 - (-1*5/2) = 2
	got := MultiShotPlan(SubPoint{0, 0}, SubPoint{10, 0}, 5, 1)
	if len(got) != 5 {
		t.Fatalf("%d shots", len(got))
	}

	for i, s := range got {
		want := SubPoint{10, 2 - i}
		if s.To != want || s.From != (SubPoint{}) || s.Kind != KindMultiShot || s.Loop != i {
			t.Errorf("arrow %d = %+v, want aim %v", i, s, want)
		}
	}

	if MultiShotPlan(SubPoint{}, SubPoint{1, 1}, 0, 1) != nil {
		t.Error("zero count must give nothing")
	}

	if MultiShotSlot(1, true) != 1 || MultiShotSlot(3, true) != 2 || MultiShotSlot(3, false) != 1 {
		t.Error("slot choice")
	}
}

func TestChargedStrikePlan(t *testing.T) {
	got := ChargedStrikePlan(SubPoint{2, 3}, SubPoint{6, 5}, 3, 1)
	if len(got) != 3 {
		t.Fatal(len(got))
	}

	for i, s := range got {
		if s.From != (SubPoint{6, 5}) || s.To != (SubPoint{10, 7}) || s.Loop != i || s.Kind != KindAimed || !s.PathType10 || s.PathSeed != uint32(10+i) {
			t.Errorf("bolt %d = %+v", i, s)
		}
	}
}

func TestGuidedArrowPlan(t *testing.T) {
	u := GuidedArrowPlan(SubPoint{10, 10}, SubPoint{14, 7}, true, 1)
	if u.Kind != KindHomingUnit || u.Data28 != 1 || u.Data2C != (-3)*0x10000+4 {
		t.Errorf("unit: %+v", u)
	}

	g := GuidedArrowPlan(SubPoint{10, 10}, SubPoint{14, 7}, false, 1)
	if g.Kind != KindHomingGround || g.Data28 != 2 {
		t.Errorf("ground: %+v", g)
	}
}

func TestLightningStrikePlan(t *testing.T) {
	main := ChainTarget{5, SubPoint{10, 10}}
	others := []ChainTarget{main, {2, SubPoint{30, 10}}, {7, SubPoint{14, 10}}, {6, SubPoint{12, 12}}, {1, SubPoint{9, 10}}}

	// 0x4c28d0: the smallest id ABOVE the struck unit (6), not the nearest
	s, ok := LightningStrikePlan(main, others, 5, 4, 1)
	if !ok || s.To != (SubPoint{12, 12}) || s.Data28 != 4 || s.Parent != 5 || s.Kind != 0x20 {
		t.Errorf("%v %+v", ok, s)
	}

	// nothing above: the smallest id of all (1)
	s, ok = LightningStrikePlan(main, []ChainTarget{main, {1, SubPoint{9, 10}}, {3, SubPoint{8, 10}}}, 5, 4, 1)
	if !ok || s.To != (SubPoint{9, 10}) {
		t.Errorf("fallback %v %+v", ok, s)
	}

	// alone (or the others out of range): the bolt is drawn on the struck unit itself
	s, ok = LightningStrikePlan(main, others[:2], 5, 4, 1)
	if !ok || s.To != main.Pos {
		t.Errorf("alone %v %+v", ok, s)
	}
}

func TestClientScanNext(t *testing.T) {
	if _, ok := ClientScanNext(nil, 3); ok {
		t.Error("empty")
	}

	got, _ := ClientScanNext([]ChainTarget{{9, SubPoint{}}, {4, SubPoint{}}, {2, SubPoint{}}}, 3)
	if got.ID != 4 {
		t.Errorf("above: %d", got.ID)
	}

	got, _ = ClientScanNext([]ChainTarget{{2, SubPoint{}}, {1, SubPoint{}}, {3, SubPoint{}}}, 3)
	if got.ID != 1 {
		t.Errorf("fallback: %d", got.ID)
	}
}

func TestStrafeClientPlan(t *testing.T) {
	cur := ChainTarget{1, SubPoint{5, 0}}
	others := []ChainTarget{cur, {2, SubPoint{9, 0}}, {3, SubPoint{3, 0}}, {4, SubPoint{40, 0}}}

	s, left, next, ok := StrafeClientPlan(SubPoint{0, 0}, cur, 3, others, 10, 1)
	if s.To != cur.Pos || left != 2 || !ok || next.ID != 2 { // the smallest id above 1 in range
		t.Errorf("%+v %d %+v %v", s, left, next, ok)
	}

	if _, left, _, ok = StrafeClientPlan(SubPoint{}, cur, 1, others, 10, 1); left != 0 || ok {
		t.Error("last arrow ends the burst")
	}

	// a lone enemy: the scan hands it back, so the burst goes on at it (VERIFIED wave 24)
	if _, _, next, ok = StrafeClientPlan(SubPoint{}, cur, 3, []ChainTarget{cur}, 10, 1); !ok || next.ID != cur.ID {
		t.Error("a lone enemy keeps the burst")
	}

	if shots := StrafeClientBurst(SubPoint{}, cur, 4, []ChainTarget{cur}, 10, 1); len(shots) != 4 {
		t.Errorf("%d arrows at a lone enemy, want 4", len(shots))
	}

	if StrafeSlot(1, true) != 1 || StrafeSlot(3, true) != 2 || StrafeSlot(3, false) != 1 {
		t.Error("weapon class picks the column")
	}
}

func TestSynergyMissileSlot(t *testing.T) {
	cases := []struct {
		bits4, stat, want int
		has               bool
	}{
		{0, 5, 1, true}, {4, 5, 1, false}, {4, 1, 1, true}, {4, 2, 2, true}, {4, 3, 3, true}, {4, 9, 3, true},
	}
	for _, c := range cases {
		if got := SynergyMissileSlot(c.bits4, c.has, c.stat); got != c.want {
			t.Errorf("%+v got %d", c, got)
		}
	}
}
