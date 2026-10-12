// Package d2palshift reads the per-monster-class Palshift.dat (data\global\monsters\<token>\cof\)
// and applies a row to a palette.
//
// Format, measured on the real files (every class checked has exactly 2048 bytes): 8 rows of 256
// bytes, each row a remap of palette indexes (some rows are the identity). The exe
// (GFX_GetMonsterSpecialPaletteForUnit 0x473260) hands the row pointer of the unit's colour-shift
// index n (a per-unit value) to the draw call: rows 2..7 for ordinary classes, rows 0 and 1 only for
// two special classes (monster ids 0x16b and 0x16c); a global table is used for n >= 8. Which n
// a spawned monster gets (difficulty palette, unique RandTransforms, TransLvl + 2) is not traced:
// UNVERIFIED. The remap direction (shown colour = palette[row[index]]) is UNVERIFIED too.
package d2palshift

import "errors"

const (
	// Rows is the number of remap rows in a file.
	Rows = 8
	// RowSize is the size of one row.
	RowSize = 256
)

// Table is a loaded Palshift.dat.
type Table [Rows][RowSize]byte

// ErrSize is returned for a file that is not exactly 8 rows long.
var ErrSize = errors.New("palshift.dat must be 2048 bytes")

// Load parses a Palshift.dat.
func Load(data []byte) (*Table, error) {
	if len(data) != Rows*RowSize {
		return nil, ErrSize
	}

	t := &Table{}
	for i := range t {
		copy(t[i][:], data[i*RowSize:])
	}

	return t, nil
}

// Row returns the remap of colour-shift index n and whether the class has one for it: only
// 0..7 live in the file (see the package comment for the other cases).
func (t *Table) Row(n int) (*[RowSize]byte, bool) {
	if t == nil || n < 0 || n >= Rows {
		return nil, false
	}

	return &t[n], true
}

// Apply returns the RGB palette seen through a row: out[i] = pal[row[i]].
func Apply(pal *[RowSize][3]uint8, row *[RowSize]byte) [RowSize][3]uint8 {
	var out [RowSize][3]uint8

	for i := range out {
		out[i] = pal[row[i]]
	}

	return out
}
