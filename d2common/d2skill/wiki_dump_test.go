package d2skill

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// wikiEnv is the calc context of a skill at an effective level; base holds the hard points (synergies read them).
func wikiEnv(reg *Registry, name string, eff int, base map[string]int) (*Skill, *Env) {
	lv := map[string]int{}
	for k, v := range base {
		lv[k] = v
	}

	lv[name] = eff
	u := newUnit(reg, lv)
	u.base = base

	sk := reg.ByName(name)

	return sk, NewEnv(sk, eff, u, reg)
}

// TestWikiDump writes the engine values of the four classes to WIKI_DUMP (a TSV for the cross check notes).
func TestWikiDump(t *testing.T) {
	out := os.Getenv("WIKI_DUMP")
	if out == "" {
		t.Skip("WIKI_DUMP not set")
	}

	reg := loadRealRegistry(t)

	var b strings.Builder

	f := func(v int32) string { return fmt.Sprintf("%.2f", float64(v)/256) }

	for _, cls := range []string{"ama", "bar", "pal", "nec"} {
		var names []string

		for _, id := range reg.IDs() {
			if sk := reg.ByID(id); sk.CharClass == cls {
				names = append(names, sk.Name)
			}
		}

		sort.Strings(names)

		for _, name := range names {
			for _, syn := range []bool{false, true} {
				// effective level; hard points are eff-3 for the three "+3 skills" columns
				for _, lvl := range []int{1, 10, 20, 4, 13, 23} {
					base := map[string]int{}

					if syn {
						for _, n := range names {
							base[n] = 20
						}
					}

					base[name] = lvl
					if lvl == 4 || lvl == 13 || lvl == 23 {
						base[name] = lvl - 3
					}

					sk, e := wikiEnv(reg, name, lvl, base)
					d := sk.Descriptor(e, lvl, 0, 0, 0)
					fmt.Fprintf(&b, "%s\t%s\tsyn=%v\tL%d\tmana=%s\tphys=%s-%s\tsrc=%d\telem=%s:%s-%s len=%d\tth=%d\tcalc=%d,%d,%d,%d\taura=%d,%d\tpet=%d\tpas=", cls, name, syn, lvl,
						f(int32(sk.ManaCost(lvl))), f(d.PhysMin), f(d.PhysMax), sk.SrcDam, sk.EType, f(sk.ElemMin(e, lvl)), f(sk.ElemMax(e, lvl)), sk.ElemLen(e, lvl),
						sk.ToHitBonus(e, lvl), e.eval(sk.Calc[1]), e.eval(sk.Calc[2]), e.eval(sk.Calc[3]), e.eval(sk.Calc[4]),
						e.eval(sk.AuraLenCalc), e.eval(sk.AuraRangeCalc), e.eval(sk.PetMax))

					for i := 1; i <= 5; i++ {
						if sk.PassiveStat[i] != "" {
							fmt.Fprintf(&b, "%s=%d;", sk.PassiveStat[i], e.eval(sk.PassiveCalc[i]))
						}
					}

					b.WriteString("\taur=")

					for i := 1; i <= 6; i++ {
						if sk.AuraStat[i] != "" {
							fmt.Fprintf(&b, "%s=%d;", sk.AuraStat[i], e.eval(sk.AuraStatCalc[i]))
						}
					}

					b.WriteString("\n")
				}
			}
		}
	}

	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
