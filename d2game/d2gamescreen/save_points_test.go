package d2gamescreen

import (
	"os"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2quest"
)

// The save points of the game all end in one writer: the client sends the save
// packet (OnPlayerSave, through saveBeforeExit) and the server writes the .d2s
// with HeroStateFactory.SaveD2S (d2core/d2hero.CommitD2S). These tests pin the
// points so that none is dropped by accident.

func TestAutosaveIntervalIs8192Frames(t *testing.T) {
	if got := autosaveIntervalSeconds * 25; got != 8192 {
		t.Fatalf("autosave every %v frames at 25 fps, want 8192", got)
	}
}

func TestBossKillsForceASave(t *testing.T) {
	for name, class := range map[string]int{"Diablo": d2quest.NPCDiablo, "Baal": d2quest.NPCBaalCrab} {
		if !d2quest.KillForcesSave(class) {
			t.Errorf("the kill of %s does not force a save", name)
		}
	}

	if d2quest.KillForcesSave(0) {
		t.Error("an ordinary monster forces a save")
	}
}

func TestSavePointsCallTheCommonSaver(t *testing.T) {
	tests := []struct {
		file string
		want string
		min  int
	}{
		{"autosave.go", "v.saveBeforeExit()", 1},   // periodic
		{"game_death.go", "v.saveBeforeExit()", 3}, // death, resurrection, return to town
		{"quests.go", "v.saveBeforeExit()", 1},     // Diablo and Baal
		{"game.go", "v.saveBeforeExit()", 1},       // exit
		{"game.go", "v.OnPlayerSave()", 1},         // the save packet
		{"autosave.go", "v.OnPlayerSave()", 1},     // saveBeforeExit -> the save packet
		{"hero_levelup.go", "v.OnPlayerSave()", 1}, // level up
		{"game_merc.go", "v.OnPlayerSave()", 2},    // mercenary hire and gear
	}

	for _, tt := range tests {
		src, err := os.ReadFile(tt.file)
		if err != nil {
			t.Fatal(err)
		}

		if got := strings.Count(string(src), tt.want); got < tt.min {
			t.Errorf("%s: %q appears %d times, want at least %d", tt.file, tt.want, got, tt.min)
		}
	}

	// and the server side: every level change, act change, waypoint, trade and idle kick reaches saveD2S
	srv, err := os.ReadFile("../../d2networking/d2server/game_server.go")
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.Count(string(srv), "g.saveD2S("); got < 4 {
		t.Errorf("game_server.go: %d calls of saveD2S, want at least 4 (save packet, act, level change, waypoint)", got)
	}

	if !strings.Contains(string(srv), "g.heroStateFactory.SaveD2S(state)") {
		t.Error("saveD2S no longer goes through HeroStateFactory.SaveD2S")
	}
}
