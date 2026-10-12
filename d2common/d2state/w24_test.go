package d2state

import "testing"

func TestRunMonsterRegenPoisonTick(t *testing.T) {
	// 100 points of life, poison of 2.5 points per frame (640 in 8.8)
	out := RunMonsterRegen(MonsterRegen{HP: 100 * 256, MaxHP: 100 * 256, Regen: -640, RoomKnown: true})
	if !out.Rearm || !out.Changed || out.HP != 100*256-640 || out.Kill {
		t.Fatalf("tick: %+v", out)
	}
}

func TestRunMonsterRegenSumsStreams(t *testing.T) {
	// poison -300, burn -200, natural +50: one number, no per-kind rounding
	out := RunMonsterRegen(MonsterRegen{HP: 5000, MaxHP: 9000, Regen: -450, RoomKnown: true})
	if out.HP != 4550 {
		t.Fatalf("hp %d", out.HP)
	}
}

func TestRunMonsterRegenZeroStopsTheEvent(t *testing.T) {
	out := RunMonsterRegen(MonsterRegen{HP: 5000, MaxHP: 9000, Regen: 0, RoomKnown: true})
	if out.Rearm || out.Changed {
		t.Fatalf("net 0 must cancel the event: %+v", out)
	}
}

func TestRunMonsterRegenPreventHeal(t *testing.T) {
	// state 0x34 blocks regeneration and does not re-arm
	out := RunMonsterRegen(MonsterRegen{HP: 5000, MaxHP: 9000, Regen: 40, PreventHeal: true, RoomKnown: true})
	if out.Rearm || out.Changed || out.HP != 5000 {
		t.Fatalf("regen not blocked: %+v", out)
	}

	// ... but damage still runs
	out = RunMonsterRegen(MonsterRegen{HP: 5000, MaxHP: 9000, Regen: -40, PreventHeal: true, RoomKnown: true})
	if !out.Changed || out.HP != 4960 {
		t.Fatalf("damage blocked: %+v", out)
	}

	// a zero net with the state also stops without scheduling
	out = RunMonsterRegen(MonsterRegen{HP: 5000, MaxHP: 9000, Regen: 0, PreventHeal: true, RoomKnown: true})
	if out.Rearm {
		t.Fatalf("rearmed: %+v", out)
	}
}

func TestRunMonsterRegenLifeStateDropsBase(t *testing.T) {
	// base regeneration 30, poison -100: with a mask-14C state only the poison counts... total stat is -70
	out := RunMonsterRegen(MonsterRegen{HP: 5000, MaxHP: 9000, Regen: -70, BaseRegen: 30, LifeFlagState: true, RoomKnown: true})
	if out.HP != 4900 {
		t.Fatalf("hp %d", out.HP)
	}
}

func TestRunMonsterRegenClampAtMax(t *testing.T) {
	out := RunMonsterRegen(MonsterRegen{HP: 8990, MaxHP: 9000, Regen: 40, RoomKnown: true})
	if out.HP != 9000 || out.Rearm {
		t.Fatalf("clamp: %+v", out)
	}
}

func TestRunMonsterRegenTownProtectsLastPoint(t *testing.T) {
	for _, in := range []MonsterRegen{
		{HP: 200, MaxHP: 9000, Regen: -100, RoomKnown: true, InTown: true},
		{HP: 200, MaxHP: 9000, Regen: -100, RoomKnown: false},
	} {
		out := RunMonsterRegen(in)
		if out.Changed || out.Kill || out.HP != 200 || !out.Rearm {
			t.Fatalf("%+v -> %+v", in, out)
		}
	}

	// above one whole point the town does not protect
	out := RunMonsterRegen(MonsterRegen{HP: 600, MaxHP: 9000, Regen: -100, RoomKnown: true, InTown: true})
	if !out.Changed || out.HP != 500 {
		t.Fatalf("town above 1 hp: %+v", out)
	}

	// regeneration is never blocked by the town
	out = RunMonsterRegen(MonsterRegen{HP: 200, MaxHP: 9000, Regen: 10, InTown: true})
	if out.HP != 210 {
		t.Fatalf("town regen: %+v", out)
	}
}

