package d2automap

import "strings"

// String table ids of the automap text lines (AUTOMAP_DrawAutomap 0x4564a0, VERIFIED ids; the texts
// are looked up in the player's own string table, none are stored here).
const (
	StrGameName     = 0x1055 // "Game: " + game name
	StrPassword     = 0x1056 // "Password: " + password
	StrDifficultyFn = 0x1057 // difficulty line template with one %s
	StrHell         = 0x1422
	StrNightmare    = 0x1423
	StrPanelLine    = 0x58ca // line shown when a panel is open (0x455710); text UNVERIFIED, absent from string.tbl
	StrSpace        = 0xf9b  // " ", used by the gold ground label
)

// TextInput is what the automap text lines depend on.
type TextInput struct {
	GameName  string // empty in single player (the exe draws the line only when the join info has a name)
	Password  string
	LevelName string
	// Difficulty is 0 normal, 1 nightmare, 2 hell (DifficultyStringID; normal draws no line).
	Difficulty int
	// Host is the local host address line text (drawn by 0x455840 for panel layout 6 or 8). The
	// exe's format of that line is UNVERIFIED, so the caller passes the finished text.
	Host      string
	PanelOpen bool
}

// BuildLines returns the non-empty lines in the engine's order: game name, password, level name,
// version, difficulty, host, panel line. str resolves a string table id and returns "" when it is missing
// (a line whose text is missing is dropped).
func BuildLines(in TextInput, str func(id int) string) []string {
	var out []string

	if in.GameName != "" {
		if s := str(StrGameName); s != "" {
			out = append(out, s+in.GameName)
		}
	}

	if in.Password != "" {
		if s := str(StrPassword); s != "" {
			out = append(out, s+in.Password)
		}
	}

	out = append(out, in.LevelName, VersionText)

	if id := DifficultyStringID(in.Difficulty); id != 0 {
		name, tmpl := str(id), str(StrDifficultyFn)
		if name != "" && tmpl != "" {
			out = append(out, strings.Replace(tmpl, "%s", name, 1))
		}
	}

	if in.Host != "" {
		out = append(out, in.Host)
	}

	if in.PanelOpen {
		if s := str(StrPanelLine); s != "" {
			out = append(out, s)
		}
	}

	return out
}
