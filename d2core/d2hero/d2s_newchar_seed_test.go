package d2hero

import (
	"testing"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2enum"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2s"
)

func TestNewMapSeedNeverZero(t *testing.T) {
	tests := []struct {
		name  string
		draws []uint32
		want  uint32
	}{
		{"first draw", []uint32{7}, 7},
		{"skips zero", []uint32{0, 0, 9}, 9},
		{"only zeros", []uint32{0}, 1},
	}

	for _, tt := range tests {
		i := 0
		draw := func() uint32 {
			v := tt.draws[i%len(tt.draws)]
			i++

			return v
		}

		if got := NewMapSeed(draw); got != tt.want {
			t.Errorf("%s: seed %d, want %d", tt.name, got, tt.want)
		}
	}
}

// A new character's .d2s carries the map seed of its state, so the next load plays the same maps.
func TestNewCharacterD2SKeepsMapSeed(t *testing.T) {
	state := &HeroState{HeroName: "Seedy", HeroType: d2enum.HeroSorceress, Expansion: true, MapSeed: 0xCAFE1234}

	data, err := NewCharacterD2S(state, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}

	h, err := d2s.ParseHeader(data)
	if err != nil {
		t.Fatal(err)
	}

	if h.MapSeed != 0xCAFE1234 {
		t.Errorf("header map seed %#x, want 0xcafe1234", h.MapSeed)
	}

	state.MapSeed = 0
	data, _ = NewCharacterD2S(state, time.Unix(1700000000, 0))

	if h, _ = d2s.ParseHeader(data); h.MapSeed != 0 {
		t.Errorf("no seed in the state must leave the header field empty, got %#x", h.MapSeed)
	}
}

func TestEnsureMapSeed(t *testing.T) {
	f := &HeroStateFactory{}

	kept := &HeroState{MapSeed: 42}
	if f.EnsureMapSeed(kept) || kept.MapSeed != 42 {
		t.Errorf("a hero with a seed must keep it, got %d", kept.MapSeed)
	}

	fresh := &HeroState{} // no FilePath: nothing is written
	if !f.EnsureMapSeed(fresh) || fresh.MapSeed == 0 {
		t.Errorf("a hero without a seed must get one, got %d", fresh.MapSeed)
	}

	if f.EnsureMapSeed(nil) {
		t.Error("nil hero")
	}
}
