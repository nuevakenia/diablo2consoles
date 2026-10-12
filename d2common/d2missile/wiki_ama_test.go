package d2missile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2calc"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
)

// Wave 30 wiki cross check, Amazon: the physical-to-element conversion percent of the three elemental arrows
// (SrvDmgFunc 1, DmgCalc1 = dl12) and the Ice Arrow freeze factor, read from the real patch_d2 missiles.txt.
// Wiki (Amazon Basin, LoD): Magic Arrow 1 + (lvl - 1) percent = 1 / 10 / 20, Fire and Cold Arrow 1 + 2 lvl
// = 3 / 21 / 41 percent. The d2emu page lists 5 / 23 / 43 for Magic Arrow; that is not the table (wiki differs).
func TestWikiAmazonArrowConversion(t *testing.T) {
	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	b, err := os.ReadFile(filepath.Join(dir, "skills", "patch_d2", "missiles.txt"))
	if err != nil {
		t.Skip(err)
	}

	d := d2txt.LoadDataDictionary(b)
	specs := map[string]*Spec{}

	for d.Next() {
		sp := &Spec{Name: d.String("Missile"), SrvDmgFunc: d.Number("pSrvDmgFunc"), EType: d.String("EType"),
			DParam: [2]int{d.Number("dParam1"), d.Number("dParam2")}}
		if c := d.String("DmgCalc1"); c != "" {
			sp.DmgCalc1 = d2calc.Compile(c, d2calc.KindMissile)
		}

		specs[sp.Name] = sp
	}

	for name, want := range map[string]map[int]int{
		"magicarrow": {1: 1, 10: 10, 20: 20, 30: 30},
		"firearrow":  {1: 3, 10: 21, 20: 41, 30: 61, 50: 100},
		"coldarrow":  {1: 3, 10: 21, 20: 41, 30: 61, 50: 100},
	} {
		sp := specs[name]
		if sp == nil || sp.SrvDmgFunc != 1 {
			t.Fatalf("%s: %+v", name, sp)
		}

		for lvl, pct := range want {
			if got := convertPercent(&Missile{Spec: sp, Level: lvl}); got != pct {
				t.Errorf("%s L%d converts %d percent, wiki %d", name, lvl, got, pct)
			}
		}
	}

	// Ice Arrow: SrvDmgFunc 2, the freeze length is the cold length times dParam1 = 100 percent
	if ia := specs["icearrow"]; ia == nil || ia.SrvDmgFunc != 2 || ia.DParam[0] != 100 {
		t.Errorf("icearrow %+v", ia)
	}
}
