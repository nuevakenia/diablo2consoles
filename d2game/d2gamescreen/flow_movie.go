package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
)

// logActMovie applies the exe's cinematic rule to an act trip (d2level.ActChangeMovie) and logs the movie that
// the original would play: "CINEMATIC act A -> B movie=N file=F". Playing the Bink file stays behind
// OD2_PLAY_VIDEOS (Bink decoding is off by default), and the in-game hand-over to the video screen is not
// wired (UNVERIFIED how the original resumes the game afterwards beyond packet 0x61); the log is what the
// scenarios read.
func (v *Game) logActMovie(from, to int, via string) {
	var kind d2level.Via

	switch via {
	case "portal":
		kind = d2level.ViaPortal
	case "talk":
		kind = d2level.ViaTalk
	case "npc":
		kind = d2level.ViaNPC
	default:
		return
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
		return
	}

	v.travel.moviePlayed = true

	v.Infof("CINEMATIC act %d -> %d movie=%d file=%s then=%d playback=%v", from, to, m.Movie,
		d2level.MovieFile(m.Movie, "640x292"), m.Then, PlayVideosEnabled())
}
