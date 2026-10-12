package d2monster

import "testing"

func TestPickUMonClass(t *testing.T) {
	var asked []int

	roll := func(n int) int { asked = append(asked, n); return n - 1 }

	got, ok := PickUMonClass(roll, []string{"a", "", "b", "c", ""})
	if !ok || got != "c" || len(asked) != 1 || asked[0] != 3 {
		t.Fatalf("got %q %v draws %v", got, ok, asked)
	}

	if _, ok := PickUMonClass(roll, []string{"", ""}); ok {
		t.Fatal("empty list must report false")
	}
}
