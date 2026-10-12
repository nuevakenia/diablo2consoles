package d2skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tableRow returns the columns of row n (0 based, below the header) of a tab separated table in $D2_TABLES.
func tableRow(t *testing.T, file string, n int) []string {
	t.Helper()

	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	b, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		t.Skip(err)
	}

	lines := strings.Split(string(b), "\n")
	if n+1 >= len(lines) {
		t.Fatalf("%s has no row %d", file, n)
	}

	return strings.Split(strings.TrimRight(lines[n+1], "\r"), "\t")
}

// TestExeTableRows pins the row identities the exe's numeric ids are mapped through (item type ids and state ids are
// the row numbers of ItemTypes.txt / states.txt; the convention is confirmed by 0x16 scroll, 0x29 key, 0x39 blunt and
// state 24 slowed, see the w27 / w36 notes).
func TestExeTableRows(t *testing.T) {
	for _, c := range []struct {
		file string
		row  int
		col  int
		want string
	}{
		{"ItemTypes.txt", 0x2d, 1, "weap"}, {"ItemTypes.txt", 0x2e, 1, "mele"}, {"ItemTypes.txt", 0x16, 1, "scro"},
		{"states.txt", 12, 0, "STATE_INFERNO"}, {"states.txt", 24, 0, "STATE_SLOWED"},
		{"states.txt", 0x36, 0, "STATE_UNINTERRUPTABLE"},
	} {
		if got := tableRow(t, c.file, c.row)[c.col]; got != c.want {
			t.Errorf("%s row %d col %d = %q, want %q", c.file, c.row, c.col, got, c.want)
		}
	}
}
