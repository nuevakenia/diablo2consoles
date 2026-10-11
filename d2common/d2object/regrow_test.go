package d2object

import (
	"reflect"
	"testing"
)

func TestRegrow(t *testing.T) {
	cases := []struct {
		name                                    string
		saved, max, savedF, nowF, period, parm2 int
		charges, mode                           int
		refills                                 []int
	}{
		{"empty, long absence gains one", 0, 2, 100, 100000, 500, 1, 1, 1, []int{500}},
		{"empty, short absence gains none", 0, 2, 100, 300, 500, 1, 0, 2, []int{500, 1000}},
		{"one left, long absence fills", 1, 2, 0, 90000, 500, 1, 2, 0, nil},
		{"full stays full", 2, 2, 0, 10, 500, 1, 2, 0, nil},
	}

	for _, c := range cases {
		r := Regrow(c.saved, c.max, c.savedF, c.nowF, c.period, c.parm2)
		if !r.OK || r.Charges != c.charges || r.Mode != c.mode || !reflect.DeepEqual(r.RefillAfter, c.refills) {
			t.Errorf("%s: got %+v", c.name, r)
		}
	}

	if Regrow(0, 2, 0, 10, 0, 1).OK {
		t.Error("period 0 must not restore")
	}
}
