package d2gamescreen

import (
	"fmt"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2interface"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
)

// movieState is the cinematic a running Game shows over itself. The game stands still while it plays (the exe's
// movie loop pumps messages only, UNVERIFIED how it resumes the game beyond packet 0x61), the movies of one trip run
// one after the other (the Act IV end movie is followed by the Lord of Destruction intro), and Esc/Space/click skip
// the one on screen.
type movieState struct {
	cur  *VideoPlayer
	next []int // table rows still to play
	// after runs once the queue is empty (the act trip that the movie precedes)
	after func()
}

// playGameMovies shows the given rows of the exe's movie table over the game, one after the other. It returns false
// (and shows nothing) when movies are off or the rows have no file.
func (v *Game) playGameMovies(rows ...int) bool {
	if !PlayVideosEnabled() || v.movie.cur != nil {
		return false
	}

	var q []int

	for _, r := range rows {
		if r >= 0 && movieAsset(r) != "" {
			q = append(q, r)
		}
	}

	if len(q) == 0 {
		return false
	}

	v.movie.next = q

	v.audioProvider.PlayBGM("") // the exe clears the sound before a movie; the next level starts its own music
	v.startNextMovie()

	return v.movie.cur != nil
}

func (v *Game) startNextMovie() {
	for len(v.movie.next) > 0 {
		row := v.movie.next[0]
		v.movie.next = v.movie.next[1:]

		p := CreateVideoPlayer(v.asset, v.renderer, v.inputManager, v.audioProvider, movieAsset(row), v.logLevel, nil)
		p.overlay = true
		p.onDone = func() { v.movieEnded(p) }
		v.Infof("MOVIE play row=%d file=%s", row, d2level.MovieFile(row, "640x292"))
		p.Open()

		if p.Active() {
			v.movie.cur = p
			v.uiManager.SetHidden(true) // the panel, belt and orbs are widgets: keep them off the picture

			return
		}

		// could not be opened: Advance of a failed player would call finish; do it here and try the next one
		_ = p.OnUnload()
	}

	v.movie.cur = nil
	v.uiManager.SetHidden(false)

	if f := v.movie.after; f != nil {
		v.movie.after = nil
		f()
	}
}

// movieEnded is called by a movie that finished or was skipped.
func (v *Game) movieEnded(p *VideoPlayer) {
	_ = p.OnUnload()
	v.movie.cur = nil
	v.Infof("MOVIE end %s skipped=%v", p.path, p.runner != nil && !p.runner.Done())
	v.startNextMovie()
}

// advanceMovie runs the movie over the game; it reports true while a movie is on screen (the game is paused).
func (v *Game) advanceMovie(elapsed float64) bool {
	if v.movie.cur == nil {
		return false
	}

	_ = v.movie.cur.Advance(elapsed)

	return v.movie.cur != nil
}

// renderMovie draws the movie over the game.
func (v *Game) renderMovie(screen d2interface.Surface) {
	if v.movie.cur != nil {
		v.movie.cur.Render(screen)
	}
}

// commandSkipMovie is "skipmovie": the same as Esc on the movie.
func (v *Game) commandSkipMovie(_ []string) error {
	if v.movie.cur == nil {
		return fmt.Errorf("no movie is playing")
	}

	v.movie.cur.Skip()

	return nil
}

// stopMovie drops a movie that is still on screen when the game screen goes away (no trip follows).
func (v *Game) stopMovie() {
	v.movie.next, v.movie.after = nil, nil

	if p := v.movie.cur; p != nil {
		v.movie.cur = nil
		p.done = true
		_ = p.OnUnload()
	}

	v.uiManager.SetHidden(false)
}
