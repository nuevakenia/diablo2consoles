package d2audio

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2records"
)

type tsv struct {
	col  map[string]int
	rows [][]string
}

func readTSV(t *testing.T, path string) tsv {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip(err)
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := tsv{col: map[string]int{}}

	for i, h := range strings.Split(lines[0], "\t") {
		out.col[h] = i
	}

	for _, ln := range lines[1:] {
		if ln != "" {
			out.rows = append(out.rows, strings.Split(ln, "\t"))
		}
	}

	return out
}

func (c tsv) num(r []string, name string) int {
	i, ok := c.col[name]
	if !ok || i >= len(r) {
		return 0
	}

	n, _ := strconv.Atoi(r[i])

	return n
}

func (c tsv) str(r []string, name string) string {
	i, ok := c.col[name]
	if !ok || i >= len(r) {
		return ""
	}

	return r[i]
}

// TestRealLevelAudioSelection checks, against the real Levels.txt,
// SoundEnviron.txt and Sounds.txt (from $D2_TABLES, skipped when unset), the
// chain the exe walks for the hero's level: Levels.txt SoundEnv ->
// SoundEnviron row -> Song / Day or Night Ambience / Day or Night Event, and
// the invariants that make it playable. Handles only are compared.
func TestRealLevelAudioSelection(t *testing.T) {
	dir := os.Getenv("D2_TABLES")
	if dir == "" {
		t.Skip("D2_TABLES not set")
	}

	levels := readTSV(t, filepath.Join(dir, "drlg", "patch_d2", "Levels.txt"))
	envTxt := readTSV(t, filepath.Join(dir, "sound", "patch_d2", "SoundEnviron.txt"))
	sounds := readTSV(t, filepath.Join(dir, "sound", "patch_d2", "Sounds.txt"))

	handle := map[int]string{}
	music := map[int]bool{}

	for _, r := range sounds.rows {
		idx := sounds.num(r, "Index")
		handle[idx] = sounds.str(r, "Sound")
		music[idx] = sounds.num(r, "Music Vol") != 0
	}

	envs := map[int]*d2records.SoundEnvironRecord{}

	for _, r := range envTxt.rows {
		rec := &d2records.SoundEnvironRecord{
			Handle: envTxt.str(r, "Handle"), Index: envTxt.num(r, "Index"), Song: envTxt.num(r, "Song"),
			DayAmbience: envTxt.num(r, "Day Ambience"), NightAmbience: envTxt.num(r, "Night Ambience"),
			DayEvent: envTxt.num(r, "Day Event"), NightEvent: envTxt.num(r, "Night Event"),
			EventDelay: envTxt.num(r, "Event Delay"), Indoors: envTxt.num(r, "Indoors"),
		}
		envs[rec.Index] = rec
	}

	type want struct{ song, dayAmb, nightAmb, dayEv, nightEv string }

	// level id -> expected handles (town vs field vs dungeon vs act bosses)
	expect := map[int]want{
		1:   {"music_town_1", "scene_wilderness_day", "scene_wilderness_night", "", ""},
		2:   {"music_wilderness", "scene_wilderness_day", "scene_wilderness_night", "event_wild_day_1", "event_wild_night_1"},
		8:   {"music_caves", "scene_cave", "scene_cave", "event_cave_1", "event_cave_1"},
		21:  {"music_crypt", "scene_crypt", "scene_crypt", "event_crypt_1", "event_crypt_1"},
		38:  {"music_tristram", "scene_wilderness_day", "scene_wilderness_night", "event_wild_day_1", "event_wild_night_1"},
		40:  {"music_town_2", "scene_desert_day", "scene_desert_night", "event_town_2_day_1", "event_town_2_night_1"},
		50:  {"music_harem", "scene_harem", "scene_harem", "event_harem_1", "event_harem_1"},
		75:  {"music_town_3", "scene_town_3_day", "scene_town_3_night", "event_town_3_day_1", "event_town_3_night_1"},
		103: {"music_town_4", "scene_mesa", "scene_mesa", "", ""},
		109: {"music_xtown", "scene_wilderness_day", "scene_wilderness_night", "", ""},
		131: {"music_xbaal", "scene_catacombs", "scene_catacombs", "event_catacombs_1", "event_catacombs_1"},
	}

	byID := map[int][]string{}
	for _, r := range levels.rows {
		byID[levels.num(r, "Id")] = r
	}

	for id, w := range expect {
		row := byID[id]
		if row == nil {
			t.Errorf("level %d missing", id)
			continue
		}

		env := envs[levels.num(row, "SoundEnv")]
		if env == nil {
			t.Errorf("level %d: no SoundEnviron row", id)
			continue
		}

		for _, night := range []bool{false, true} {
			sel := Select(env, night)
			got := want{handle[sel.Song], handle[env.DayAmbience], handle[env.NightAmbience], handle[env.DayEvent], handle[env.NightEvent]}

			if got.song != w.song || got != w {
				t.Errorf("level %d (env %s): got %+v, want %+v", id, env.Handle, got, w)
			}

			wantAmb, wantEv := w.dayAmb, w.dayEv
			if night {
				wantAmb, wantEv = w.nightAmb, w.nightEv
			}

			if handle[sel.Ambience] != wantAmb || handle[sel.Event] != wantEv {
				t.Errorf("level %d night=%v: selection ambience %q event %q, want %q %q", id, night,
					handle[sel.Ambience], handle[sel.Event], wantAmb, wantEv)
			}
		}
	}

	// invariants for every level with a SoundEnv
	for _, r := range levels.rows {
		id := levels.num(r, "Id")

		env := envs[levels.num(r, "SoundEnv")]
		if env == nil {
			t.Errorf("level %d: SoundEnv %d has no SoundEnviron row", id, levels.num(r, "SoundEnv"))
			continue
		}

		if env.Song != 0 && !music[env.Song] {
			t.Errorf("level %d env %s: song %d is not a Music Vol row", id, env.Handle, env.Song)
		}

		for _, idx := range []int{env.DayAmbience, env.NightAmbience, env.DayEvent, env.NightEvent} {
			if idx != 0 && handle[idx] == "" {
				t.Errorf("level %d env %s: sound %d not in Sounds.txt", id, env.Handle, idx)
			}

			if idx != 0 && music[idx] {
				t.Errorf("level %d env %s: ambience/event %d is a music row", id, env.Handle, idx)
			}
		}

		if (env.DayEvent != 0 || env.NightEvent != 0) && env.EventDelay <= 0 {
			t.Errorf("level %d env %s: events without an Event Delay", id, env.Handle)
		}
	}
}
