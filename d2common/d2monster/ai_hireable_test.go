package d2monster

import "testing"

// fakeMerc adds the MercWorld methods to fakeWorld.
type fakeMerc struct {
	*fakeWorld
	owner    OwnerInfo
	noOwner  bool
	teleport int
	level    int
	ranged   bool
	cast     CastResult
	casts    int
}

func (f *fakeMerc) Owner(*Brain) (OwnerInfo, bool) { return f.owner, !f.noOwner }
func (f *fakeMerc) Teleport(*Brain) bool           { f.teleport++; return true }
func (f *fakeMerc) MercLevel(*Brain) int           { return f.level }
func (f *fakeMerc) IsRanged(*Brain) bool           { return f.ranged }
func (f *fakeMerc) ChooseAndCast(*Brain, Target, int) CastResult {
	f.casts++

	return f.cast
}

func newMerc(ownerDist int) (*fakeMerc, *Brain) {
	fw := newFake(5, true)
	fw.hasTarget = false // no enemy unless a test enables it
	f := &fakeMerc{
		fakeWorld: fw, level: 20, cast: CastBusy,
		owner: OwnerInfo{Target: Target{ID: 1, X: 100 + ownerDist, Y: 100, Size: 1, IsPlayer: true}, Mode: ModeNeutral},
	}
	b := brainAt(profile("Hireable"))
	b.Class = 338

	def, _ := Lookup("Hireable")
	b.SetAI(def)

	return f, b
}

func TestHireableFollowLeash(t *testing.T) {
	for _, tc := range []struct {
		name      string
		dist      int
		ownerMode Mode
		wantLog   string
		teleports int
	}{
		{"teleport above 100", 120, ModeNeutral, "", 1},
		{"run beyond the outer ring (24)", 30, ModeNeutral, "run-target/5", 0},
		{"owner walking, beyond far (16)", 20, ModeWalk, "walk-target/5", 0},
		{"owner running, beyond far (16)", 20, ModeRun, "run-target/5", 0},
		{"owner standing, within outer: no follow", 20, ModeNeutral, "", 0},
		{"close: idle", 8, ModeNeutral, "", 0},
	} {
		f, b := newMerc(tc.dist)
		f.owner.Mode = tc.ownerMode
		b.Mode = ModeNeutral

		Tick(f, b)

		if f.teleport != tc.teleports || (tc.wantLog != "" && f.last() != tc.wantLog) ||
			(tc.wantLog == "" && len(f.log) != 0) {
			t.Errorf("%s: log=%v teleports=%d", tc.name, f.log, f.teleport)
		}
	}
}

func TestHireableAttackAndIdle(t *testing.T) {
	// enemy in sight: the skill chooser runs
	f, b := newMerc(8)
	f.hasTarget, f.attackOK = true, true
	f.dist = 2

	Tick(f, b)

	if f.casts != 1 {
		t.Fatalf("casts = %d", f.casts)
	}

	// a failed cast sleeps 10 frames instead of spinning
	f, b = newMerc(8)
	f.hasTarget, f.attackOK, f.cast = true, true, CastFailed
	f.dist = 2

	Tick(f, b)

	if b.Wake != 10 {
		t.Errorf("wake = %d, want 10", b.Wake)
	}

	// no owner: idle 10
	f, b = newMerc(8)
	f.noOwner = true

	Tick(f, b)

	if len(f.log) != 0 || b.Wake != 10 {
		t.Errorf("no owner: log=%v wake=%d", f.log, b.Wake)
	}

	// far away enemies (beyond 25) are ignored
	f, b = newMerc(8)
	f.hasTarget, f.attackOK = true, true
	f.dist = 40

	Tick(f, b)

	if f.casts != 0 {
		t.Error("cast at a target beyond 25")
	}
}

func TestHireablePressureThreshold(t *testing.T) {
	// the desert guard (338) uses the fixed threshold 0x62 = 98: almost every
	// roll is below it, so the pressure counter stays 0 and it casts
	f, b := newMerc(8)
	f.hasTarget, f.attackOK, f.dist = true, true, 2
	casts := 0

	for i := 0; i < 200; i++ {
		b.Wake = 0
		f.frame = i

		before := f.casts

		Tick(f, b)

		if f.casts > before {
			casts++
		}
	}

	if casts < 180 {
		t.Errorf("fixed threshold 98: casts = %d of 200", casts)
	}

	// a level-1 rogue (not fixed): thr = 0x28+2 = 42 -> about 58% of the rolls bump and wait
	f, b = newMerc(8)
	b.Class = 271
	f.ranged, f.level = true, 1
	f.hasTarget, f.attackOK, f.dist = true, true, 2 // within 3 and visible
	casts = 0

	for i := 0; i < 400; i++ {
		b.Wake = 0
		f.frame = i

		before := f.casts

		Tick(f, b)

		if f.casts > before {
			casts++
		}
	}

	if casts < 60 || casts > 330 {
		t.Errorf("level-1 archer casts = %d of 400, want a minority", casts)
	}
}

// The desert guard (AIParam1 != 0) runs at a target farther than 2 edge tiles
// instead of casting; the rogue (AIParam1 == 0) casts from range (the chooser
// walks in) and only shuffles within 3 tiles (VERIFIED 0x5e3fa0, wave 20).
func TestHireableBranchByAIParam(t *testing.T) {
	f, b := newMerc(8)
	f.hasTarget, f.attackOK, f.dist = true, true, 6
	Tick(f, b)

	if f.casts != 0 || f.last() != "run-target/1" {
		t.Errorf("guard far target: casts=%d log=%v", f.casts, f.log)
	}

	f, b = newMerc(8)
	b.Class = 271
	f.level = 98
	f.hasTarget, f.attackOK, f.dist = true, true, 8
	Tick(f, b)

	if f.casts != 1 {
		t.Errorf("rogue far target: casts=%d log=%v", f.casts, f.log)
	}

	shuffled := 0

	f, b = newMerc(8)
	b.Class = 271
	f.level = 98
	f.hasTarget, f.attackOK, f.dist = true, true, 2

	for i := 0; i < 200; i++ {
		b.Wake = 0
		f.frame = i
		f.log = nil

		before := f.casts
		Tick(f, b)

		if f.casts == before && len(f.log) > 0 {
			shuffled++
		}
	}

	if shuffled < 50 || shuffled > 150 {
		t.Errorf("rogue shuffled %d of 200 within 3 tiles, want about half", shuffled)
	}
}
