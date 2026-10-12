package d2state

import (
	"reflect"
	"testing"
)

func TestRemoveAuraStates(t *testing.T) {
	s := New()
	s.SetDefs(Defs{"might": {Name: "might", Aura: true}, "holyfire": {Name: "holyfire", Aura: true},
		"conversion": {Name: "conversion", Aura: true}, "stun": {Name: "stun"}})

	for _, n := range []string{"might", "holyfire", "conversion", "stun"} {
		s.Apply(1, Instance{Name: n})
	}

	got := s.RemoveAuraStates("conversion")
	if !reflect.DeepEqual(got, []string{"holyfire", "might"}) {
		t.Fatalf("removed %v", got)
	}

	if !s.Active(1, "conversion") || !s.Active(1, "stun") || s.Active(1, "might") {
		t.Fatal("wrong survivors")
	}
}
