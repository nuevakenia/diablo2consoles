package d2player

import "testing"

// Golden of the escape menu pages (800x600, widest pentagram frame 54): row tops and the pentagram y.
func TestGameMenuPagesGolden(t *testing.T) {
	// main page equals the old plan
	old := Mode800.GameMenuPlan(3, 3, 54)
	got := GameMenuMain.Plan(Mode800, 0, 54)

	for i := range old {
		if old[i] != got[i] {
			t.Errorf("main row %d: %+v vs %+v", i, got[i], old[i])
		}
	}

	// (600-80)/2 - 45*8/2 = 260 - 180 = 80, pitch 45, pentagram 49 below the row top
	snd := GameMenuSound.Plan(Mode800, 0, 54)
	if len(snd) != 8 || snd[0].RowTop != 80 || snd[7].RowTop != 395 || snd[0].PentY != 129 || snd[1].LeftX != 97 || snd[1].RightX != 649 {
		t.Fatalf("sound plan %+v", snd)
	}

	// 7 rows: 260 - (45*7)/2 = 260 - 157 = 103
	au := GameMenuAutomap.Plan(Mode800, 0, 54)
	if len(au) != 7 || au[0].RowTop != 103 || au[6].RowTop != 373 {
		t.Fatalf("automap plan first %d last %d", au[0].RowTop, au[6].RowTop)
	}

	if r := GameMenuVideo.RowTop(Mode640, 0); r != (480-0x50)/2-45*8/2 {
		t.Fatalf("640 video row 0: %d", r)
	}

	// Go layouts map onto the exe pages: the option rows start after the title row
	p, rows := gameMenuPageFor(soundOptionsLayoutID, 7)
	if p.Name != "sound" || rows != 8 || p.FirstActionRow != 1 {
		t.Fatalf("sound page %v %d", p.Name, rows)
	}

	p, rows = gameMenuPageFor(optionsLayoutID, 7)
	if p.Name != "options" || rows != 7 {
		t.Fatalf("options page %v %d", p.Name, rows)
	}
}
