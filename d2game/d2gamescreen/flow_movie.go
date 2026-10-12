package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
)

// actMovie applies the exe's cinematic rule to an act trip (d2level.ActChangeMovie) and logs the movie that
// the original would play: "CINEMATIC act A -> B movie=N file=F". Playing the Bink file stays behind
// the Cinematics option (PlayVideosEnabled) and runs over the game (game_movie.go); the log is what the scenarios read.
func (v *Game) actMovie(from, to int, via string) (d2level.ActMovie, bool) {
	var kind d2level.Via

	switch via {
	case "portal":
		kind = d2level.ViaPortal
	case "talk":
		kind = d2level.ViaTalk
	case "npc":
		kind = d2level.ViaNPC
	default:
		return d2level.ActMovie{}, false
	}

	done := false

	if q := v.heroQuests(); q != nil {
		if slot := d2level.FinishedSlot(from); slot >= 0 {
			done = q.Get(slot, d2s.QuestBitDone)
		}
	}

	m, ok := d2level.ActChangeMovie(from, to, kind, done, v.travel.moviePlayed, v.heroExpansion())
	if !ok {
		v.Infof("CINEMATIC none for act %d -> %d via=%s (slot done=%v, already played=%v)", from, to, via, done, v.travel.moviePlayed)
		return d2level.ActMovie{}, false
	}

	v.travel.moviePlayed = true

	v.Infof("CINEMATIC act %d -> %d movie=%d file=%s then=%d playback=%v", from, to, m.Movie,
		d2level.MovieFile(m.Movie, "640x292"), m.Then, PlayVideosEnabled())

	return m, true
}

// playBaalMovie plays the Lord of Destruction ending when Baal dies in the Worldstone Chamber (VERIFIED: the sender at
// 0x58c595 sends movie 7 without testing the play flag; expansion games only).
func (v *Game) playBaalMovie() {
	m := d2level.BaalMovie()

	v.Infof("CINEMATIC baal dead movie=%d file=%s playback=%v", m.Movie, d2level.MovieFile(m.Movie, "640x292"), PlayVideosEnabled())

	v.playGameMovies(m.Movie)
}
