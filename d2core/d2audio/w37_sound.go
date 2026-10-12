package d2audio

// Wave 37 sound rules, read from Game.exe 1.14b (see d2-re-notes w37-sound4.md).
// Everything here is VERIFIED unless the comment says UNVERIFIED.

// Sounds.txt row ranges the exe tests playing entries by.
const (
	// SpeechFirst..SpeechLast are the hero and NPC voice rows (ids 0xb76..0x1230).
	SpeechFirst = 0xb76
	SpeechLast  = 0x1230
	// MusicFirst..MusicLast are the music rows (ids 0x1231..0x125a), area tracks first, quest stings last.
	MusicFirst = 0x1231
	MusicLast  = 0x125a
)

// SpeechPlaying is "a unit sound is already playing" of the exe
// (UNITSND_HasPlayingEntry 0x4b67d0): any entry in the speech range holds a voice, whoever owns it.
// It gates the hero's refusal and chat lines (UNITSND_PlayUnitSoundById 0x4c8470) and the hero
// hit grunt (UNITSND_PlayUnitModeStartSound 0x4c8e90).
func (s *SoundEngine) SpeechPlaying() bool {
	return s.Bank().IndexPlaying(SpeechFirst, SpeechLast, false)
}

// SpeechActive is SOUND_AnyUnitSoundActive 0x4b6570: a speech entry exists that is not finished
// (a line waiting for its start delay counts). It gates the area-entry hero voice.
func (s *SoundEngine) SpeechActive() bool {
	return s.Bank().IndexPlaying(SpeechFirst, SpeechLast, true)
}

// MusicActive is SOUND_AnyEntryActiveInRange1231 0x4b65b0.
func (s *SoundEngine) MusicActive() bool {
	return s.Bank().IndexPlaying(MusicFirst, MusicLast, true)
}

// ---- hero hit and death vocals ----

// The hero's hit grunt and death cry are the first two slots of the per class record at 0x728d24
// (read for all seven classes: amazon_hit_1/_death_1 ... druid_hit_1/_death_1, the first rows of
// each class' vocal group; VERIFIED). UNITSND_HandleUnitModeSound 0x4c9030 sends the modes
// DT(0), GH(4) and 0x13 of a player to UNITSND_PlayUnitModeStartSound 0x4c8e90, which plays
// slot 0 on GH / 0x13 only when no speech entry is playing (SpeechPlaying), slot 1 on death.
// There is no time spacing in the exe: the spacing comes from the hit recovery mode being entered.

// HitGruntPlays is the exe's gate for the hero hit grunt.
func HitGruntPlays(speechPlaying, dead bool) bool { return !speechPlaying && !dead }

// ---- object mode sounds ----

// ObjectSlot is one Sounds.txt handle of an object class, played when the object enters mode.
type ObjectSlot struct {
	Mode   int
	Handle string
}

