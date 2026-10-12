package d2audio

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2animspeed"

// FormMonSoundsID is the MonSounds.txt row of the monster a Druid form is drawn
// as (monstats Wolf 430 and Bear 431 name the rows werewolf and werebear). A
// shapeshifted hero is the monster class for the animations (VERIFIED earlier,
// UNIT_GetShapeshiftAdjustedMode 0x646620); that his attack, hit and bite
// sounds come from the monster's MonSounds row is UNVERIFIED (the Sounds.txt
// rows druidwerewolf_* and druidwerebear_* exist for nothing else).
func FormMonSoundsID(token string) string {
	switch token {
	case d2animspeed.TokenWerewolf:
		return "werewolf"
	case d2animspeed.TokenWerebear:
		return "werebear"
	}

	return ""
}

// Sound rules of wave 36 (sound fidelity part 3). Everything here that is not
// marked VERIFIED is a port decision: Game.exe could not be read for it (the
// Ghidra project was not reachable during this wave), see w36-sound3.md.

// Sounds.txt handles of the sounds objects and the town portal make. Which
// function of the exe plays each one is UNVERIFIED; the rows are named after
// the objects (object\waypointignite2.wav, object\portalopen.wav,
// skill\misc\portalcast.wav, skill\misc\portalenter.wav).
const (
	HandleWaypointOpen    = "object_waypoint_open"
	HandleTownPortalOpen  = "object_townportal"
	HandleTownPortalCast  = "player_townportal_cast"
	HandleTownPortalEnter = "player_townportal_enter"
)

// DoorSound is the Sounds.txt handle of a door being operated, chosen by the
// objects.txt row (the row ordinal). UNVERIFIED: the rows are named after the
// door kinds (wood, metal, gate, stone tomb door, secret door); which row the
// original uses for which objects.txt door is guessed from the objects' acts
// and tokens (Act 2 T* = tomb doors, SQ / SY = secret doors, AD = gate).
func DoorSound(objectRow int, opening bool) string {
	suffix := "close"
	if opening {
		suffix = "open"
	}

	switch objectRow {
	case 91, 92, 98, 99: // Act 2 tomb doors
		return "object_door_stone"
	case 229, 230: // Act 2 secret doors
		return "object_door_secret"
	case 47: // gate
		return "object_door_gate_" + suffix
	case 290, 291, 292, 293, 294, 295: // the lower case door tokens
		return "object_door_metal_" + suffix
	}

	return "object_door_wood_" + suffix
}

// UnitSounds remembers the sound a unit (hero, monster) started last, for the
// exe's "unit sound already playing" test of UNITSND_PlayUnitSoundById
// 0x4c8470, which suppresses the refusal voices and the chat lines.
type UnitSounds struct {
	last map[int]*Sound
}

// Note records the sound a unit just started (nil is ignored).
func (u *UnitSounds) Note(emitter int, s *Sound) {
	if s == nil {
		return
	}

	if u.last == nil {
		u.last = map[int]*Sound{}
	}

	u.last[emitter] = s
}

// Playing says whether the unit's last sound is still sounding or waiting for
// its start delay.
func (u *UnitSounds) Playing(emitter int) bool {
	s := u.last[emitter]

	return s != nil && s.Alive()
}

// Forget drops the unit (it died or left).
func (u *UnitSounds) Forget(emitter int) { delete(u.last, emitter) }

// ResumeTicks is how often the music position is stored while a track plays:
// every 125 ticks (VERIFIED, w29: SOUND_UpdateLevelMusicEnvironment 0x4d98a0
// keeps the position at cue points; the cue point spacing is UNVERIFIED and
// taken as this period).
const ResumeTicks = 125

// MusicResume remembers where each area track stopped so that it continues
// there when the hero comes back (the exe stores the position of the track
// every 125 ticks and starts the track at it again). Positions are in game
// ticks from the start of the track.
type MusicResume struct {
	pos map[int]int
}

// Position is where the track song should start, 0 for a track not seen yet.
func (m *MusicResume) Position(song int) int { return m.pos[song] }

// Store keeps the position of song, rounded down to the last cue point, the
// way the exe only stores it on its 125 tick beat. length is the track length
// in ticks (0 = unknown, the position is not wrapped).
func (m *MusicResume) Store(song, playedTicks, length int) {
	if song == 0 || playedTicks < 0 {
		return
	}

	if m.pos == nil {
		m.pos = map[int]int{}
	}

	p := playedTicks / ResumeTicks * ResumeTicks
	if length > 0 {
		p %= length
	}

	m.pos[song] = p
}
