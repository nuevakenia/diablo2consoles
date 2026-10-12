package d2asset

import (
	"strings"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2tbl"
)

// Numeric string ids as the exe uses them (AUTOMAP_DrawAutomap 0x4564a0 and others): ids below 10000
// read string.tbl, 10000.. expansionstring.tbl and 20000.. patchstring.tbl, each by the node's own
// id (UNVERIFIED for the two offsets: the usual D2 convention, not read from the exe here; a real
// string.tbl check of ids 0x1055 0x1056 0x1057 0x1422 0x1423 0xf9b is VERIFIED).
const (
	stringIDExpansionBase = 10000
	stringIDPatchBase     = 20000
)

type idTable struct {
	base int
	m    d2tbl.IDStrings
}

// idTableBase gives the id offset of a table by its file name (0 for string.tbl).
func idTableBase(tablePath string) int {
	p := strings.ToLower(tablePath)

	switch {
	case strings.Contains(p, "patchstring"):
		return stringIDPatchBase
	case strings.Contains(p, "expansionstring"):
		return stringIDExpansionBase
	}

	return 0
}

// addIDTable registers the id-keyed view of a loaded table.
func (am *AssetManager) addIDTable(tablePath string, data []byte) {
	m, err := d2tbl.LoadIDStrings(data, am.Locale().Decode)
	if err != nil {
		am.Debugf("no id view of %s: %v", tablePath, err)
		return
	}

	am.idTables = append(am.idTables, idTable{base: idTableBase(tablePath), m: m})
}

// StringByID returns the text of a numeric string id (for example 0x1055, the automap "Game: " line)
// and whether it exists.
func (am *AssetManager) StringByID(id int) (string, bool) {
	for _, t := range am.idTables {
		if id < t.base {
			continue
		}

		if s, ok := t.m[id-t.base]; ok {
			return s, true
		}
	}

	return "", false
}
