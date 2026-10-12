package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monster"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2resource"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2asset"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

// prefetchModeOrder is the order the game first asks for the modes of a monster: it walks, gets hit,
// attacks, dies. Only the modes a class has (monstats2 m* columns) are decoded.
var prefetchModeOrder = []d2enum.MonsterAnimationMode{
	d2enum.MonsterAnimationModeNeutral,
	d2enum.MonsterAnimationModeWalk,
	d2enum.MonsterAnimationModeGetHit,
	d2enum.MonsterAnimationModeAttack1,
	d2enum.MonsterAnimationModeDeath,
	d2enum.MonsterAnimationModeRun,
	d2enum.MonsterAnimationModeAttack2,
	d2enum.MonsterAnimationModeCast,
	d2enum.MonsterAnimationModeSkill1,
	d2enum.MonsterAnimationModeSkill2,
	d2enum.MonsterAnimationModeSkill3,
	d2enum.MonsterAnimationModeSkill4,
	d2enum.MonsterAnimationModeBlock,
	d2enum.MonsterAnimationModeDead,
}

// prefetchRequest turns a monstats row into the animation files to decode, or false when the row has no
// animation data.
func (d *Director) prefetchRequest(stat *d2records.MonStatRecord) (d2asset.MonsterPrefetch, bool) {
	if stat == nil {
		return d2asset.MonsterPrefetch{}, false
	}

	statEx := d.asset.Records.Monster.Stats2[stat.ExtraDataKey]
	if statEx == nil {
		return d2asset.MonsterPrefetch{}, false
	}

	req := d2asset.MonsterPrefetch{
		Token:       stat.AnimationDirectoryToken,
		WeaponClass: statEx.BaseWeaponClass,
		Palette:     d2resource.PaletteUnits,
		Equipment:   statEx.EquipmentOptions,
	}

	for _, m := range prefetchModeOrder {
		if statEx.HasAnimationMode[m] {
			req.Modes = append(req.Modes, m)
		}
	}

	return req, true
}

// Prefetch decodes the animations of the given monster types ahead of their first use: the neutral mode
// before it returns when syncNeutral is set (the spawn needs it), the other modes on background goroutines, so the first attack,
// hit or death of a type does not stall a frame. It changes no game state and draws no random number.
func (d *Director) Prefetch(stats []*d2records.MonStatRecord, syncNeutral bool) {
	if !d2asset.PrefetchEnabled() {
		return
	}

	var reqs []d2asset.MonsterPrefetch

	for _, st := range stats {
		if r, ok := d.prefetchRequest(st); ok {
			reqs = append(reqs, r)
		}
	}

	d.asset.PrefetchMonsters(reqs, syncNeutral)
}

// prefetchTypes prefetches the drawn types of a level and the followers they bring.
func (d *Director) prefetchTypes(types []d2monster.ClassInfo) {
	var stats []*d2records.MonStatRecord

	for _, ci := range types {
		for _, id := range []int{ci.Class, ci.Minion1, ci.Minion2} {
			if st := d.statByID[id]; st != nil {
				stats = append(stats, st)
			}
		}
	}

	d.Prefetch(stats, false)
}