func TestRunMonsterRegenKill(t *testing.T) {
	out := RunMonsterRegen(MonsterRegen{HP: 400, MaxHP: 9000, Regen: -500, RoomKnown: true})
	if !out.Kill || out.HP != 0 {
		t.Fatalf("kill: %+v", out)
	}

	// the threshold is 1/256 of a point, not one point: 255 left alive when the tick leaves 255... falls to 0 only below 1
	out = RunMonsterRegen(MonsterRegen{HP: 700, MaxHP: 9000, Regen: -500, RoomKnown: true})
	if out.Kill || out.HP != 200 {
		t.Fatalf("200/256 of a point is alive: %+v", out)
	}

	// uninterruptable: delayed death
	out = RunMonsterRegen(MonsterRegen{HP: 400, MaxHP: 9000, Regen: -500, RoomKnown: true, Uninterruptable: true})
	if out.Kill || !out.Delay || out.HP != 0 {
		t.Fatalf("uninterruptable: %+v", out)
	}

	// already dead: no second kill
	out = RunMonsterRegen(MonsterRegen{HP: 400, MaxHP: 9000, Regen: -500, RoomKnown: true, Dead: true})
	if out.Kill || out.Delay {
		t.Fatalf("dead: %+v", out)
	}
}

func TestRunPlayerRegenNeverKills(t *testing.T) {
	out := RunPlayerRegen(PlayerRegen{HP: 300, MaxHP: 9000, Regen: -500})
	if out.HP != LifeUnit {
		t.Fatalf("player floor: %d", out.HP)
	}

	out = RunPlayerRegen(PlayerRegen{HP: 5000, MaxHP: 9000, Regen: -500})
	if out.HP != 4500 {
		t.Fatalf("hp %d", out.HP)
	}

	out = RunPlayerRegen(PlayerRegen{HP: 8900, MaxHP: 9000, Regen: 500})
	if out.HP != 9000 || !out.DropHealthPot {
		t.Fatalf("max: %+v", out)
	}

	out = RunPlayerRegen(PlayerRegen{HP: 5000, MaxHP: 9000, Regen: 0})
	if out.HP != 5000 || out.DropHealthPot {
		t.Fatalf("zero: %+v", out)
	}

	out = RunPlayerRegen(PlayerRegen{HP: 5000, MaxHP: 9000, Regen: -500, Dead: true})
	if out.HP != 5000 {
		t.Fatalf("dead: %+v", out)
	}
}

func TestPotionRecoveryTotal(t *testing.T) {
	tab := [5]int{9, 18, 27, 36, 45}

	cases := map[int]int{1: 0, 2: 9, 15: 9 * 14, 16: 9*14 + 18, 30: 9*14 + 18*15, 31: 9*14 + 18*15 + 27,
		45: 9*14 + 18*15 + 27*15, 46: 9*14 + 18*15 + 27*15 + 36, 60: 9*14 + 18*15 + 27*15 + 36*15,
		61: 9*14 + 18*15 + 27*15 + 36*15 + 45}

	// the exe's boundaries are strict: 0xf < level uses the second slope, so level 15 is on the first
	cases[15] = 14 * 9
	cases[30] = 14*9 + 15*18
	cases[45] = 14*9 + 15*18 + 15*27
	cases[60] = 14*9 + 15*18 + 15*27 + 15*36

	for lvl, want := range cases {
		if got := potionRecoveryTotal(lvl, tab); got != want {
			t.Errorf("level %d: %d want %d", lvl, got, want)
		}
	}
}

func TestOpenWoundsPerFrame(t *testing.T) {
	base := OpenWoundsPerFrame(30, false, false)
	if base != 9*14+18*15+40 {
		t.Fatalf("level 30: %d", base)
	}

	if OpenWoundsPerFrame(30, true, false) != base/4 {
		t.Fatalf("player quarter")
	}

	if OpenWoundsPerFrame(30, false, true) != base/2 {
		t.Fatalf("champion / unique half")
	}

	if OpenWoundsPerFrame(0, false, false) != 40 {
		t.Fatalf("level floor: %d", OpenWoundsPerFrame(0, false, false))
	}
}

func TestRearmAfterHit(t *testing.T) {
	if !RearmAfterHit(true, true, 10, -5) || RearmAfterHit(true, true, 10, 0) || RearmAfterHit(false, true, 10, -5) ||
		RearmAfterHit(true, false, 10, -5) || RearmAfterHit(true, true, 0, -5) {
		t.Fatal("rearm rule")
	}
}

