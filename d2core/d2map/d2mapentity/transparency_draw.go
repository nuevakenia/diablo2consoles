package d2mapentity

import "github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"

// More of the equipment draw mode chooser (GFX_ResolveEquipComponentCodeA 0x4d8090, w37):
//
//  1. a player with unit stat 0xb5 != 0 draws every component at mode 1 (50 percent);
//  2. a monster with stat 0xb5 > 0 draws at mode 1 when the stat is 1..15, and when it is above 15 only
//     while one of its inventory items is ethereal (flag 0x400000);
//  3. otherwise an item whose Items.txt row has transparent != 0 draws at mode transtbl (the stock data
//     has one such row: the Torch, transtbl 3 = d2enum.DrawEffectModulate);
//  4. otherwise the ethereal weapon/shield rule (EtherealLayerEffect).
//
// The stat 0xb5 name is not known (UNVERIFIED which skill or state sets it).
const statTransparencyID = 0xb5

// StatTransparency applies rules 1 and 2: whether the unit forces mode 1 on all components.
func StatTransparency(isPlayer bool, stat int, hasEtherealItem bool) bool {
	if isPlayer {
		return stat != 0
	}

	switch {
	case stat <= 0:
		return false
	case stat > 15: //nolint:gomnd // exe constant
		return hasEtherealItem
	}

	return true
}

// ItemTransparentEffect applies rule 3: the draw effect of an Items.txt row with transparent set. The
// transtbl value is the exe draw mode number (same order as d2enum.DrawEffect); out of range gives false.
func ItemTransparentEffect(transparent bool, transtbl int) (d2enum.DrawEffect, bool) {
	if !transparent || transtbl < 0 || transtbl > int(d2enum.DrawEffectMod2X) {
		return 0, false
	}

	return d2enum.DrawEffect(transtbl), true
}

// allLayersTransparent sets mode 1 on every layer (rule 1).
func allLayersTransparent(out map[d2enum.CompositeType]d2enum.DrawEffect) map[d2enum.CompositeType]d2enum.DrawEffect {
	if out == nil {
		out = map[d2enum.CompositeType]d2enum.DrawEffect{}
	}

	for l := d2enum.CompositeType(0); l < d2enum.CompositeTypeMax; l++ {
		out[l] = d2enum.DrawEffectPctTransparency50
	}

	return out
}
