package d2skill

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2missile"
)

// Glacial Spike (hit function 13, 0x5a8360, read in Ghidra): the area hit uses
// sHitPar1 else aurarangecalc as its radius, and sHitPar2 else auralencalc as
// the elemental (chill) length of the damage it carries.
func TestGlacialSpikeHit13(t *testing.T) {
	for _, tc := range []struct {
		name          string
		par           [3]int
		lvl           int
		wantR, wantLn int
	}{
		{"table radius from aurarangecalc", [3]int{}, 1, 4, 100},
		{"per-level aurarangecalc", [3]int{}, 5, 8, 100},
		{"sHitPar1 wins", [3]int{6}, 5, 6, 100},
		{"sHitPar2 length wins", [3]int{0, 33}, 1, 4, 33},
	} {
		f := newFixture(map[string]int{"Gl": tc.lvl})
		f.reg.Add(&Skill{ID: 800, Name: "Gl", SrvMissile: "glacialspike", ManaShift: 8,
			DamageSpec:    DamageSpec{EType: "cold", EMin: 1, EMax: 1},
			AuraRangeCalc: d2calc.Compile("ln12", d2calc.KindSkill),
			AuraLenCalc:   d2calc.Compile("100", d2calc.KindSkill),
			Params:        [9]int{0, 4, 1}})
		f.u.levels["Gl"] = tc.lvl
		f.p.Missiles.(missileTable)["glacialspike"] = &d2missile.Spec{ID: 801, Name: "glacialspike", SrvDoFunc: 1,
			SrvHitFunc: 13, Vel: 16, MaxVel: 16, Range: 40, CollideType: 3, CollideKill: true, SHitPar: tc.par}
		f.sim.Table = f.p.Missiles

		var radii, lens []int

		f.sim.OnEvent = func(e d2missile.Event) {
			if e.Kind == d2missile.EventArea {
				radii = append(radii, e.Radius)
				lens = append(lens, int(e.Damage.ColdLen))
			}
		}

		if st, r := f.cast("Gl", 60, 0); !st.OK || !r.OK || len(r.Missiles) != 1 {
			t.Fatalf("%s: cast %+v %+v", tc.name, st, r)
		}

		for i := 0; i < 60; i++ {
			f.w.frame++
			f.sim.Step()
		}

		if len(radii) != 1 || radii[0] != tc.wantR || lens[0] != tc.wantLn {
			t.Errorf("%s: area radii %v lens %v, want [%d] [%d]", tc.name, radii, lens, tc.wantR, tc.wantLn)
		}
	}
}