func TestSpeedFormulas(t *testing.T) {
	if EffectiveSpeed(120, 60) != 40 || EffectiveSpeed(150, 30) != 25 || EffectiveSpeed(120, 0) != 0 {
		t.Fatal("diminishing returns")
	}

	// chill -50 on a unit with 30 effective FRW
	if WalkRunPct(0, -50) != 25 {
		t.Fatalf("walk floor: %d", WalkRunPct(0, -50))
	}

	if WalkRunPct(1000, -50) != EffectiveSpeed(150, 1000)-50 {
		t.Fatal("walk with frw")
	}

	if AttackRatePct(0, -50, 0) != 15 || AttackRatePct(5000, 100, 0) != 175 {
		t.Fatal("attack clamp")
	}

	if OtherRatePct(-50) != 15 || OtherRatePct(500) != 175 || OtherRatePct(100) != 100 {
		t.Fatal("other clamp")
	}

	if CastRatePct(0) != 100 || CastRatePct(100000) != 175 {
		t.Fatal("cast")
	}

	// chill does not touch hit recovery or block
	if HitRecoveryPct(0) != 50 || BlockRatePct(0, false) != 50 || BlockRatePct(0, true) != 100 {
		t.Fatal("hit recovery and block")
	}

	if AnimRate(256, 50) != 128 || AnimRate(256, -3) != 0 || AnimRate(0x7fff, 400) != 0x7fff {
		t.Fatal("anim rate")
	}
}

func TestFrozenSkipsEvent(t *testing.T) {
	for _, ev := range []int{3, 4, 5, 8, 0xc} {
		if FrozenSkipsEvent(ev, true, false) {
			t.Errorf("event %d must pass for a frozen unit", ev)
		}
	}

	for _, ev := range []int{0, 1, 2, 6, 7, 9, 0xa, 0xb, 0xd, 0xe} {
		if !FrozenSkipsEvent(ev, true, false) {
			t.Errorf("event %d must be skipped for a frozen unit", ev)
		}
	}

	if FrozenSkipsEvent(1, true, true) || FrozenSkipsEvent(1, false, false) {
		t.Fatal("only live frozen units skip")
	}
}

func TestCurseGate(t *testing.T) {
	if n, ok := CurseGate(false, 100, 500, true); !ok || n != 500 {
		t.Fatal("non-curse ignores everything")
	}

	if _, ok := CurseGate(true, 100, 500, false); ok {
		t.Fatal("immune")
	}

	if n, ok := CurseGate(true, 25, 500, false); !ok || n != 375 {
		t.Fatalf("shortened: %d", n)
	}

	if _, ok := CurseGate(true, 0, 500, true); ok {
		t.Fatal("attract refuses curses")
	}
}

func TestAttractRefusesCurses(t *testing.T) {
	s := New()
	s.SetDefs(Defs{
		"attract":       {Name: "attract", Curse: true},
		"amplifydamage": {Name: "amplifydamage", Curse: true},
		"fanaticism":    {Name: "fanaticism"},
	})

	if ok, _ := s.ApplyTimed(0, Instance{Name: "attract", Until: 100, SkillID: 1}); !ok {
		t.Fatal("first attract refused")
	}

	if ok, _ := s.ApplyTimed(1, Instance{Name: "amplifydamage", Until: 100, SkillID: 2}); ok {
		t.Fatal("amplify landed on an attracted unit")
	}

	if s.Active(1, "amplifydamage") {
		t.Fatal("amplify present")
	}

	// a second attract does not refresh the first
	s.ApplyTimed(2, Instance{Name: "attract", Until: 300, SkillID: 1})

	if s.Get(2, "attract").Until != 100 {
		t.Fatal("attract refreshed")
	}

	// a non-curse state is not checked
	if ok, _ := s.ApplyTimed(3, Instance{Name: "fanaticism", Until: 100, SkillID: 3}); !ok {
		t.Fatal("non-curse refused")
	}

	// after it ends, curses land again
	if ok, _ := s.ApplyTimed(150, Instance{Name: "amplifydamage", Until: 400, SkillID: 2}); !ok {
		t.Fatal("curse refused after attract ended")
	}
}
