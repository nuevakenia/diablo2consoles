package d2skills

import "testing"

func TestLandedIsPerSkill(t *testing.T) {
	e := &Engine{}

	if e.Landed("Twister") != 0 {
		t.Fatal("fresh engine")
	}

	e.noteLanded("Twister", 2)
	e.noteLanded("Molten Boulder", 1)
	e.noteLanded("Twister", 0)
	e.noteLanded("", 5)
	e.noteLanded("Twister", -1)

	if e.Landed("Twister") != 2 || e.Landed("Molten Boulder") != 1 {
		t.Errorf("twister %d boulder %d", e.Landed("Twister"), e.Landed("Molten Boulder"))
	}
}
