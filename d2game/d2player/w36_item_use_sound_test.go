package d2player

import "testing"

type useSoundItem struct {
	InventoryItem
	handle string
}

func (s useSoundItem) UseSoundHandle() string { return s.handle }

func TestItemUseSound(t *testing.T) {
	var got []string

	SetItemSoundHook(func(handle, kind string) { got = append(got, kind+":"+handle) })
	t.Cleanup(func() { SetItemSoundHook(nil) })

	itemUseSound(useSoundItem{handle: "item_scroll"}, "item-use")
	itemUseSound(useSoundItem{}, "item-use")           // no use sound row
	itemUseSound(soundItem{handle: "item_sword"}, "x") // an item without the method
	itemUseSound(nil, "item-use")

	if len(got) != 1 || got[0] != "item-use:item_scroll" {
		t.Fatalf("got %v, want only the scroll's use sound", got)
	}
}
