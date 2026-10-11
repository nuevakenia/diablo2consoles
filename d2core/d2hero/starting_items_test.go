package d2hero

import "testing"

func TestIsPotion(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"hp1", true}, {"hp5", true}, {"mp3", true}, {"rvs", true}, {"rvl", true},
		{"hp6", false}, {"hp", false}, {"tsc", false}, {"isc", false}, {"sst", false}, {"", false},
	}

	for _, tc := range tests {
		if got := isPotion(tc.code); got != tc.want {
			t.Errorf("isPotion(%q) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestBodySlotLocation(t *testing.T) {
	tests := []struct {
		loc  string
		want bool
	}{
		{"rarm", true}, {"larm", true}, {"tors", true}, {"RARM", true}, {" belt ", true},
		{"", false}, {"stor", false}, {"inv", false},
	}

	for _, tc := range tests {
		if got := bodySlotLocation(tc.loc); got != tc.want {
			t.Errorf("bodySlotLocation(%q) = %v, want %v", tc.loc, got, tc.want)
		}
	}
}

func TestStarterBodyItems(t *testing.T) {
	// the Paladin row of CharStats.txt: sword and buckler worn, potions and scrolls not
	codes := []string{"ssd", "buc", "hp1", "tsc", "isc", "0"}
	locs := []string{"rarm", "larm", "", "", "", ""}

	got := starterBodyItems(codes, locs)
	want := []starterBodyItem{{"ssd", 4}, {"buc", 5}}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d: got %v, want %v", i, got[i], want[i])
		}
	}

	if got := starterBodyItems([]string{"sst", "hp1"}, []string{"rarm"}); len(got) != 1 || got[0].Code != "sst" {
		t.Errorf("a short location list must not panic or invent items: %v", got)
	}
}
