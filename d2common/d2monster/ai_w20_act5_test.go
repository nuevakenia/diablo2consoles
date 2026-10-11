package d2monster

import "testing"

// stateWorld adds the aura-state hosts to fakeBaal.
type stateWorld struct {
	*fakeBaal
	auraState, auraTarget int
	targetStates          map[int]bool
	blocked               bool
	cands                 []DiaCandidate
}

func (s *stateWorld) SlotAuraState(*Brain, int) int       { return s.auraState }
func (s *stateWorld) SlotAuraTargetState(*Brain, int) int { return s.auraTarget }
func (s *stateWorld) TargetHasState(_ *Brain, _ Target, st int) bool {
	return s.targetStates[st]
}
func (s *stateWorld) LineBlocked(*Brain, Target, int) bool { return s.blocked }
func (s *stateWorld) DiabloCandidates(*Brain) []DiaCandidate {
	return s.cands
}

func newStateWorld(dist int, inRange bool) *stateWorld {
	return &stateWorld{
		fakeBaal: &fakeBaal{fakeWorld: newFake(dist, inRange)}, auraState: -1, auraTarget: -1,
		targetStates: map[int]bool{},
	}
}

func TestSkillStatesFree(t *testing.T) {
	for _, tc := range []struct {
		name      string
		min       int
		aura      int
		self      map[int]bool
		auraT     int
		tgt       map[int]bool
		wantClear bool
	}{
		{"no states", 0, -1, nil, -1, nil, true},
		{"own aurastate up blocks", 0, 5, map[int]bool{5: true}, -1, nil, false},
		{"own aurastate down passes", 0, 5, nil, -1, nil, true},
		{"target state up blocks", 0, -1, nil, 7, map[int]bool{7: true}, false},
		{"state id 0 counts for the throne", 0, 0, map[int]bool{0: true}, -1, nil, false},
		{"state id 0 ignored for the ancient", 1, 0, map[int]bool{0: true}, -1, nil, true},
	} {
		w := newStateWorld(10, false)
		w.auraState, w.auraTarget, w.targetStates = tc.aura, tc.auraT, tc.tgt

		if tc.self != nil {
			w.states = tc.self
		}

		b := brainAt(skills(profile("BaalThrone", 100), "Static"))
		c := &Ctx{B: b, W: w}

		if got := c.skillStatesFree(slot1, Target{ID: 1}, tc.min); got != tc.wantClear {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.wantClear)
		}
	}
}

func TestBaalThroneSkillGate(t *testing.T) {
	// the aura state of Skill1 already up on the throne: no cast, wait 10
	w := newStateWorld(30, false)
	w.auraState = 3
	w.states[3] = true

	b := brainAt(skills(profile("BaalThrone", 100), "Static"))
	b.Profile.AIDist = 64
	Tick(w, b)

	if w.last() == "cast0" {
		t.Fatalf("throne cast through its own aura state: %v", w.log)
	}

	// state down: cast
	w2 := newStateWorld(30, false)
	w2.auraState = 3

	b2 := brainAt(skills(profile("BaalThrone", 100), "Static"))
	b2.Profile.AIDist = 64
	Tick(w2, b2)

	if w2.last() != "cast0" {
		t.Fatalf("throne did not cast: %v", w2.log)
	}
}

// BaalTaunt: the cast ends the think even when the host refuses it, and a far
// hero is reached by moving the TAUNTER (SelfMover), then waiting 25.
func TestBaalTauntW20(t *testing.T) {
	w := &fakeBaal{fakeWorld: newFake(50, false)}
	b := brainAt(profile("BaalTaunt", 20, 2, 40))
	b.Profile.AIDist = 55
	Tick(w, b)

	if !w.pulled {
		t.Fatal("taunter did not hop onto the far hero")
	}

	if b.Wake != 25 {
		t.Fatalf("wake %d, want 25 after the hop", b.Wake)
	}
}

func TestBaalCrabEngagedIsLineOfSight(t *testing.T) {
	// out of reach, line blocked: the idle table (no Nova/Cold/Inferno weights
	// of the engaged table); line clear: the engaged table
	p := skills(profile("BaalCrab"), "Baal Nova", "Baal Inferno", "Baal Tentacle", "Baal Cold Missiles")

	count := func(blocked bool) map[string]int {
		seen := map[string]int{}

		for id := uint32(1); id < 600; id++ {
			w := newStateWorld(30, false)
			w.blocked = blocked
			w.cands = []DiaCandidate{{Target: Target{ID: 1, X: 120, Y: 100, Size: 1, IsPlayer: true},
				Stats: DiaStats{LifePct: 100, S24: 10}}}

			b := NewBrain(id, 1, Hell, p, testSeed)
			b.X, b.Y = 100, 100
			Tick(w, b)

			seen[w.last()]++
		}

		return seen
	}

	clear, blocked := count(false), count(true)

	// the idle table has no Nova weight with a lone target and no far flag
	if blocked["cast0"] != 0 {
		t.Errorf("blocked line still cast Nova: %v", blocked)
	}

	if clear["cast0"] == 0 {
		t.Errorf("clear line never cast Nova: %v", clear)
	}
}

