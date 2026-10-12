package d2gamescreen

import (
	"math/rand"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2monsters"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// Wave 36 sound wiring: objects, the town portal, the Druid forms. Rules and
// their UNVERIFIED parts are in d2audio/w36_sound.go.

// formSoundRecord is the MonSounds.txt row of the form the hero is in, nil in human shape.
func (v *Game) formSoundRecord() *d2records.MonsterSoundRecord {
	if v.localPlayer == nil || !v.localPlayer.InForm() {
		return nil
	}

	id := d2audio.FormMonSoundsID(v.localPlayer.FormToken())
	if id == "" {
		return nil
	}

	return v.asset.Records.Monster.Sounds[id]
}

// playFormSwing plays the attack sounds of the Werewolf / Werebear form and
// reports whether the hero is in a form (then no weapon swing is played).
func (v *Game) playFormSwing() bool {
	if v.localPlayer == nil || !v.localPlayer.InForm() {
		return false
	}

	rec := v.formSoundRecord()

	// nolint:gosec // sound variation only
	for _, s := range d2monsters.FormAttackSounds(rec, d2monster.ModeAttack1, rand.Intn) {
		v.playSoundAtPos(s.Handle, v.localPlayer.GetPosition(), d2audio.PlayOpts{
			Emitter: heroEmitter, Hero: true, Kind: "form-" + s.Kind, Who: v.localPlayer.Name(),
			Delay: s.DelayTicks, Volume: s.Volume,
		})
	}

	return true
}

// playObjectSound plays a Sounds.txt handle at an object.
func (v *Game) playObjectSound(handle string, ob *d2mapentity.Object, kind string) {
	if handle == "" || ob == nil {
		return
	}

	v.playSoundAtPos(handle, ob.GetPosition(), d2audio.PlayOpts{Kind: kind, Who: ob.Label()})
}

// playHeroSound plays a Sounds.txt handle at the hero (the town portal cast).
func (v *Game) playHeroSound(handle, kind string) {
	if handle == "" || v.localPlayer == nil {
		return
	}

	v.playSoundAtPos(handle, v.localPlayer.GetPosition(),
		d2audio.PlayOpts{Emitter: heroEmitter, Hero: true, Kind: kind, Who: v.localPlayer.Name()})
}
