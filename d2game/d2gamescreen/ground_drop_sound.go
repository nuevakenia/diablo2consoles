package d2gamescreen

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2audio"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/diablo2item"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// playGroundDrop plays the sounds of an item landing (ITEM_PlayGroundDropSound
// 0x4be190): item_flippy at once, then the item's drop sound after its
// dropsfxframe frames (12 by default; frame delay reading UNVERIFIED).
func (v *Game) playGroundDrop(it *diablo2item.Item, ent *d2mapentity.Item) {
	var base d2audio.ItemSoundRow

	if rec := it.CommonRecord(); rec != nil {
		base = d2audio.ItemSoundRow{Drop: v.soundIndex(rec.DropSound), Use: v.soundIndex(rec.UseSound),
			DropFrame: rec.DropSfxFrame}
	}

	var special *d2audio.ItemSoundRow

	if u := it.UniqueRecord(); u != nil {
		special = &d2audio.ItemSoundRow{Drop: v.soundIndex(u.DropSound), Use: v.soundIndex(u.UseSound),
			DropFrame: u.DropSfxFrame}
	} else if s := it.SetItemRecord(); s != nil {
		special = &d2audio.ItemSoundRow{Drop: v.soundIndex(s.DropSound), Use: v.soundIndex(s.UseSound),
			DropFrame: s.DropSfxFrame}
	}

	drop, _, frame := d2audio.ItemSounds(base, special)

	for _, g := range d2audio.GroundDropPlan(drop, frame) {
		v.playSoundAtPos(v.soundHandle(g.ID), ent.GetPosition(), d2audio.PlayOpts{Kind: "drop", Delay: g.Delay})
	}
}

// soundIndex returns the Sounds.txt index of a handle (0 when unknown).
func (v *Game) soundIndex(handle string) int {
	if e, ok := v.asset.Records.Sound.Details[handle]; ok {
		return e.Index
	}

	return 0
}

// soundHandle returns the Sounds.txt handle of an index ("" when unknown).
func (v *Game) soundHandle(index int) string {
	if v.soundHandles == nil {
		v.soundHandles = make(map[int]string, len(v.asset.Records.Sound.Details))

		for h, e := range v.asset.Records.Sound.Details {
			v.soundHandles[e.Index] = h
		}
	}

	return v.soundHandles[index]
}
