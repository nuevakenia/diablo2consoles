package d2automap

import "testing"

var labelResolutions = []struct {
	name string
	w, h int
}{
	{"1512x982", 1512, 982},
	{"1920x1080", 1920, 1080},
	{"3440x1440", 3440, 1440},
}

func TestLayoutLabelsEdges(t *testing.T) {
	for _, res := range labelResolutions {
		t.Run(res.name, func(t *testing.T) {
			reqs := []LabelRequest{
				{ItemX: 2, ItemY: 300, TextW: 80, TextH: 16},          // left edge
				{ItemX: res.w - 3, ItemY: 300, TextW: 80, TextH: 16},  // right edge
				{ItemX: 500, ItemY: 0, TextW: 80, TextH: 16},          // top
				{ItemX: 700, ItemY: res.h - 20, TextW: 80, TextH: 16}, // bottom
			}

			got := LayoutLabels(reqs, 0, res.w, res.h)
			if len(got) != 4 {
				t.Fatalf("got %d boxes", len(got))
			}

			if got[0].L != 6 {
				t.Errorf("left clamp: %+v", got[0])
			}

			if got[1].R != res.w-1 || got[1].R-got[1].L != 88 {
				t.Errorf("right clamp: %+v", got[1])
			}

			if got[2].T != 6 {
				t.Errorf("top clamp: %+v", got[2])
			}

			if got[3].B != res.h-0x30-1 && got[3].B != res.h-0x30 {
				t.Errorf("bottom clamp: %+v (limit %d)", got[3], res.h-0x30)
			}
		})
	}
}

func TestLayoutLabelsOverlapAndCap(t *testing.T) {
	// three labels on the same item point stack up/down without touching
	reqs := []LabelRequest{
		{ItemX: 600, ItemY: 400, TextW: 100, TextH: 16},
		{ItemX: 600, ItemY: 400, TextW: 100, TextH: 16},
		{ItemX: 600, ItemY: 400, TextW: 100, TextH: 16},
	}

	got := LayoutLabels(reqs, 0, 1512, 982)
	for i := range got {
		for j := i + 1; j < len(got); j++ {
			if got[i].overlaps(got[j]) {
				t.Errorf("labels %d and %d overlap: %+v %+v", i, j, got[i], got[j])
			}
		}
	}

	if got[1].T >= got[0].T {
		t.Errorf("second label should move up first: %+v %+v", got[0], got[1])
	}

	many := make([]LabelRequest, 50)
	for i := range many {
		many[i] = LabelRequest{ItemX: 100 + 90*(i%15), ItemY: 100 + 30*(i/15), TextW: 80, TextH: 16}
	}

	if n := len(LayoutLabels(many, 0, 1920, 1080)); n != MaxGroundLabels {
		t.Errorf("cap: %d", n)
	}
}

func TestLayoutLabelsGolden(t *testing.T) {
	got := LayoutLabels([]LabelRequest{{ItemX: 756, ItemY: 491, TextW: 92, TextH: 16}}, 0, 1512, 982)
	want := LabelBox{L: 706, T: 479, R: 806, B: 491}

	if got[0] != want {
		t.Errorf("got %+v want %+v", got[0], want)
	}
}
