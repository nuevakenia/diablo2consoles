package d2level

import "testing"

func TestActChangeMovie(t *testing.T) {
	cases := []struct {
		name               string
		from, to           int
		via                Via
		done, played, lod  bool
		want               int
		wantThen, wantSent bool
	}{
		{"warriv first", 1, 2, ViaNPC, false, false, true, MovieAct2Start, false, true},
		{"meshif first", 2, 3, ViaNPC, false, false, false, MovieAct3Start, false, true},
		{"durance portal", 3, 4, ViaPortal, false, false, true, MovieAct4Start, false, true},
		{"tyrael plays end then lod intro", 4, 5, ViaTalk, false, false, true, MovieAct4End, true, true},
		{"tyrael classic nothing", 4, 5, ViaTalk, false, false, false, 0, false, false},
		{"second trip, slot already done", 1, 2, ViaNPC, true, false, true, 0, false, false},
		{"already played this session", 2, 3, ViaNPC, false, true, true, 0, false, false},
		{"back trip", 2, 1, ViaNPC, false, false, true, 0, false, false},
	}

	for _, c := range cases {
		m, ok := ActChangeMovie(c.from, c.to, c.via, c.done, c.played, c.lod)
		if ok != c.wantSent || (ok && m.Movie != c.want) || (ok && (m.Then >= 0) != c.wantThen) {
			t.Errorf("%s: got %+v %v", c.name, m, ok)
		}
	}

	if BaalMovie().Movie != MovieLodOut {
		t.Error("baal")
	}
}

func TestMovieFile(t *testing.T) {
	cases := map[int]string{
		MoviePromo: "d2_promo.bik", MovieAct1Start: "d2intro640x292.bik", MovieAct4End: "ACT04END640x292.BIK",
		MovieLodIntro: "D2X_INTRO_640x292.BIK", MovieLodOut: "D2X_OUT_640x292.BIK", NumMovies: "", -1: "",
	}

	for i, want := range cases {
		if got := MovieFile(i, "640x292"); got != want {
			t.Errorf("%d: %q want %q", i, got, want)
		}
	}
}
