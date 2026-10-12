package d2combat

import "testing"

type zeroRoller struct{}

func (zeroRoller) Roll(int32) uint32 { return 0 }

func TestConvertPhysical(t *testing.T) {
	for _, c := range []struct {
		class byte
		pct   int32
		want  Damage
	}{
		{3, 100, Damage{Magic: 1000}},
		{3, 30, Damage{Physical: 700, Magic: 300}},
		{4, 50, Damage{Physical: 500, Cold: 500, ColdLen: 50}},
		{5, 100, Damage{Poison: 125, PoisonLen: 50}},
		{11, 100, Damage{Fire: 1000, BurnLen: 50}},
		{0, 100, Damage{Physical: 1000}},
	} {
		d := Damage{Physical: 1000}
		ConvertPhysical(&d, zeroRoller{}, c.class, c.pct)

		if d.Physical != c.want.Physical || d.Magic != c.want.Magic || d.Cold != c.want.Cold || d.Fire != c.want.Fire ||
			d.Poison != c.want.Poison || d.ColdLen != c.want.ColdLen || d.PoisonLen != c.want.PoisonLen ||
			d.BurnLen != c.want.BurnLen {
			t.Errorf("class %d pct %d: %+v, want %+v", c.class, c.pct, d, c.want)
		}
	}
}
