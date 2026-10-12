package d2monster

import "testing"

func TestW28UniqueVariantRoll(t *testing.T) {
	seq := func(v ...int) (func(int) int, *int) {
		i := 0

		return func(int) int { x := v[i%len(v)]; i++; return x }, &i
	}

	r, n := seq(5)
	if !UniqueVariantRoll(r, 0, 0, 2, 0, 0) || *n != 1 { // 5 < 6 below the maximum
		t.Fatalf("6%% branch: draws %d", *n)
	}

	r, n = seq(6)
	if UniqueVariantRoll(r, 0, 0, 2, 0, 0) || *n != 2 { // 6 is not below 6; the last draw is consumed
		t.Fatalf("no unique: draws %d", *n)
	}

	r, n = seq(99)
	if UniqueVariantRoll(r, 2, 0, 2, 0, 0) || *n != 1 { // at the maximum only the last draw
		t.Fatalf("at max: draws %d", *n)
	}

	r, n = seq(49, 99)
	if !UniqueVariantRoll(r, 0, 1, 2, 5, 10) || *n != 1 { // ramp 5*100/10 = 50 > 49
		t.Fatalf("ramp: draws %d", *n)
	}

	r, _ = seq(50, 99, 99)
	if UniqueVariantRoll(r, 0, 1, 1, 5, 10) {
		t.Fatal("ramp miss then 6% miss")
	}
}
