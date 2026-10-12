package d2gamescreen

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2skill"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
)

// heroHitMinTicks spaces the hit grunts (UNVERIFIED: the exe plays the class
// hit row on the hit-recovery mode start; this spacing is an engine choice).
const heroHitMinTicks = 12

// heroClassPrefix is the Sounds.txt voice prefix of the local hero.
func (v *Game) heroClassPrefix() string {
	if v.localPlayer == nil {
		return ""
	}

	code := d2hero.ClassCodeOfHero(v.localPlayer.Class)
	if code == "" {
		return ""
	}

	for _, p := range d2audio.HeroClassPrefixes {
		if strings.HasPrefix(p, code) {
			return p
		}
	}

	return ""
}

// soundIndexLookups returns the handle<->index lookups of Sounds.txt.
func (v *Game) soundIndexLookups() (func(string) (int, bool), func(int) string) {
	byIndex := map[int]string{}
	for h, e := range v.asset.Records.Sound.Details {
		byIndex[e.Index] = h
	}

	return func(h string) (int, bool) {
			e, ok := v.asset.Records.Sound.Details[h]
			if !ok {
				return 0, false
			}

			return e.Index, true
		}, func(i int) string {
			return byIndex[i]
		}
}

// sayHero plays hero voice id (d2audio.Voice*) with the exe's suppression
// rules (UNITSND_PlayUnitSoundById 0x4c8470). Whether a voice is still playing
// is approximated by the speech sound of the NPC dialogue (UNVERIFIED).
func (v *Game) sayHero(id int) {
	prefix := v.heroClassPrefix()
	if prefix == "" || v.localPlayer == nil {
		return
	}

	dead := v.death.dead
	if !v.heroVoiceGate.Allow(id, v.soundEngine.Ticks(), false, dead) {
		v.Infof("HEROVOICE id=%#x suppressed", id)
		return
	}

	indexOf, handleAt := v.soundIndexLookups()

	h, ok := d2audio.VoiceHandle(prefix, id, indexOf, handleAt)
	if !ok {
		return
	}

	v.Infof("HEROVOICE id=%#x handle=%s tick=%d", id, h, v.soundEngine.Ticks())
	v.playSoundAtPos(h, v.localPlayer.GetPosition(), d2audio.PlayOpts{
		Emitter: heroEmitter, Hero: true, Kind: "hero-voice", Who: v.localPlayer.Name(), Delay: d2audio.VoiceDelay(id),
	})
}

// sayHeroVocal plays the hit (slot 0) or death (slot 1) vocal of the class record.
func (v *Game) sayHeroVocal(death bool) {
	prefix := v.heroClassPrefix()
	if prefix == "" || v.localPlayer == nil {
		return
	}

	h, kind := prefix+"_hit_1", "hit"
	if death {
		h, kind = prefix+"_death_1", "death"
	} else if now := v.soundEngine.Ticks(); now-v.heroHitTick < heroHitMinTicks && v.heroHitTick != 0 {
		return
	} else {
		v.heroHitTick = now
	}

	v.Infof("HEROVOICE %s handle=%s", kind, h)
	v.playSoundAtPos(h, v.localPlayer.GetPosition(), d2audio.PlayOpts{
		Emitter: heroEmitter, Hero: true, Kind: "hero-voice", Who: v.localPlayer.Name(),
	})
}

// onSkillRefused maps a refused cast to the hero's complaint (needmana, not in town).
func (v *Game) onSkillRefused(reason string) {
	switch reason {
	case d2skill.ReasonMana:
		v.sayHero(d2audio.VoiceNeedMana)
	case d2skill.ReasonTown:
		v.sayHero(d2audio.VoiceNotInTown)
	}
}
