package d2loadscreen

import "testing"

func TestLayout(t *testing.T) {
	l := LayoutFor(800, 600)

	if l.PictureX != 400-128 || l.PictureBottomY != 300+128 {
		t.Errorf("picture %d,%d", l.PictureX, l.PictureBottomY)
	}

	if l.Frame != (Rect{400 - 130, 300 + 158, 400 + 130, 300 + 178}) || l.Fill != (Rect{400 - 128, 300 + 160, 400 + 128, 300 + 176}) {
		t.Errorf("rects %+v %+v", l.Frame, l.Fill)
	}

	if l.Fill.R-l.Fill.L != BarMax || l.Fill.B-l.Fill.T != l.BarHeight {
		t.Errorf("fill %+v is not %d x %d", l.Fill, BarMax, l.BarHeight)
	}

	// the frame is the fill grown by 2 pixels on every side
	if l.Frame.L != l.Fill.L-2 || l.Frame.R != l.Fill.R+2 || l.Frame.T != l.Fill.T-2 || l.Frame.B != l.Fill.B+2 {
		t.Errorf("frame %+v around %+v", l.Frame, l.Fill)
	}

	if PathFor(true) == PathFor(false) {
		t.Error("paths")
	}
}

func TestShowsBar(t *testing.T) {
	for mode := 0; mode < 12; mode++ {
		want := mode >= 6 && mode <= 9
		if ShowsBar(mode, 0) != want {
			t.Errorf("mode %d", mode)
		}
	}

	if ShowsBar(7, 1) {
		t.Error("bar with a non-zero session value")
	}
}

func TestFrames(t *testing.T) {
	f := Frames{Count: 3}

	var got []int
	for i := 0; i < 6; i++ {
		got = append(got, f.Draw(true))
	}

	// 0 1 2 then the last frame holds
	want := []int{0, 1, 2, 2, 2, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("frames %v, want %v", got, want)
		}
	}

	g := Frames{Count: 3}
	if g.Draw(false) != 0 || g.Draw(false) != 0 {
		t.Error("a draw without advance moved on")
	}
}

func TestSaveSendRatio(t *testing.T) {
	// total 4 chunks: each chunk adds 64, the 8 steps of a chunk add 8 each
	if SaveSendRatio(0, 4, 0) != 0 || SaveSendRatio(0, 4, 7) != 0x700/32 {
		t.Error("start")
	}

	prev := -1
	for chunk := 0; chunk < 4; chunk++ {
		for step := 0; step < SaveSendSteps; step++ {
			r := SaveSendRatio(chunk, 4, step)
			if r < prev {
				t.Fatalf("ratio fell at chunk %d step %d: %d < %d", chunk, step, r, prev)
			}

			prev = r
		}
	}

	if SaveSendRatio(4, 4, 0) != BarMax {
		t.Errorf("done = %d, want %d", SaveSendRatio(4, 4, 0), BarMax)
	}

	if SaveSendRatio(1, 0, 0) != 0 {
		t.Error("no chunks")
	}

	if SaveSendRatio(2, 4, 0) != BarMax/2 {
		t.Errorf("half = %d", SaveSendRatio(2, 4, 0))
	}
}

func TestStagesOrder(t *testing.T) {
	draws := 0
	first := -1

	for i, s := range Stages {
		if s.Draws {
			draws++

			if first < 0 {
				first = i
			}
		}
	}

	if draws != 3 {
		t.Errorf("%d draw points", draws)
	}

	// the first picture is drawn after its cel is loaded and before the save upload
	if Stages[first-1].Name != "load loading screen cel and palette" || Stages[first+1].Name != "send local save to server" {
		t.Errorf("stage %d neighbours", first)
	}
}
