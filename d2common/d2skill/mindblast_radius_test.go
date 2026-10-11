package d2skill

import "testing"

// SRVDO_051 takes its radius from SRVDO_EvalSkillCalcOrStateValue (prgcalc1)
// and only falls back to aurarangecalc when that is 0.
func TestMindBlastRadiusPrgCalcFirst(t *testing.T) {
	for _, tc := range []struct {
		prg  string
		want int
	}{{"", 1}, {"12", 2}, {"3", 0}} {
		cf := nextFixture(map[string]int{"Mind Blast": 10})
		cf.reg.ByName("Mind Blast").PrgCalc[1] = row{"p": tc.prg}.calc("p")
		cf.addFoe("near", 5, 0)
		cf.addFoe("far", 9, 0)
		cf.u.roller = &seq{vals: []uint32{99, 99}}
		id := cf.id("Mind Blast")
		tg := Target{X: 0, Y: 0}
		cf.p.Start(cf.u, id, tg)
		r := cf.p.Do(cf.u, id, tg)

		hits := 0

		for _, e := range r.Effects {
			if e.Kind == "hit_unit" {
				hits++
			}
		}

		if hits != tc.want {
			t.Errorf("prgcalc1 %q: %d hits, want %d", tc.prg, hits, tc.want)
		}
	}
}
