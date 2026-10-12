package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// FormSound is one sound of a hero drawn as a monster (Druid Werewolf and
// Werebear), from the monster's MonSounds.txt row. The caller positions it.
type FormSound struct {
	Kind, Handle string
	DelayTicks   int
	Volume       int
}

// FormAttackSounds are the sounds of the form's attack mode (A1 or A2): the
// vocal with its probability and the weapon sound, as a monster plays them.
// UNVERIFIED that the hero's form uses the monster rows (see
// d2audio.FormMonSoundsID).
func FormAttackSounds(r *d2records.MonsterSoundRecord, mode d2monster.Mode, rnd func(int) int) []FormSound {
	return toFormSounds(attackPlans(r, mode, rnd))
}

// FormHitSound is the hit sound of the form (MonSounds HitSound).
func FormHitSound(r *d2records.MonsterSoundRecord) []FormSound {
	return toFormSounds(hitPlans(r))
}

func toFormSounds(plans []soundPlan) []FormSound {
	out := make([]FormSound, 0, len(plans))

	for _, p := range plans {
		out = append(out, FormSound{Kind: p.kind, Handle: p.handle, DelayTicks: p.delay, Volume: p.vol})
	}

	return out
}
