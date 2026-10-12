package d2gamescreen

import (
	"os"
	"strings"
	"sync"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
)

var (
	cinematicsMu     sync.Mutex
	cinematicsOption func() bool
)

// SetCinematicsOption wires the Options -> Cinematics choice (d2config.Configuration.Cinematics) to the screens.
func SetCinematicsOption(f func() bool) {
	cinematicsMu.Lock()
	defer cinematicsMu.Unlock()

	cinematicsOption = f
}

// automationRun reports a scripted run (any OD2_AUTO* variable): the scenarios of scripts/verify.d must not be
// held up by a movie, so there the movies are off unless OD2_PLAY_VIDEOS=1 asks for them.
func automationRun() bool {
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "OD2_AUTO") {
			return true
		}
	}

	return false
}

// PlayVideosEnabled reports whether the movies play on their own: the opening movie on the first launch, the act
// change movies and the Baal movie. Default ON (the Cinematics option of the Accessibility page switches it off).
// OD2_PLAY_VIDEOS=1 forces them on, OD2_PLAY_VIDEOS=0 off; scripted runs (OD2_AUTO*) default to off.
func PlayVideosEnabled() bool {
	switch os.Getenv("OD2_PLAY_VIDEOS") {
	case "1":
		return true
	case "0":
		return false
	}

	if automationRun() {
		return false
	}

	cinematicsMu.Lock()
	f := cinematicsOption
	cinematicsMu.Unlock()

	return f == nil || f()
}

// VideosAvailable reports that the Cinematics menu may play a movie when asked (the option only governs the movies
// that start by themselves); OD2_PLAY_VIDEOS=0 still turns every movie off.
func VideosAvailable() bool { return os.Getenv("OD2_PLAY_VIDEOS") != "0" }

// movieAsset is the asset path of a row of the exe's movie table (d2level.Movie*), "" for the promo row and
// anything out of range. Rows 1 to 7 map to the paths in d2resource.
func movieAsset(movie int) string {
	switch movie {
	case d2level.MovieAct1Start:
		return d2resource.Act1Intro
	case d2level.MovieAct2Start:
		return d2resource.Act2Intro
	case d2level.MovieAct3Start:
		return d2resource.Act3Intro
	case d2level.MovieAct4Start:
		return d2resource.Act4Intro
	case d2level.MovieAct4End:
		return d2resource.Act4Outro
	case d2level.MovieLodIntro:
		return d2resource.Act5Intro
	case d2level.MovieLodOut:
		return d2resource.Act5Outro
	}

	return ""
}
