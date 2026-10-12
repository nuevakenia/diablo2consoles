package d2player

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// partyTwoPlayerLines is the party panel of a game with two players: the panel lists the OTHER players, so one row
// (row 0) is on screen, plus the header hover (a hero that has a party) and the close button. Both modes.
func partyTwoPlayerLines() []string {
	var out []string

	for _, m := range []struct {
		name string
		mode Mode
	}{{"800x600", Mode800}, {"640x480", Mode640}} {
		out = append(out, fmt.Sprintf("%s %s", m.name, m.mode.PartyHeaderHover()))
		out = append(out, fmt.Sprintf("%s %s", m.name, m.mode.PartyCloseRect()))

		for _, r := range m.mode.PartyRowRects(0) {
			out = append(out, fmt.Sprintf("%s %s", m.name, r))
		}
	}

	return out
}

// TestPartyPanelTwoPlayersGolden compares the two-player party panel layout with testdata/party_panel_two.golden
// (rectangles only; the numbers come from UI_RebuildPartyList, see party_layout.go). Regenerate with
// UPDATE_GOLDEN=1 only after re-reading the numbers from the executable.
func TestPartyPanelTwoPlayersGolden(t *testing.T) {
	path := filepath.Join("testdata", "party_panel_two.golden")
	got := strings.Join(partyTwoPlayerLines(), "\n") + "\n"

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(want) != got {
		t.Fatalf("party panel differs from the golden:\n got:\n%s want:\n%s", got, want)
	}
}
