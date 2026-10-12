package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// Wave 37 sound wiring: the area-entry hero voice and the object mode sounds.
// The rules are VERIFIED in d2audio/w37_sound.go.

// areaVoiceState is the exe's bookkeeping of SOUND_PlayAreaEntryVoiceOnLevelChange 0x4c8d10.
type areaVoiceState struct {
	level     int          // level the hero is in (the exe's DAT_007c0a64)
	changedAt int64        // tick of the change (DAT_007c0a58)
	done      int          // level a voice was last decided for (DAT_007c0a8c)
	last      int          // level last looked at (DAT_007c0954)
	heard     map[int]bool // levels whose record was used up (DAT_007bf940)
	set       bool
}

// areaVoiceEnter notes a level change.
func (v *Game) areaVoiceEnter(level int) {
	if v.areaVoice.set && v.areaVoice.level == level {
		return
	}

	v.areaVoice.set = true
	v.areaVoice.level = level
	v.areaVoice.changedAt = v.soundEngine.Ticks()
}

// advanceAreaVoice runs once per frame: more than 0x3d ticks after a level change the area-entry voice is
// decided, once per change.
func (v *Game) advanceAreaVoice() {
	st := &v.areaVoice
	if !st.set || st.done == st.level || v.soundEngine.Ticks()-st.changedAt <= d2audio.AreaEntryDelayTicks {
		return
	}

	st.done = st.level
	v.areaVoiceDecide(st.level)
}

// areaVoiceDecide is SOUND_PlayAreaEntryVoiceOnLevelChange: a level of a record not used up yet uses the whole
// record up (every level of it counts as heard whatever happens next), and its "find" line is said when its quest
// is still open, the hero made no sound in the last 0x3e ticks and no speech is active.
func (v *Game) areaVoiceDecide(level int) {
	st := &v.areaVoice
	if level == st.last {
		return
	}

	if st.heard[level] {
		st.last = level
		return
	}

	rec := d2audio.AreaVoiceFor(level)
	st.last = level

	if rec == nil {
		return
	}

	if st.heard == nil {
		st.heard = map[int]bool{}
	}

	for _, l := range rec.Levels {
		st.heard[l] = true
	}

	open := true

	if r := v.quests(); r != nil {
		open = r.g.QuestOpen(rec.Quest)
	}

	now := v.soundEngine.Ticks()

	switch {
	case !open:
		v.Infof("AREAVOICE level=%d voice=%#x skipped: quest %d is not open", level, rec.Voice, rec.Quest)
	case now-v.heroSoundTick <= d2audio.AreaEntryQuietTicks && v.heroSoundTick != 0:
		v.Infof("AREAVOICE level=%d voice=%#x skipped: the hero just made a sound", level, rec.Voice)
	case v.soundEngine.SpeechActive():
		v.Infof("AREAVOICE level=%d voice=%#x skipped: speech is active", level, rec.Voice)
	default:
		v.Infof("AREAVOICE level=%d (%s) voice=%#x quest=%d", level, v.levelName(level), rec.Voice, rec.Quest)
		v.sayHero(rec.Voice)
	}
}

// playDoorSound is the door sound of a door being operated: opening enters mode 2 (mode 1 for the doors whose
// table has no mode 2 sound), closing enters mode 0.
func (v *Game) playDoorSound(ob *d2mapentity.Object, opening bool) {
	if ob == nil || ob.Record() == nil {
		return
	}

	v.playObjectSound(d2audio.DoorOperateSound(ob.Record().Index, opening), ob, "door")
}
