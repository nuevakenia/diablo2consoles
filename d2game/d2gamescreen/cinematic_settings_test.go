package d2gamescreen

import (
	"os"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
)

// clearAuto removes every OD2_AUTO* variable for the test (t.Setenv restores them afterwards).
func clearAuto(t *testing.T) {
	t.Helper()

	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 && strings.HasPrefix(kv[:i], "OD2_AUTO") {
			t.Setenv(kv[:i], "")
			os.Unsetenv(kv[:i])
		}
	}
}

func TestPlayVideosEnabled(t *testing.T) {
	defer SetCinematicsOption(nil)

	cases := []struct {
		name   string
		env    string // OD2_PLAY_VIDEOS
		auto   bool   // a scripted run
		option func() bool
		want   bool
	}{
		{"default on", "", false, nil, true},
		{"option on", "", false, func() bool { return true }, true},
		{"option off", "", false, func() bool { return false }, false},
		{"scripted run is off by default", "", true, nil, false},
		{"forced on in a scripted run", "1", true, func() bool { return false }, true},
		{"forced off", "0", false, func() bool { return true }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearAuto(t)
			t.Setenv("OD2_PLAY_VIDEOS", tc.env)

			if tc.auto {
				t.Setenv("OD2_AUTOGAME", "x")
			}

			SetCinematicsOption(tc.option)

			if got := PlayVideosEnabled(); got != tc.want {
				t.Fatalf("PlayVideosEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVideosAvailable(t *testing.T) {
	t.Setenv("OD2_PLAY_VIDEOS", "")

	if !VideosAvailable() {
		t.Fatal("the Cinematics menu plays unless OD2_PLAY_VIDEOS=0")
	}

	t.Setenv("OD2_PLAY_VIDEOS", "0")

	if VideosAvailable() {
		t.Fatal("OD2_PLAY_VIDEOS=0 turns every movie off")
	}
}

func TestMovieAssetCoversTheTable(t *testing.T) {
	seen := map[string]bool{}

	for row := d2level.MovieAct1Start; row < d2level.NumMovies; row++ {
		p := movieAsset(row)
		if p == "" || seen[p] {
			t.Fatalf("row %d: path %q empty or duplicated", row, p)
		}

		seen[p] = true

		// the resource path and the exe's name pattern name the same file (ignoring case and the folder)
		want := strings.ToLower(d2level.MovieFile(row, "640x292"))
		if !strings.HasSuffix(strings.ToLower(p), want) {
			t.Errorf("row %d: %q does not end in %q", row, p, want)
		}
	}

	if movieAsset(d2level.MoviePromo) != "" || movieAsset(99) != "" {
		t.Fatal("the promo row and unknown rows have no file")
	}
}
