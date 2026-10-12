package d2monsters

import (
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2monreg"
	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2map/d2mapentity"
)

// RareNameKeys are the string table keys of the random name of a rare (unique, non super unique, non champion)
// monster: Format is the string id of the format (0x6b9 prefix + suffix, 0x6ba with the appellation), the keys name the
// words in the order the format takes them (prefix, suffix, appellation; Appellation is empty for the short form).
type RareNameKeys struct {
	Format                      int
	Prefix, Suffix, Appellation string
}

// RareNameOf rolls the name of a unique monster from its name seed, the exe's way (d2monreg.RollRareName), over the
// row order of the three name tables. The second result is false when the monster has no random name (a super unique,
// a champion, a monster that is not unique, or a class with a fixed name string) or the tables are not loaded.
func (d *Director) RareNameOf(m *d2mapentity.Monster) (RareNameKeys, bool) {
	if m == nil || d.asset == nil || d.asset.Records == nil || m.Stat == nil {
		return RareNameKeys{}, false
	}

	if m.TypeFlags&(d2mapentity.MonTypeSuperUnique|d2mapentity.MonTypeChampion) != 0 || !isUnique(m) {
		return RareNameKeys{}, false
	}

	if _, fixed := d2monreg.FixedNameString(m.Stat.ID); fixed {
		return RareNameKeys{}, false
	}

	n := &d.asset.Records.Monster.Name
	if len(n.PrefixOrder) == 0 || len(n.SuffixOrder) == 0 {
		return RareNameKeys{}, false
	}

	return rareNameKeys(m.UMod.NameSeed, n.PrefixOrder, n.SuffixOrder, d.asset.Records.Monster.Name.AppellationOrder), true
}

// rareNameKeys maps the roll onto the table rows.
func rareNameKeys(seed uint16, prefix, suffix, appellation []string) RareNameKeys {
	r := d2monreg.RollRareName(seed, len(prefix), len(suffix), len(appellation))

	out := RareNameKeys{Format: d2monreg.RareNameFormat1, Prefix: prefix[r.Prefix], Suffix: suffix[r.Suffix]}

	if r.Long && len(appellation) > 0 {
		out.Format = d2monreg.RareNameFormat2
		out.Appellation = appellation[r.Appellation]
	}

	return out
}
