package d2state

import (
	"reflect"
	"sort"
	"testing"
)

func TestCure(t *testing.T) {
	s := New()
	s.Apply(0, Instance{Name: "freeze", Until: 100})
	s.Apply(0, Instance{Name: "amplifydamage", Until: 100}) // curable
	s.Apply(0, Instance{Name: "battleorders", Until: 100})  // not curable
	s.AddStream(0, "poison", 256, 50, "mon", 0)
	s.AddStream(0, "burn", 256, 50, "mon", 0)

	got := s.Cure(10, func(n string) bool { return n == "amplifydamage" })
	sort.Strings(got)

	if !reflect.DeepEqual(got, []string{"amplifydamage", "freeze"}) {
		t.Fatalf("removed %v", got)
	}

	if s.Active(10, "freeze") || s.Active(10, "amplifydamage") || !s.Active(10, "battleorders") {
		t.Errorf("states left: %v", s.Names(10))
	}

	// the poison stream ends, the burn stream stays
	kinds := []string{}
	for _, st := range s.Streams(10) {
		kinds = append(kinds, st.Kind)
	}

	if !reflect.DeepEqual(kinds, []string{"burn"}) {
		t.Errorf("streams left: %v", kinds)
	}
}

func TestCurePoisonStreamOnly(t *testing.T) {
	s := New()
	s.AddStream(0, "poison", 100, 20, "mon", 0)

	if got := s.Cure(5, nil); !reflect.DeepEqual(got, []string{"poison"}) {
		t.Fatalf("removed %v", got)
	}

	if len(s.Streams(5)) != 0 {
		t.Error("poison stream survived")
	}

	// nothing to cure
	if got := s.Cure(5, nil); len(got) != 0 {
		t.Errorf("removed %v from a healthy unit", got)
	}
}
