package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
)

func init() {
	// patch_d2 skills.txt: Vine Attack (294), Plague Poppy (222), Cycle of Life (231), CorpseCycler (307), VineCycler (325).
	rows = append(rows,
		row{"skill": "Vine Attack", "Id": "740", "srvdofunc": "130", "srvmissilea": "plague vines", "calc1": "min(24,ln12)",
			"Param1": "3", "Param2": "1", "HitShift": "8", "manashift": "8"},
		row{"skill": "Poppy9", "Id": "741", "charclass": "dru", "srvdofunc": "115", "summon": "plaguepoppy", "pettype": "vine",
			"petmax": "1", "summode": "S1", "sumskill1": "Vine Attack", "sumsk1calc": "lvl", "calc1": "par3 * (lvl-1)",
			"calc2": "ulvl * 3 / 4 + lvl", "Param1": "3", "Param2": "1", "Param3": "25", "EType": "pois", "EMin": "12", "EMax": "16",
			"HitShift": "8", "manashift": "8"},
		row{"skill": "CorpseCycler9", "Id": "742", "srvdofunc": "999", "srvmissilea": "recycler delay", "calc1": "dm12",
			"Param1": "3", "Param2": "12", "aurarangecalc": "10", "HitShift": "8", "manashift": "8"},
		row{"skill": "VineCycler9", "Id": "743", "srvdofunc": "999", "srvmissilea": "vine recycler delay", "calc1": "dm12",
			"Param1": "1", "Param2": "8", "aurarangecalc": "10", "HitShift": "8", "manashift": "8"},
	)
}

func vineFixture(levels map[string]int) *classFixture {
	cf := newClassFixture(levels)

	tab := cf.p.Missiles.(missileTable)
	tab["plague vines"] = &d2missile.Spec{ID: 474, Name: "plague vines", SrvDoFunc: 26, Vel: 7, MaxVel: 7, Range: 20,
		Param: [5]int{9}, SubMissile: [3]string{"plague vines trail"}, CanSlow: true}
	tab["plague vines trail"] = &d2missile.Spec{ID: 475, Name: "plague vines trail", SrvDoFunc: 1, SrvHitFunc: 50, Range: 100,
		LevRange: 25, CollideType: 3, LastCollide: true, Size: 3, AlwaysExplode: true, SHitPar: [3]int{15}}
	tab["recycler delay"] = &d2missile.Spec{ID: 498, Name: "recycler delay", SrvDoFunc: 29, Range: 47, Param: [5]int{45}}
	tab["vine recycler delay"] = &d2missile.Spec{ID: 540, Name: "vine recycler delay", SrvDoFunc: 33, Range: 47, Param: [5]int{45}}

	return cf
}

// SRVDO_130 0x5d02d0 + SRVDO_VineAttack_CreateMissile 0x5d0220: calc1 vines (min(24, ln12)) start on the target and
// head one step along (0,1), (0,-1), (1,0), (-1,0) by index & 3, all on path type 10.
func TestVineAttackMakesCalc1VinesUnderTheTarget(t *testing.T) {
	for _, tc := range []struct{ lvl, want int }{{1, 3}, {2, 4}, {6, 8}, {40, 24}} {
		cf := vineFixture(map[string]int{"Vine Attack": tc.lvl})
		_, r := cf.cast("Vine Attack", 30, 20)

		if len(r.Missiles) != tc.want {
			t.Fatalf("level %d: %d vines, want %d", tc.lvl, len(r.Missiles), tc.want)
		}

		for i, m := range r.Missiles {
			if int(m.X) != 30 || int(m.Y) != 20 {
				t.Errorf("level %d vine %d starts at %v,%v, not on the target", tc.lvl, i, m.X, m.Y)
			}

			if !m.OnWalkPath() {
				t.Errorf("level %d vine %d is not on path type 10", tc.lvl, i)
			}
		}
	}

	cf := vineFixture(map[string]int{"Vine Attack": 2})
	_, r := cf.cast("Vine Attack", 30, 20)
	want := [][2]float64{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	for i, m := range r.Missiles {
		if m.DX != want[i][0] || m.DY != want[i][1] {
			t.Errorf("vine %d heads %v,%v, want %v", i, m.DX, m.DY, want[i])
		}
	}
}

// CastTrap on a vine missile (SrvDoFunc 26): the vines come up under the target, calc1 of Vine Attack at the summoning
// skill's level, instead of one missile flying from the vine.
func TestTrapVinesComeUpUnderTheTarget(t *testing.T) {
	cf := vineFixture(map[string]int{"Poppy9": 3, "Vine Attack": 1})
	m := cf.p.CastTrap(cf.u, cf.id("Poppy9"), "plague vines", 10, 10, Target{X: 40, Y: 25})

	if m == nil {
		t.Fatal("no vine")
	}

	n := 0

	for _, mm := range cf.sim.Missiles() {
		if mm.Spec.Name != "plague vines" {
			continue
		}

		n++

		if int(mm.X) != 40 || int(mm.Y) != 25 {
			t.Errorf("vine at %v,%v", mm.X, mm.Y)
		}
	}

	// Poppy9 at level 3: Vine Attack's calc1 at level 3 = 3 + 2 = 5
	if n != 5 {
		t.Errorf("%d vines, want 5", n)
	}
}

// SRVST_063 + SrvDoFunc 29 / 33: the recycler delay missile is a stationary one at the hero carrying calc1 (dm12 over
// the cycler's Param1 / Param2) as the percent to give back.
func TestCyclerMissileCarriesDm12(t *testing.T) {
	for _, tc := range []struct {
		skill    string
		lvl      int
		lo, hi   int
		wantFunc int
	}{
		{"CorpseCycler9", 1, 3, 12, 29},
		{"CorpseCycler9", 20, 3, 12, 29},
		{"VineCycler9", 1, 1, 8, 33},
		{"VineCycler9", 20, 1, 8, 33},
	} {
		cf := vineFixture(map[string]int{})
		m := cf.p.CastCycler(cf.u, tc.skill, tc.lvl)

		if m == nil {
			t.Fatalf("%s: no missile", tc.skill)
		}

		if m.Spec.SrvDoFunc != tc.wantFunc || m.Velocity != 0 {
			t.Errorf("%s: do %d velocity %d", tc.skill, m.Spec.SrvDoFunc, m.Velocity)
		}

		// dm12 diminishing curve (skills-2.md): ((110*lvl)*(b-a))/(100*(lvl+6)) + a
		want := ((110*tc.lvl)*(tc.hi-tc.lo))/(100*(tc.lvl+6)) + tc.lo
		if m.RestorePct != want {
			t.Errorf("%s level %d: restores %d percent, want %d", tc.skill, tc.lvl, m.RestorePct, want)
		}
	}

	if vineFixture(map[string]int{}).p.CastCycler(nil, "no such skill", 1) != nil {
		t.Error("an unknown cycler casts nothing")
	}
}
