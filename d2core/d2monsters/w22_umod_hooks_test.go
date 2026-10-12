package d2monsters

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func hooksHero(x, y float64) *d2mapentity.Player {
	p := &d2mapentity.Player{Stats: &d2hero.HeroStatsState{Health: 100, MaxHealth: 100}}
	p.Position.Set(x, y)

	return p
}

// skillsDirector loads the real skills.txt (needs $D2_TABLES).
func skillsDirector(t *testing.T) *Director {
	t.Helper()

	root := os.Getenv("D2_TABLES")
	if root == "" {
		t.Skip("D2_TABLES not set")
	}

	rm, err := d2records.NewRecordManager(d2util.LogLevelNone)
	if err != nil {
		t.Fatal(err)
	}

	buf, err := os.ReadFile(filepath.Join(root, "monsters", "patch_d2", "skills.txt"))
	if err != nil {
		t.Skip(err)
	}

	if err = rm.Load(d2resource.Skills, d2txt.LoadDataDictionary(buf)); err != nil {
		t.Fatal(err)
	}

	d := testDirector()
	d.asset = &d2asset.AssetManager{Records: rm}
	d.targets = map[uint32]*d2mapentity.Player{}
	d.opt.IgnoreTown = true

	return d
}

func cursedUnit(level int, unique bool) *unit {
	m := fxTestMonster()
	m.Modifiers = []int{7}
	m.Vitals.Level = level

	if !unique {
		m.TypeFlags = 0
	}

	return &unit{m: m, b: d2monster.NewBrain(1, 1, d2monster.Normal, &d2monster.Profile{}, 99)}
}

// Cursed: a unique's landed hit curses the heroes around its target with Amplify Damage three times in four
// (physical resistance -100 for 200 + 75 * (level - 1) frames at skill level monster level / 5 + 1).
func TestCursedHitCursesNearbyHeroes(t *testing.T) {
	d := skillsDirector(t)
	near, far := hooksHero(100, 100), hooksHero(120, 100)
	d.targets[1], d.targets[2] = near, far
	d.frame = 1000

	u := cursedUnit(40, true) // skill level 9: radius 11, 800 frames

	// replay the exe's gate on a copy of the seed: the cast happens exactly when the low two bits are not zero
	ref := *u.b.Seed
	want := ref.Step()&3 != 0

	d.cursedHit(u, near)

	got := d.cursedStats(near)
	if want {
		if got["damageresist"] != -100 {
			t.Fatalf("near hero stats %v, want damageresist -100", got)
		}

		if d.curses[near]["damageresist"].until != 1000+800 {
			t.Errorf("until %d, want 1800", d.curses[near]["damageresist"].until)
		}

		if d.Counters.Curses != 1 {
			t.Errorf("curses %d (the far hero at 20 subtiles is outside the radius of 11)", d.Counters.Curses)
		}
	} else if got != nil {
		t.Fatalf("the seed said no cast but stats %v", got)
	}

	if d.cursedStats(far) != nil {
		t.Errorf("far hero cursed: %v", d.cursedStats(far))
	}

	// the curse ends
	d.frame = 1800
	if d.cursedStats(near) != nil {
		t.Error("the curse must have expired")
	}

	// the merge for the hero's stat totals
	d.frame = 1000
	d.curses = nil
	d.curseHero(near, nil, 10)

	if d.TimedHeroStats(near) != nil {
		t.Error("no stat, no timed entry")
	}
}

func TestCursedNeedsUniqueAndModifier(t *testing.T) {
	d := skillsDirector(t)
	hero := hooksHero(10, 10)
	d.targets[1] = hero

	plain := cursedUnit(40, false) // not a unique
	before := *plain.b.Seed

	d.cursedHit(plain, hero)

	if *plain.b.Seed != before || d.cursedStats(hero) != nil {
		t.Error("a monster that is not unique neither rolls nor curses")
	}

	other := cursedUnit(40, true)
	other.m.Modifiers = []int{9}
	before = *other.b.Seed

	d.cursedHit(other, hero)

	if *other.b.Seed != before {
		t.Error("without modifier 7 the seed is untouched")
	}

	// about three in four of many hits cast
	u := cursedUnit(40, true)
	casts := 0

	for i := 0; i < 400; i++ {
		d.curses = nil
		d.cursedHit(u, hero)

		if d.cursedStats(hero) != nil {
			casts++
		}
	}

	if casts < 250 || casts > 350 {
		t.Errorf("%d casts of 400", casts)
	}
}

// The timer of modifier 41: due every 75 frames, wakes a neutral monster two frames ahead, skips a dead one.
func TestUModTimerTick(t *testing.T) {
	d := testDirector()
	d.frame = 500

	m := fxTestMonster()
	m.UMod.TimerEvent = true
	m.UMod.TimerAt = 575

	b := d2monster.NewBrain(1, 1, d2monster.Normal, &d2monster.Profile{}, 5)
	b.Wake = 9999
	u := &unit{m: m, b: b}

	d.umodTimer(u)

	if b.Wake != 9999 || d.Counters.UModTimers != 0 {
		t.Fatal("not due yet")
	}

	d.frame = 575
	d.umodTimer(u)

	if m.UMod.TimerAt != 650 || d.Counters.UModTimers != 1 {
		t.Errorf("next tick %d count %d", m.UMod.TimerAt, d.Counters.UModTimers)
	}

	// the monster is in mode neutral by default: the think event is re-armed two frames ahead
	if b.Wake != 577 {
		t.Errorf("wake %d, want 577", b.Wake)
	}

	// without the modifier nothing happens
	m2 := fxTestMonster()
	d.umodTimer(&unit{m: m2, b: b})

	if d.Counters.UModTimers != 1 {
		t.Error("a monster without the timer event must not tick")
	}
}

func TestRareNameKeysMapsRows(t *testing.T) {
	pre := []string{"p0", "p1", "p2", "p3", "p4"}
	suf := []string{"s0", "s1", "s2", "s3"}
	app := []string{"a0", "a1", "a2"}

	longs := 0

	for seed := 0; seed < 300; seed++ {
		k := rareNameKeys(uint16(seed), pre, suf, app)
		if k.Prefix == "" || k.Suffix == "" {
			t.Fatalf("seed %d: %+v", seed, k)
		}

		if (k.Appellation != "") != (k.Format == 0x6ba) {
			t.Fatalf("seed %d: format %#x appellation %q", seed, k.Format, k.Appellation)
		}

		if k.Format == 0x6ba {
			longs++
		}
	}

	if longs < 100 || longs > 200 {
		t.Errorf("%d long names of 300, want about half", longs)
	}

	// without an appellation table the long form falls back to the short one
	if k := rareNameKeys(1, pre, suf, nil); k.Format != 0x6b9 || k.Appellation != "" {
		t.Errorf("no appellations: %+v", k)
	}
}

// Modifier 1 steps the unit seed and keeps its low 16 bits as the name seed.
func TestNameSeedAdvances(t *testing.T) {
	m := fxTestMonster()
	m.Brain = d2monster.NewBrain(1, 1, d2monster.Normal, &d2monster.Profile{}, 77)
	u := (&Director{}).newFxUnit(m)

	ref := *m.Brain.Seed
	want := uint16(ref.Step())

	u.AdvanceNameSeed()

	if m.UMod.NameSeed != want || u.NameSeed() != want {
		t.Errorf("name seed %#x, want %#x", m.UMod.NameSeed, want)
	}

	if *m.Brain.Seed != ref {
		t.Error("the unit seed must have taken exactly one step")
	}
}
