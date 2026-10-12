package d2drop

import (
	"reflect"
	"testing"
)

// The moduli of every roll of ITEMGEN_RollDropQuality (556690), re-read in
// wave 23: with all tests failing (roll 255) the sequence of moduli is
// unique, set, rare, magic, superior, normal.
func TestRollQualityModuli(t *testing.T) {
	r := &Ratio{
		Unique:    DropRatio{Base: 100, Divisor: 10, Min: 50},
		Set:       DropRatio{Base: 50, Divisor: 5, Min: 20},
		Rare:      DropRatio{Base: 40, Divisor: 4, Min: 10},
		Magic:     DropRatio{Base: 20, Divisor: 2, Min: 5},
		HiQuality: DropRatio{Base: 8, Divisor: 3},
		Normal:    DropRatio{Base: 6, Divisor: 2},
	}
	// d = 12: unique (100-1)*128, set (50-2)*128, rare (40-3)*128,
	// magic (20-6)*128, superior (8-4)*128, normal (6-6)*128 <= 0
	rng := &scriptRNG{vals: []uint32{255, 255, 255, 255, 255}}
	q := RollQuality(rng, r, QualityInput{ILvl: 20, QLvl: 8, TypeRare: true})

	if q != QualityNormal {
		t.Fatalf("quality %v, want normal (normal chance <= 0)", q)
	}

	want := []int32{99 * 128, 48 * 128, 37 * 128, 14 * 128, 4 * 128}
	if !reflect.DeepEqual(rng.asked, want) {
		t.Fatalf("moduli %v, want %v", rng.asked, want)
	}
}

// Magic find below -99 skips the four tests; with magic find the unique test
// divides by the diminished value (ApplyMagicFindDiminishing(250)) and the
// magic test by 100+MF without diminishing.
func TestRollQualityMagicFind(t *testing.T) {
	r := &Ratio{
		Unique: DropRatio{Base: 100, Divisor: 10, Min: 0}, Set: DropRatio{Base: 100, Divisor: 10},
		Rare: DropRatio{Base: 100, Divisor: 10}, Magic: DropRatio{Base: 20, Divisor: 2},
		HiQuality: DropRatio{Base: 8, Divisor: 3}, Normal: DropRatio{Base: 6, Divisor: 2},
	}

	rng := &scriptRNG{vals: []uint32{255, 255}}
	RollQuality(rng, r, QualityInput{ILvl: 10, QLvl: 10, MagicFind: -100})

	if want := []int32{8 * 128, 6 * 128}; !reflect.DeepEqual(rng.asked, want) {
		t.Fatalf("MF -100: moduli %v, want %v", rng.asked, want)
	}

	// MF 300: x = 400, diminished by 250: 300*250/550+100 = 236; 100*12800/236
	rng = &scriptRNG{vals: []uint32{255, 255, 255, 255}}
	RollQuality(rng, r, QualityInput{ILvl: 10, QLvl: 10, MagicFind: 300, TypeRare: true})

	if got := rng.asked[0]; got != int32(100*12800/236) {
		t.Fatalf("unique modulus %d, want %d", got, 100*12800/236)
	}
	// set: factor 500: 300*500/800+100 = 287; rare: factor 600: 300*600/900+100 = 300
	if rng.asked[1] != int32(100*12800/287) || rng.asked[2] != int32(100*12800/300) {
		t.Fatalf("set/rare moduli %v", rng.asked[1:3])
	}
	// magic: no diminishing, 100+MF = 400
	if rng.asked[3] != int32(20*12800/400) {
		t.Fatalf("magic modulus %d", rng.asked[3])
	}
}

func TestRequestItemFlagsForceEthereal(t *testing.T) {
	// the roll is still taken; only the outcome is forced (554d90)
	if flagEthereal != 0x400000 || flagSocketed != 0x800 {
		t.Fatal("flag values")
	}
}
