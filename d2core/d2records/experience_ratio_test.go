package d2records

import (
	"os"
	"strings"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2fileformats/d2txt"
	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2herostats"
)

func expRow(level, ratio string) string {
	return level + strings.Repeat("\t0", 7) + "\t" + ratio + "\n"
}

// TestExperienceLoaderInstallsRatio: a table with the ExpRatio column (patch_d2
// has it, d2exp does not) installs the kill-experience ratio; one without it
// installs nothing.
func TestExperienceLoaderInstallsRatio(t *testing.T) {
	defer d2herostats.SetExpRatio(nil)

	head := "Level\tAmazon\tSorceress\tNecromancer\tPaladin\tBarbarian\tDruid\tAssassin"
	withCol := head + "\tExpRatio\nMaxLvl" + strings.Repeat("\t99", 7) + "\t10\n" +
		expRow("0", "1024") + expRow("1", "1024") + expRow("2", "512")

	if err := experienceLoader(&RecordManager{}, d2txt.LoadDataDictionary([]byte(withCol))); err != nil {
		t.Fatal(err)
	}

	if got := d2herostats.ExpRatioAt(2048, 2); got != 1024 {
		t.Errorf("level 2 ratio 512: got %d want 1024", got)
	}

	if got := d2herostats.ExpRatioAt(2048, 1); got != 2048 {
		t.Errorf("level 1 ratio 1024: got %d want 2048", got)
	}

	noCol := head + "\nMaxLvl" + strings.Repeat("\t99", 7) + "\n0" + strings.Repeat("\t0", 7) + "\n"

	if err := experienceLoader(&RecordManager{}, d2txt.LoadDataDictionary([]byte(noCol))); err != nil {
		t.Fatal(err)
	}

	if got := d2herostats.ExpRatioAt(2048, 2); got != 2048 {
		t.Errorf("no column: got %d want 2048", got)
	}
}

// TestExperienceRatioRealTable checks the shipped patch_d2 table when
// EXP_RATIO_FILE points at it: 1024 up to level 69, 976 at 70, 5 at 99.
func TestExperienceRatioRealTable(t *testing.T) {
	p := os.Getenv("EXP_RATIO_FILE")
	if p == "" {
		t.Skip("EXP_RATIO_FILE not set")
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	defer d2herostats.SetExpRatio(nil)

	if err := experienceLoader(&RecordManager{}, d2txt.LoadDataDictionary(b)); err != nil {
		t.Fatal(err)
	}

	for lvl, want := range map[int]int{30: 100000, 69: 100000, 70: 95312, 98: 585} {
		if got := d2herostats.ExpRatioAt(100000, lvl); got != want {
			t.Errorf("level %d: got %d want %d", lvl, got, want)
		}
	}
}
