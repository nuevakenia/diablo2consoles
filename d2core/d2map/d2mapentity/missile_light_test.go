package d2mapentity

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

func newLightMissile(id string, light, flicker int) *Missile {
	rec := &d2records.MissileRecord{Light: d2records.MissileLight{Diameter: light, Flicker: flicker, Red: 255, Green: 128}}

	return &Missile{AnimatedEntity: &AnimatedEntity{mapEntity: mapEntity{uuid: id}}, record: rec}
}

func TestMissileLightFlicker(t *testing.T) {
	a := newLightMissile("a", 4, 4)
	b := newLightMissile("b", 4, 4)

	ls, ok := a.LightSource()
	if !ok || ls.Radius != 4 || ls.R != 255 || ls.G != 128 {
		t.Fatalf("before the first tick: %+v %v", ls, ok)
	}

	same, differ := 0, 0

	for i := 0; i < 60; i++ {
		a.advanceLight(0.04)
		b.advanceLight(0.04)

		ra, _ := a.LightSource()
		rb, _ := b.LightSource()

		if ra.Radius < 4 || ra.Radius > 8 {
			t.Fatalf("tick %d: radius %d out of [4,8]", i, ra.Radius)
		}

		if ra.Radius == rb.Radius {
			same++
		} else {
			differ++
		}
	}

	if differ == 0 {
		t.Error("two missiles with different ids flicker in step")
	}
}

func TestMissileLightNoFlickerAndNone(t *testing.T) {
	c := newLightMissile("c", 3, 0)
	for i := 0; i < 20; i++ {
		c.advanceLight(0.04)
	}

	if ls, ok := c.LightSource(); !ok || ls.Radius != 3 {
		t.Errorf("constant light moved: %+v %v", ls, ok)
	}

	if _, ok := newLightMissile("d", 0, 3).LightSource(); ok {
		t.Error("Light 0 must not light")
	}

	// a long frame runs at most the catch-up ticks
	e := newLightMissile("e", 2, 8)
	e.advanceLight(10)

	if e.flicker.Cur > (2+maxLightCatchUp)*8 {
		t.Errorf("catch-up ramp too large: %d", e.flicker.Cur)
	}
}
