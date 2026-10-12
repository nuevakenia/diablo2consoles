package diablo2item

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2automap"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2item/d2drop"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2ui"
)

// groundQuality is the quality number the label colour rule reads. A rolled item carries the quality
// it was made with; other items derive it from their records (the same order as nameColorToken). A
// runeword counts as unique (gold), as rolledColor does.
func (i *Item) groundQuality() int {
	if i.runeword != "" {
		return int(d2drop.QualityUnique)
	}

	if i.rolled != nil || i.quality != d2drop.QualityNone {
		return int(i.quality)
	}

	affixes := len(i.PrefixRecords()) + len(i.SuffixRecords())

	switch {
	case i.attributes != nil && i.attributes.crafted:
		return int(d2drop.QualityCrafted)
	case i.SetItemRecord() != nil:
		return int(d2drop.QualitySet)
	case i.UniqueRecord() != nil:
		return int(d2drop.QualityUnique)
	case affixes > maxAffixesOnMagicItem:
		return int(d2drop.QualityRare)
	case affixes > 0:
		return int(d2drop.QualityMagic)
	}

	return int(d2drop.QualityNormal)
}

// isRuneType is ITEM_IsOfType(item, 0x4a) of the label routine; 0x4a is taken to be the "rune" type
// (UNVERIFIED which type id 0x4a is).
func (i *Item) isRuneType() bool {
	if i.TypeCode == "rune" {
		return true
	}

	if i.factory == nil || i.factory.asset == nil || i.factory.asset.Records == nil {
		return false
	}

	for _, t := range i.factory.asset.Records.Item.Equivalency["rune"] {
		if t != nil && t.Code == i.TypeCode {
			return true
		}
	}

	return false
}

// GroundLabelInput is the input of the ground label colour rule (INV_DrawGroundItemLabels 0x4bd100).
func (i *Item) GroundLabelInput() d2automap.GroundItem {
	return d2automap.GroundItem{
		Quality:  i.groundQuality(),
		Socketed: i.NumSockets() > 0 || len(i.SocketCodes) > 0,
		Ethereal: i.IsEthereal(),
		Quest:    i.IsQuestItem(),
		Code:     i.CommonCode,
		IsRune:   i.isRuneType(),
	}
}

// GroundLabelColor is the engine colour index of the ground label of the item.
func (i *Item) GroundLabelColor() int { return d2automap.LabelColor(i.GroundLabelInput()) }

// GroundLabelText is the item name coloured by the ground label rule (not the inventory name colour).
func (i *Item) GroundLabelText() string {
	return d2ui.ColorTokenize(i.plainLabel(), d2ui.ColorTokenForIndex(i.GroundLabelColor()))
}