func TestBaalCrabNearbyIsDifficulty(t *testing.T) {
	// Buff (weight 40, 70 above score 0x3c) is zeroed for a lone candidate in
	// Normal and Nightmare only (element 3 of the scaling block).
	for _, tc := range []struct {
		d    Difficulty
		want int
	}{{Normal, 0}, {Nightmare, 0}, {Hell, 40}} {
		s := BaalSituation{Engaged: true, SelfHP: 100, Targets: 1, Dist: 5, Nearby: int(tc.d)}
		if int(tc.d) > 2 {
			s.Nearby = 2
		}

		if got := BaalWeights(tc.d, s)[baalBuff]; got != tc.want {
			t.Errorf("diff %d: buff weight %d, want %d", tc.d, got, tc.want)
		}
	}
}

func TestBaalCloneRules(t *testing.T) {
	p := skills(profile("BaalCrabClone"), "Baal Nova", "Baal Inferno", "Baal Tentacle", "Baal Cold Missiles")

	// owner gone -> dismissed even with a live leader
	w := &fakeBaal{fakeWorld: newFake(10, true), ownerGone: true}
	b := brainAt(p)
	b.Leader = brainAt(p)
	Tick(w, b)

	if !w.dismiss {
		t.Fatal("clone with a dead owner survives")
	}

	// no target: sleeps 15 after deciding
	w2 := &fakeBaal{fakeWorld: newFake(10, true)}
	w2.hasTarget = false
	b2 := brainAt(p)
	b2.Leader = brainAt(p)
	Tick(w2, b2)

	if w2.dismiss || b2.Wake != 15 {
		t.Fatalf("targetless clone: dismiss=%v wake=%d (want sleep 15)", w2.dismiss, b2.Wake)
	}
}

func TestBaalCrabScansWithLeash(t *testing.T) {
	// two candidates; the source supplies them, the best score wins and the
	// count reaches the tables
	w := newStateWorld(10, false)
	w.hasTarget = false
	w.attackOK = false
	w.cands = []DiaCandidate{
		{Target: Target{ID: 1, X: 105, Y: 100, Size: 1, IsPlayer: true}, Stats: DiaStats{LifePct: 100, S24: 10}},
		{Target: Target{ID: 2, X: 106, Y: 100, Size: 1, IsPlayer: true}, Stats: DiaStats{LifePct: 10, S24: 90, S25: 90}},
	}

	p := skills(profile("BaalCrab"), "Baal Nova", "Baal Inferno", "Baal Tentacle", "Baal Cold Missiles")
	b := NewBrain(7, 1, Hell, p, testSeed)
	b.X, b.Y = 100, 100

	c := &Ctx{B: b, W: w}
	best, score, count, _ := c.pickAway(nil, crabAway)

	if best == nil || best.ID != 2 || count != 2 || score <= 0 {
		t.Fatalf("pick = %+v score %d count %d", best, score, count)
	}
}

func TestAncientStatesAndScan(t *testing.T) {
	// state B always scans (the attack target is ignored) and honours the
	// skill's states
	w := newStateWorld(10, true)
	w.cands = []DiaCandidate{{Target: Target{ID: 9, X: 108, Y: 100, Size: 1, IsPlayer: true},
		Stats: DiaStats{LifePct: 100, S24: 20}}}
	w.auraState = 4
	w.states[4] = true

	p := skills(profile("Ancient", 20, 100, 100, 10, 5), "Static")
	b := NewBrain(3, ancientB, Hell, p, testSeed)
	b.X, b.Y = 100, 100

	c := &Ctx{B: b, W: w}
	c.Target, c.Dist = &Target{ID: 1, X: 150, Y: 100}, 50
	tg, d, ok := ancientTarget(c, false)

	if !ok || tg.ID != 9 || d != 7 {
		t.Fatalf("state B target %+v d=%d ok=%v (want the scanned hero 9)", tg, d, ok)
	}

	tg, _, ok = ancientTarget(c, true)
	if !ok || tg.ID != 1 {
		t.Fatalf("state A target %+v (want the attack target 1)", tg)
	}

	// the Skill1 cast is skipped while its aurastate is up
	Tick(w, b)

	for _, l := range w.log {
		if l == "cast0" {
			t.Fatalf("ancient B cast through the aura state: %v", w.log)
		}
	}
}

func TestBaalMinionRollsWithoutTarget(t *testing.T) {
	// the approach roll is drawn even when there is no target
	w := newFake(10, false)
	w.hasTarget, w.attackOK = false, false
	b := brainAt(profile("BaalMinion", 50, 50, 50, 10))
	before := *b.Seed

	thinkBaalMinion(&Ctx{B: b, W: w})

	if *b.Seed == before {
		t.Fatal("no roll drawn without a target")
	}
}
