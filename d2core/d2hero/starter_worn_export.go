package d2hero

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

// exportNewWorn writes the worn items that were made in the game (the club and
// buckler a new hero starts with, gear picked up and put on) into a save whose
// body location for them is empty. Without it a new hero's weapon and shield
// were lost at the first save: the file kept only the potions and scrolls and
// the hero came back bare-handed. A location that already holds an item is never
// replaced here (exportEquipment keeps the file's own item), so a hero whose
// file has its gear is left as it was. The items are inserted before the rest of
// the list, as the game writes the worn items first.
func exportNewWorn(c *d2s.Character, containers *HeroContainers, tables *d2s.ItemTables, ids *AffixIDs,
	warn func(string, ...interface{})) int {
	if containers == nil || !containers.EquippedSet || tables == nil || ids == nil {
		return 0
	}

	taken := map[int]bool{}

	for i := range c.Items {
		if it := &c.Items[i]; it.Location == d2s.LocationEquipped && !it.Ear {
			taken[int(it.Equipped)] = true
		}
	}

	var fresh []d2s.Item

	for i := range containers.Equipped {
		s := &containers.Equipped[i]
		if s.D2S != nil || taken[s.X] || s.X < 1 || s.X > 12 {
			continue
		}

		if s.Facts == nil {
			warn("worn item %q (slot %d) has no rolled values, not written", s.Code, s.X)
			continue
		}

		it, dropped, why := D2SItemFromStored(s, tables, ids)
		if why != "" {
			warn("worn item %q (slot %d) not written: %s", s.Code, s.X, why)
			continue
		}

		if dropped > 0 {
			warn("worn item %q: %d stat(s) the save cannot hold were left out", s.Code, dropped)
		}

		it.Location, it.Page, it.X, it.Y, it.Equipped = d2s.LocationEquipped, 0, 0, 0, uint8(s.X)
		taken[s.X] = true

		fresh = append(fresh, it)
	}

	if len(fresh) > 0 {
		c.Items = append(fresh, c.Items...)
	}

	return len(fresh)
}

// wornInFile says whether the save already holds a worn item with this code at
// the body location (an item written by exportNewWorn in an earlier save).
func wornInFile(c *d2s.Character, slot int, code string) (uint32, bool) {
	for i := range c.Items {
		it := &c.Items[i]
		if it.Location == d2s.LocationEquipped && int(it.Equipped) == slot && strings.TrimSpace(it.Code) == strings.TrimSpace(code) {
			return it.ID, true
		}
	}

	return 0, false
}
