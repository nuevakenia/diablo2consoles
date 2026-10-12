package d2party

import "testing"

func addOf(s GoldSplit, id string) int {
	for _, x := range s.Shares {
		if x.ID == id {
			return x.Add
		}
	}

	return -1
}

func TestShareGold(t *testing.T) {
	big := Purse{Have: 0, Cap: 100000}
	tests := []struct {
		name     string
		amount   int
		purses   map[string]Purse
		body     string // member that is a body
		pickerB  bool
		dropped  bool
		wantA    int
		wantB    int
		wantC    int
		wantOver int
	}{
		{name: "three way split, remainder to picker", amount: 100,
			purses: map[string]Purse{"a": big, "b": big, "c": big}, wantA: 34, wantB: 33, wantC: 33},
		{name: "member over its limit hands the rest back", amount: 90,
			purses: map[string]Purse{"a": big, "b": {Have: 9990, Cap: 10000}, "c": big},
			wantA:  50, wantB: 10, wantC: 30},
		{name: "picker over its limit spills", amount: 90,
			purses: map[string]Purse{"a": {Have: 0, Cap: 20}, "b": big, "c": big},
			wantA:  20, wantB: 30, wantC: 30, wantOver: 10},
		{name: "dead member is not counted but still paid (the picker keeps the rest: 0, as in the exe)", amount: 60, body: "c",
			purses: map[string]Purse{"a": big, "b": big, "c": big}, wantA: 0, wantB: 30, wantC: 30},
		{name: "dead picker takes all", amount: 60, pickerB: true,
			purses: map[string]Purse{"a": big, "b": big, "c": big}, wantA: 60, wantB: -1, wantC: -1},
		{name: "player dropped gold bypasses the party", amount: 60, dropped: true,
			purses: map[string]Purse{"a": big, "b": big, "c": big}, wantA: 60, wantB: -1, wantC: -1},
		{name: "member without a purse is skipped (n still counts it)", amount: 90,
			purses: map[string]Purse{"a": big, "b": big}, wantA: 60, wantB: 30, wantC: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := partyOf(t)
			if err := r.Invite("a", "c"); err != nil {
				t.Fatal(err)
			}

			if _, err := r.Accept("c"); err != nil {
				t.Fatal(err)
			}

			if tc.body != "" {
				r.SetPlayerBody(tc.body, true)
			}

			if tc.pickerB {
				r.SetPlayerBody("a", true)
			}

			got := r.ShareGold("a", tc.amount, tc.purses, tc.dropped)
			if g := addOf(got, "a"); g != tc.wantA {
				t.Errorf("a = %d want %d", g, tc.wantA)
			}

			if g := addOf(got, "b"); g != tc.wantB {
				t.Errorf("b = %d want %d", g, tc.wantB)
			}

			if g := addOf(got, "c"); g != tc.wantC {
				t.Errorf("c = %d want %d", g, tc.wantC)
			}

			if got.Overflow != tc.wantOver {
				t.Errorf("overflow = %d want %d", got.Overflow, tc.wantOver)
			}
		})
	}
}

func TestShareGoldOtherLevelAndNoParty(t *testing.T) {
	big := Purse{Cap: 100000}
	r := partyOf(t)
	r.SetArea("b", 9) // a different level: neither counted nor paid

	got := r.ShareGold("a", 50, map[string]Purse{"a": big, "b": big}, false)
	if addOf(got, "a") != 50 || addOf(got, "b") != -1 {
		t.Fatalf("other level: %+v", got)
	}

	r2 := afield() // nobody in a party
	got = r2.ShareGold("a", 50, map[string]Purse{"a": big, "b": big}, false)
	if addOf(got, "a") != 50 || addOf(got, "b") != -1 {
		t.Fatalf("no party: %+v", got)
	}
}
