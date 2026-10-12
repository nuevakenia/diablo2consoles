package d2party

import "testing"

// A declaration against a member of a party makes every member of that party
// (level 9 or more) mutually hostile with the declarer (0x5a37f0 calling 0x5a3790).
func TestDeclareHostileAgainstPartyAll(t *testing.T) {
	r := afield()
	r.SetLevel("c", 20)

	if err := r.Invite("b", "c"); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Accept("c"); err != nil {
		t.Fatal(err)
	}

	if _, err := r.DeclareHostile("a", "b"); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"b", "c"} {
		if !r.Hostile("a", id) || !r.Hostile(id, "a") {
			t.Fatalf("%s must be mutually hostile with the declarer", id)
		}
	}
}
