package d2audio

import (
	"math/rand"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2daynight"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio/d2sfx"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// ticksPerSecond is the game tick rate Event Delay is expressed in (assumed
// to be the 25 Hz game frame; the column's unit is UNVERIFIED).
const ticksPerSecond = d2sfx.TicksPerSecond

// Selection is what a SoundEnviron.txt row plays at a given time of day.
type Selection struct {
	Song     int // music (Sounds.txt index, Music Vol master)
	Ambience int // looping scene sound
	Event    int // random one-shot (or short loop) sound
	Night    bool
}

// IsNight says whether a d2daynight phase uses the Night columns. VERIFIED
// (SOUND_UpdateAmbientSounds 0x4e1270: day columns when phase-1 < 3, i.e.
// phases 1, 2 and 3; phases 0, 4 and 5 use the Night columns).
func IsNight(phase int) bool {
	return d2daynight.Classify(phase) == d2daynight.Evening
}

// Select picks the sounds of an environment for day or night.
func Select(r *d2records.SoundEnvironRecord, night bool) Selection {
	if r == nil {
		return Selection{Night: night}
	}

	if night {
		return Selection{Song: r.Song, Ambience: r.NightAmbience, Event: r.NightEvent, Night: true}
	}

	return Selection{Song: r.Song, Ambience: r.DayAmbience, Event: r.DayEvent}
}

// MusicSwapTicks is how long a changed level waits before its new song
// replaces the one still playing: more than 0x4a (74) ticks (VERIFIED,
// SOUND_UpdateLevelMusicEnvironment 0x4d98a0).
const MusicSwapTicks = 0x4a

// StepMusic returns the song the level music slot holds after one update.
// The wanted song is adopted at once when nothing plays or music is off,
// otherwise only once more than MusicSwapTicks ticks passed since the level
// changed (VERIFIED 0x4d98a0). on is "sound enabled and music volume not 0";
// which flag the exe's SOUND_GetModeFlagB reads is UNVERIFIED (taken as the
// sound-effects volume being above 0, as the volume-chain notes say).
func StepMusic(cur, wanted int, ticksSinceLevelChange float64, on bool) int {
	if cur != wanted && (ticksSinceLevelChange > MusicSwapTicks || !on || cur == 0) {
		return wanted
	}

	return cur
}

// randPM is the exe's rand in [-n, n] (0x4e10b0: rand(2n+1) - n).
func randPM(n int, rnd func(int) int) int { return rnd(2*n+1) - n }

// NextEventTicks returns the wait before the next ambient event for an Event
// Delay column value d: d + rand[-d/3, d/3], in game ticks (VERIFIED,
// SOUND_UpdateAmbientSounds 0x4e1270: the pair 0x4e10b0(d/3) + d).
func NextEventTicks(delay int, rnd func(int) int) int {
	if delay <= 0 {
		return 1
	}

	return delay + randPM(delay/3, rnd)
}

// FirstEventElapsed is how long an event already counts as waiting when its
// id first becomes the active one: now - rand(interval), so the first event
// can come at once (VERIFIED, 0x4e14d8: 0x4e1030(interval) is subtracted from
// the current tick).
func FirstEventElapsed(interval int, rnd func(int) int) int {
	if interval <= 0 {
		return 0
	}

	return rnd(interval)
}

// EventOffset is where an ambient event sounds, relative to the hero, in
// sound units: x is +-[450,750] (random sign), y is [-100,100] (VERIFIED,
// 0x4e1090(0x1c2,0x2ee) times a random sign and 0x4e10b0(100) at 0x4e153a).
func EventOffset(rnd func(int) int) (dx, dy float64) {
	sign := 1
	if rnd(2) == 0 {
		sign = -1
	}

	const lo, hi = 0x1c2, 0x2ee

	dx = float64(sign * (lo + rnd(hi-lo+1)))
	dy = float64(randPM(100, rnd))

	return dx, dy
}

// EventPosition places an ambient event around the listener (see
// EventOffset). The bank doubles the y delta when measuring distance, so y is
// halved here to keep the exe's relative vector.
func EventPosition(lx, ly float64, rnd func(int) int) (x, y float64) {
	dx, dy := EventOffset(rnd)

	return lx + dx, ly + dy/2
}

// envSink is what a SoundEnvironment needs from the sound engine.
type envSink interface {
	// loop starts a non-positional looping sound; the returned function stops it.
	loop(index int) (stop func())
	// event plays a positional one-shot near the listener.
	event(index int, rnd func(int) int) (handle string)
	// musicEnabled says whether music may play (sound and music volume both above 0).
	musicEnabled() bool
	// handle names a Sounds.txt index for logs.
	handle(index int) string
}

type engineSink struct{ e *SoundEngine }

func (s engineSink) loop(index int) func() {
	snd := s.e.PlaySoundID(index)
	if snd == nil {
		return func() {}
	}

	return snd.Stop
}

func (s engineSink) musicEnabled() bool {
	return s.e.MusicEnabled()
}

func (s engineSink) handle(index int) string {
	if row := s.e.Bank().Table().Get(index); row != nil {
		return row.Handle
	}

	return ""
}

func (s engineSink) event(index int, rnd func(int) int) string {
	row := s.e.Bank().Table().Get(index)
	if row == nil {
		return ""
	}

	lx, ly := s.e.Listener()
	x, y := EventPosition(lx, ly, rnd)

	s.e.play(d2sfx.Request{Index: index, HasPos: true, X: x, Y: y}, PlayOpts{X: x, Y: y, Kind: "ambient-event", Who: row.Handle})

	return row.Handle
}

// SoundEnvironment plays the music, ambient loop and random events of the
// map area the hero is in, from SoundEnviron.txt (selected by the level's
// SoundEnv column) and the day phase.
type SoundEnvironment struct {
	sink  envSink
	envs  map[int]*d2records.SoundEnvironRecord
	rnd   func(int) int
	logf  func(format string, args ...interface{})
	phase int

	environment *d2records.SoundEnvironRecord
	night       bool
	started     bool

	stopMusic   func()
	stopAmbient func()

	ticks         float64 // game ticks since creation, advanced by Advance
	levelChangeAt float64 // tick of the last SetEnv
	songCur       int     // song held by the level music slot (the exe's DAT_007c0a60)
	songPlaying   int     // song actually started (0 when stopped)
	cur           Selection

	eventInterval float64 // game ticks between events
	eventElapsed  float64 // game ticks since the last one
	lastEvent     int     // event id the schedule belongs to (kept across environments, like the exe's DAT_007c0d18)
}

// NewSoundEnvironment creates a SoundEnvironment using the given SoundEngine
func NewSoundEnvironment(soundEngine *SoundEngine) SoundEnvironment {
	return newSoundEnvironment(engineSink{soundEngine}, soundEngine.asset.Records.Sound.Environment,
		// nolint:gosec // client-only, no need for a secure generator
		rand.Intn, soundEngine.Infof)
}

func newSoundEnvironment(sink envSink, envs map[int]*d2records.SoundEnvironRecord, rnd func(int) int,
	logf func(string, ...interface{})) SoundEnvironment {
	r := SoundEnvironment{
		sink: sink, envs: envs, rnd: rnd, logf: logf,
		phase: d2daynight.PhaseDay,
		// Start with env NONE
		environment: envs[0],
	}

	if r.environment == nil {
		r.environment = &d2records.SoundEnvironRecord{}
	}

	return r
}

// Environment returns the active SoundEnviron.txt row.
func (s *SoundEnvironment) Environment() *d2records.SoundEnvironRecord { return s.environment }

// Selection returns what is currently selected for playback.
func (s *SoundEnvironment) Selection() Selection { return s.cur }

// NextEventIn returns the game ticks left before the next ambient event.
func (s *SoundEnvironment) NextEventIn() float64 { return s.eventInterval - s.eventElapsed }

// SetEnv sets the sound environment using the given record index
func (s *SoundEnvironment) SetEnv(environmentIdx int) {
	newEnv, ok := s.envs[environmentIdx]
	if !ok {
		return
	}

	// the game re-sends the current environment every frame: only a real change restarts the swap clock
	if s.started && s.environment.Index == environmentIdx {
		return
	}

	s.levelChangeAt = s.ticks
	s.environment = newEnv
	s.apply("env change")
}

// SetDayPhase tells the environment the d2daynight phase (0..5); a change
// between day and night swaps the ambience and the event sounds.
func (s *SoundEnvironment) SetDayPhase(phase int) {
	s.phase = phase

	night := IsNight(phase)
	if night == s.night {
		return
	}

	s.night = night
	if s.started {
		s.apply("day phase")
	}
}

// apply recomputes the selection, switching only the parts that changed (the
// old sound fades out through its Fade Out column while the new one fades in).
func (s *SoundEnvironment) apply(why string) {
	sel := Select(s.environment, s.night)
	first := !s.started
	s.started = true

	s.cur.Song = sel.Song // the level music slot first, then the ambience (log order)
	s.updateMusic()

	if first || sel.Ambience != s.cur.Ambience {
		s.stopAmbient = s.restart(s.stopAmbient, sel.Ambience)
	}

	s.cur = sel

	// the exe reschedules only when the event id differs from the one the
	// current schedule belongs to (0x4e14b9), not on every environment change
	if sel.Event != 0 && sel.Event != s.lastEvent {
		s.scheduleEvent()
	}

	s.logf("AMBIENT %s env=%s(%d) phase=%d night=%v music=%s(%d) ambience=%s(%d) event=%s(%d) event_delay=%d indoors=%d",
		why, s.environment.Handle, s.environment.Index, s.phase, s.night, s.sink.handle(sel.Song), sel.Song,
		s.sink.handle(sel.Ambience), sel.Ambience, s.sink.handle(sel.Event), sel.Event,
		s.environment.EventDelay, s.environment.Indoors)
}

// updateMusic runs the level music slot: the wanted song is adopted per
// StepMusic and plays only while music is enabled.
func (s *SoundEnvironment) updateMusic() {
	on := s.sink.musicEnabled()
	s.songCur = StepMusic(s.songCur, s.cur.Song, s.ticks-s.levelChangeAt, on)

	want := 0
	if on {
		want = s.songCur
	}

	if want == s.songPlaying {
		return
	}

	if s.stopMusic != nil {
		s.stopMusic()
		s.stopMusic = nil
	}

	s.songPlaying = want
	if want != 0 {
		s.stopMusic = s.sink.loop(want)
	}

	s.logf("AMBIENT music playing=%s(%d) wanted=%s(%d) enabled=%v", s.sink.handle(want), want,
		s.sink.handle(s.cur.Song), s.cur.Song, on)
}

func (s *SoundEnvironment) restart(stop func(), index int) func() {
	if stop != nil {
		stop()
	}

	if index == 0 {
		return nil
	}

	return s.sink.loop(index)
}

func (s *SoundEnvironment) scheduleEvent() {
	delay := s.environment.EventDelay
	interval := NextEventTicks(delay, s.rnd)
	s.eventInterval = float64(interval)
	s.eventElapsed = float64(FirstEventElapsed(interval, s.rnd))
	s.lastEvent = s.cur.Event
	s.logf("AMBIENT event scheduled handle=%s first_in=%.1fs interval=%.1fs (delay column %d ticks)",
		s.sink.handle(s.cur.Event), s.NextEventIn()/ticksPerSecond, s.eventInterval/ticksPerSecond, delay)
}

// Advance advances the sound engine and plays sounds when necessary
func (s *SoundEnvironment) Advance(elapsed float64) {
	if !s.started {
		return
	}

	s.ticks += elapsed * ticksPerSecond
	s.updateMusic()

	if s.cur.Event == 0 {
		return
	}

	s.eventElapsed += elapsed * ticksPerSecond
	if s.eventElapsed < s.eventInterval {
		return
	}

	s.sink.event(s.cur.Event, s.rnd)
	s.eventElapsed = 0
	s.eventInterval = float64(NextEventTicks(s.environment.EventDelay, s.rnd))
}
