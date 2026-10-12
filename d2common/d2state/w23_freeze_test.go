package d2state

import "testing"

func TestBossFreezeAsChill(t *testing.T) {
	h := BossFreezeAsChill(Hit{FreezeLen: 40, ColdLen: 10})
	if h.FreezeLen != 0 || h.ColdLen != 40 {
		t.Fatalf("%+v", h)
	}

	// a longer chill is kept
	if h := BossFreezeAsChill(Hit{FreezeLen: 5, ColdLen: 30}); h.ColdLen != 30 || h.FreezeLen != 0 {
		t.Fatalf("%+v", h)
	}

	// no freeze: nothing changes
	if h := BossFreezeAsChill(Hit{ColdLen: 7, StunLen: 3}); h.ColdLen != 7 || h.StunLen != 3 {
		t.Fatalf("%+v", h)
	}

	// the boss then gets a chill, not a freeze, through ApplyHit
	s := New()
	out := s.ApplyHit(0, BossFreezeAsChill(Hit{FreezeLen: 40}))
	if len(out) != 1 || out[0] != Chill {
		t.Fatalf("applied %v", out)
	}
}
