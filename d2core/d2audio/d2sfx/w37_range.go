package d2sfx

// IndexPlaying reports whether an instance whose Sounds.txt row lies in
// [lo, hi] is playing (holds a voice). With waiting set, instances still
// waiting for their start delay count too (the exe's status != 2 test).
// The exe asks this by ranges of sound ids: SOUND_FindPlayingUnitSoundEntry
// 0x4b6040 (playing, ids 0xb76..0x1230 = hero and NPC speech),
// SOUND_AnyUnitSoundActive 0x4b6570 (same range, not stopped) and
// SOUND_AnyEntryActiveInRange1231 0x4b65b0 (ids 0x1231..0x125a, the music).
func (b *Bank) IndexPlaying(lo, hi int, waiting bool) bool {
	in := func(q *Instance) bool {
		if q == nil || q.st == stDead || q.index < lo || q.index > hi {
			return false
		}

		return q.st == stPlaying || (waiting && q.st != stStopping)
	}

	for _, q := range b.voices {
		if in(q) {
			return true
		}
	}

	for _, q := range b.queue {
		if waiting && in(q) {
			return true
		}
	}

	return false
}
