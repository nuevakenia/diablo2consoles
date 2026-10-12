package d2skill

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestW30WikiDump writes a table of engine values (mana, damage, calcs) for
// the sorceress, druid and assassin skills at levels 1, 10, 20 into the file
// named by W30_DUMP. It asserts nothing; the pinned values live in
// w30_wiki_test.go. Skipped without D2_TABLES and W30_DUMP.
func TestW30WikiDump(t *testing.T) {
	out := os.Getenv("W30_DUMP")
	if out == "" {
		t.Skip("W30_DUMP not set")
	}

	reg := loadRealRegistry(t)

	var b strings.Builder

	for _, id := range reg.IDs() {
		sk := reg.ByID(id)
		if sk.CharClass != "sor" && sk.CharClass != "dru" && sk.CharClass != "ass" && id != 54 && (id < 217 || id > 220) && (id < 294 || id > 298) {
			continue
		}

		for _, lvl := range []int{1, 10, 20} {
			e := realEnv(reg, sk.Name, lvl, nil)
			fmt.Fprintf(&b, "%s\t%d\tmana=%d\tetype=%s\temin=%d\temax=%d\telen=%d\tpmin=%d\tpmax=%d\tdelay=%d\t",
				sk.Name, lvl, sk.ManaCost(lvl), sk.EType, sk.ElemMin(e, lvl),
				sk.ElemMax(e, lvl), sk.ElemLen(e, lvl), sk.PhysMin(e, lvl, 0, true),
				sk.PhysMax(e, lvl, 0, true), e.Eval(sk.Delay))

			for i := 1; i <= 4; i++ {
				fmt.Fprintf(&b, "c%d=%d(%s)\t", i, e.Eval(sk.Calc[i]), sk.Calc[i].Source())
			}

			for i := 1; i <= 5; i++ {
				if sk.PassiveStat[i] != "" {
					fmt.Fprintf(&b, "pas%d=%s:%d\t", i, sk.PassiveStat[i], e.Eval(sk.PassiveCalc[i]))
				}
			}

			for i := 1; i <= 6; i++ {
				if sk.AuraStat[i] != "" {
					fmt.Fprintf(&b, "aura%d=%s:%d\t", i, sk.AuraStat[i], e.Eval(sk.AuraStatCalc[i]))
				}
			}

			if !sk.AuraRangeCalc.Empty() {
				fmt.Fprintf(&b, "range=%d\t", e.Eval(sk.AuraRangeCalc))
			}

			if !sk.PetMax.Empty() {
				fmt.Fprintf(&b, "petmax=%d\t", e.Eval(sk.PetMax))
			}

			fmt.Fprintf(&b, "par=%v\tmis=%s/%s/%s/%s\tsum=%s\n", sk.Params[1:], sk.SrvMissile, sk.SrvMissileA, sk.SrvMissileB, sk.SrvMissileC, sk.Summon)
		}
	}

	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
