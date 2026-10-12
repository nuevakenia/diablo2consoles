package d2monster

import "testing"

func TestDetachFromLeader(t *testing.T) {
	l, a, b := &Brain{}, &Brain{}, &Brain{}
	l.AddMinion(a)
	l.AddMinion(b)

	if !a.DetachFromLeader() || a.Leader != nil || len(l.Minions) != 1 {
		t.Fatal("follower detach")
	}

	if !l.DetachFromLeader() || b.Leader != nil || len(l.Minions) != 0 {
		t.Fatal("leader release")
	}

	if l.DetachFromLeader() {
		t.Fatal("lone unit")
	}
}
