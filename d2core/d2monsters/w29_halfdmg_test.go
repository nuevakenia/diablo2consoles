package d2monsters

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func TestHalfDamageIsWillowispBase(t *testing.T) {
	mk := func(base string) *fxUnit {
		return &fxUnit{m: &d2mapentity.Monster{Stat: &d2records.MonStatRecord{BaseKey: base}}}
	}

	if !mk("willowisp1").HalfDamage() || !mk("WillOWisp1").HalfDamage() {
		t.Error("wisp family must halve the extra strong damage bonus")
	}

	if mk("fallen1").HalfDamage() || (&fxUnit{m: &d2mapentity.Monster{}}).HalfDamage() {
		t.Error("other classes keep the bonus")
	}
}
