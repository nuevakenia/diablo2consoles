package d2automap

import (
	"reflect"
	"testing"
)

func fakeStr(id int) string {
	return map[int]string{
		StrGameName: "Game: ", StrPassword: "Password: ", StrDifficultyFn: "Difficulty: %s",
		StrHell: "Hell", StrNightmare: "Nightmare",
	}[id]
}

func TestBuildLines(t *testing.T) {
	tests := []struct {
		name string
		in   TextInput
		want []string
	}{
		{"single player normal", TextInput{LevelName: "Rogue Encampment"}, []string{"Rogue Encampment", VersionText}},
		{"nightmare", TextInput{LevelName: "Blood Moor", Difficulty: 1},
			[]string{"Blood Moor", VersionText, "Difficulty: Nightmare"}},
		{"hell with game and password", TextInput{GameName: "abc", Password: "pw", LevelName: "Harrogath", Difficulty: 2},
			[]string{"Game: abc", "Password: pw", "Harrogath", VersionText, "Difficulty: Hell"}},
		{"host and panel line missing text", TextInput{LevelName: "L", Host: "10.0.0.1", PanelOpen: true},
			[]string{"L", VersionText, "10.0.0.1"}},
		{"unknown difficulty draws nothing", TextInput{LevelName: "L", Difficulty: 7}, []string{"L", VersionText}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := BuildLines(tc.in, fakeStr); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildLinesStackGolden(t *testing.T) {
	width := func(s string) int { return len(s) * 8 }
	lines := BuildLines(TextInput{LevelName: "Lut Gholein", Difficulty: 2}, fakeStr)

	for _, w := range []int{1512, 1920, 3440} {
		got := TextStack(lines, w, width)
		if len(got) != 3 {
			t.Fatalf("w=%d: %d lines", w, len(got))
		}

		// right aligned 16 from the edge, rows 0x18, 0x28, 0x38
		for i, l := range got {
			if l.X != w-width(l.Text)-0x10 || l.Y != 0x18+0x10*i {
				t.Fatalf("w=%d line %d: %+v", w, i, l)
			}
		}
	}
}