// ObjectSounds are the sounds an object makes when it ENTERS a mode (UNITSND_PlayUnitModeSound
// 0x4c7f20 through OBJECT_ClientApplyModeChange 0x4b9840): the per object table at 0x7282e8
// (index objects.txt row) points at a record whose dword k is the Sounds.txt id for mode k
// (UNITSND_GetSoundSlotEntry 0x4c7e40). It plays only when the object had a mode before (the
// first mode of a freshly built object is silent), except the town portal (row 59) which
// also sounds at once. Only the door, waypoint and portal classes were decoded (116 records exist).
//
//	rows 13 14 290 291 292 293          mode 0 metal close, mode 2 metal open
//	rows 15 16 62 63 64 74 75 294 295   mode 0 wood close,  mode 2 wood open
//	rows 23 24 25 47                    mode 0 gate close,  mode 2 gate open
//	row  27                             mode 0 wood close,  mode 1 wood open
//	rows 91 92 98 99                    modes 0 and 1 tomb door (stone)
//	row  129                            modes 1 and 3 object_door_secret
//	row  153                            mode 1 object_door_duriel
//	rows 229 230                        modes 1 and 3 object_door_slime
//	row  298                            mode 1 arcane portal open (+ a loop on mode 2)
//	row  342                            mode 1 hellgate open (+ a loop on mode 2)
//	rows 59 60 377                      mode 1 object_townportal
//	waypoints (119 145 156 157 237 238 288 323 324 398 402 430 495 497 512 540) mode 1 waypoint
func ObjectSounds(row int) []ObjectSlot {
	switch row {
	case 13, 14, 290, 291, 292, 293:
		return []ObjectSlot{{0, "object_door_metal_close"}, {2, "object_door_metal_open"}}
	case 15, 16, 62, 63, 64, 74, 75, 294, 295:
		return []ObjectSlot{{0, "object_door_wood_close"}, {2, "object_door_wood_open"}}
	case 23, 24, 25, 47:
		return []ObjectSlot{{0, "object_door_gate_close"}, {2, "object_door_gate_open"}}
	case 27:
		return []ObjectSlot{{0, "object_door_wood_close"}, {1, "object_door_wood_open"}}
	case 91, 92, 98, 99:
		return []ObjectSlot{{0, "object_door_stone"}, {1, "object_door_stone"}}
	case 129:
		return []ObjectSlot{{1, "object_door_secret"}, {3, "object_door_secret"}}
	case 153:
		return []ObjectSlot{{1, "object_door_duriel"}}
	case 229, 230:
		return []ObjectSlot{{1, "object_door_slime"}, {3, "object_door_slime"}}
	case 298:
		return []ObjectSlot{{1, "object_door_arcane_open"}}
	case 342:
		return []ObjectSlot{{1, "object_door_hellgate_open"}}
	case 59, 60, 377:
		return []ObjectSlot{{1, HandleTownPortalOpen}}
	case 119, 145, 156, 157, 237, 238, 288, 323, 324, 398, 402, 430, 495, 497, 512, 540:
		return []ObjectSlot{{1, HandleWaypointOpen}}
	}

	return nil
}

// ObjectModeSound is the handle an object of objects.txt row plays on entering mode ("" = silent).
func ObjectModeSound(row, mode int) string {
	for _, s := range ObjectSounds(row) {
		if s.Mode == mode {
			return s.Handle
		}
	}

	return ""
}

// DoorOperateSound is the sound of a door being operated: opening enters mode 2 (mode 1 for the classes
// whose table has no mode 2 sound), closing enters mode 0. "" for a silent door.
func DoorOperateSound(row int, opening bool) string {
	if !opening {
		return ObjectModeSound(row, 0)
	}

	if h := ObjectModeSound(row, 2); h != "" {
		return h
	}

	return ObjectModeSound(row, 1)
}

// ---- area entry hero voice ----

// AreaVoice is one record of the table at 0x728d40 (SOUND_PlayAreaEntryVoiceOnLevelChange 0x4c8d10):
// walking into one of Levels (levels.txt ids) for the first time says Voice (a UNITSND_PlayUnitSoundById
// id, one of the "find_*" lines) when quest Quest (a d2quest id) is still open. The record is
// shared by all its levels: the first visit to any of them uses the line up (the exe marks every
// level of the record as heard).
type AreaVoice struct {
	Levels []int
	Quest  int
	Voice  int
}

// AreaVoices is the table, 14 records.
var AreaVoices = []AreaVoice{
	{[]int{17}, 2, 0x26},                     // Burial Grounds: find_burial
	{[]int{34, 35, 36, 37}, 6, 0x28},         // Catacombs 1-4: find_catacombs
	{[]int{8}, 1, 0x29},                      // Den of Evil: find_den
	{[]int{29, 30, 31}, 6, 0x2a},             // Jail 1-3: find_jail
	{[]int{26, 27}, 3, 0x2b},                 // Monastery gate / outer cloister: find_monastery
	{[]int{20, 21, 22, 23, 24, 25}, 5, 0x2c}, // Forgotten Tower: find_tower
	{[]int{38}, 6, 0x2e},                     // Tristram: find_tristram
	{[]int{2, 3, 4, 5, 6, 7}, 1, 0x2f},       // Blood Moor .. Tamoe Highland: find_wilderness
	{[]int{74}, 12, 0x37},                    // Arcane Sanctuary: find_arcane
	{[]int{58}, 13, 0x38},                    // Claw Viper Temple 1: find_clawviper
	{[]int{110}, 31, 0x4c},                   // Bloody Foothills: act5 find_wilderness
	{[]int{120}, 35, 0x4e},                   // Arreat Summit: find_mountaintop
	{[]int{121}, 34, 0x4d},                   // Nihlathak's temple area: find_nihlathak
	{[]int{132}, 36, 0x4f},                   // Worldstone chamber: find_worldstone
}

