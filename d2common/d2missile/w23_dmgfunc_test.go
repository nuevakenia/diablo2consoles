package d2missile

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2combat"
)

// SrvDmgFunc 4 (0x5aadb0, Ice Blast): chill length becomes freeze length.
func TestDmgFunc4IceBlastFreezes(t *testing.T) {
	s := NewSim(newWorld(), specs{})
	m := &Missile{Spec: &Spec{Name: "iceblast", SrvDmgFunc: 4}}
	d := d2combat.Damage{ColdLen: 170}

	s.dmgFunc(m, nil, &d)

	if d.FreezeLen != 170 || d.ColdLen != 0 {
		t.Fatalf("freeze %d chill %d", d.FreezeLen, d.ColdLen)
	}
}

// SrvDmgFunc 10 (0x5aadd0): the freeze length is a copy, the chill stays.
func TestDmgFunc10CopiesChill(t *testing.T) {
	s := NewSim(newWorld(), specs{})
	m := &Missile{Spec: &Spec{Name: "bladesoficecubes", SrvDmgFunc: 10}}
	d := d2combat.Damage{ColdLen: 60}

	s.dmgFunc(m, nil, &d)

	if d.FreezeLen != 60 || d.ColdLen != 60 {
		t.Fatalf("freeze %d chill %d", d.FreezeLen, d.ColdLen)
	}
}