// AreaEntryDelayTicks is how long after a level change the voice may fire (more than 0x3d = 61
// ticks since the change, checked from SOUND_UpdateLevelMusicEnvironment 0x4d98a0), and the hero
// must not have started a sound (hit or mode sound) in the last 0x3e ticks.
const (
	AreaEntryDelayTicks = 0x3d
	AreaEntryQuietTicks = 0x3e
)

// AreaVoiceFor returns the record of a level (nil if the level has none).
func AreaVoiceFor(level int) *AreaVoice {
	for i := range AreaVoices {
		for _, l := range AreaVoices[i].Levels {
			if l == level {
				return &AreaVoices[i]
			}
		}
	}

	return nil
}

// ---- quest music plans ----

// QuestPlan is what UNITSND_PlayUnitSoundById 0x4c8470 does for the quest lines that come with
// music (DRAWLIST_AddSoundLinkedEffect 0x4d9b20): the level music is silenced for Silence ticks,
// the quest sting Sting (Sounds.txt handle, "" for none) starts StingDelay ticks from now, and the
// hero's line starts LineDelay ticks from now (not when Line is false). Before the music stops,
// its resume position is set to cue block Cue (0 = start, 1..3 = Sounds.txt Block 1..3).
type QuestPlan struct {
	Sting      string
	StingDelay int
	Silence    int
	Cue        int
	LineDelay  int
	Line       bool
}

// QuestPlans maps the hero voice id to its plan.
var QuestPlans = map[int]QuestPlan{
	0x21: {"music_quest_andariel", 0, 475, 0, 425, true},
	0x22: {"music_quest_bloodraven", 475, 800, 1, 525, true},
	0x23: {"music_quest_den", 0, 300, 1, 250, true},
	0x25: {"", 0, 1500, 0, 0, false}, // tower: only the music is silenced, no sting and no line here
	0x32: {"music_quest_radament", 475, 1100, 1, 525, true},
	0x34: {"music_quest_tainted", 0, 425, 1, 375, true},
	0x42: {"music_quest_mephisto", 0, 425, 0, 375, true},
	0x4b: {"music_quest_diablo", 0, 475, 0, 325, true},
	0x50: {"music_quest_shenk", 0, 500, 1, 475, true},
	0x52: {"music_quest_nihlathak", 0, 550, 1, 400, true},
	0x53: {"music_quest_baal", 0, 525, 1, 425, true},
}

// ---- music cue points ----

// MusicSampleRate is the sample rate of the music files (22050 Hz stereo, read from four of the tracks;
// the others are UNVERIFIED). Sounds.txt Block 1..3 and the stream position of the exe are in samples.
const MusicSampleRate = 22050

// MusicBeatTicks is how often the playing track's resume position is refreshed (more than 0x7d = 125
// ticks since the last refresh).
const MusicBeatTicks = 0x7d

// NextCue is the resume position stored for a track whose stream is at pos samples: the next of its
// three cue blocks after pos, 0 when pos is past the last or the blocks are unset (-1)
// (SOUND_UpdateLevelMusicEnvironment 0x4d98a0).
func NextCue(pos int, blocks [3]int) int {
	next := 0

	if pos > -1 && pos < blocks[0] {
		next = blocks[0]
	}

	if blocks[0] <= pos && pos < blocks[1] {
		next = blocks[1]
	}

	if blocks[1] <= pos && pos < blocks[2] {
		next = blocks[2]
	}

	return next
}

// SamplesToTicks converts a position in music samples to game ticks (25 per second).
func SamplesToTicks(samples int) int { return samples * TicksPerSecondInt / MusicSampleRate }

// TicksToSamples converts game ticks to music samples.
func TicksToSamples(ticks int) int { return ticks * MusicSampleRate / TicksPerSecondInt }

// TicksPerSecondInt is the game tick rate as an int.
const TicksPerSecondInt = 25

// MusicResume remembers, per track, the cue point (in samples) where the area music starts again.
type MusicResume struct {
	cue map[int]int
}

// Position is where the song starts (samples), 0 for a track with no stored cue.
func (m *MusicResume) Position(song int) int { return m.cue[song] }

// Set stores the resume position of song.
func (m *MusicResume) Set(song, samples int) {
	if song == 0 {
		return
	}

	if m.cue == nil {
		m.cue = map[int]int{}
	}

	m.cue[song] = samples
}
