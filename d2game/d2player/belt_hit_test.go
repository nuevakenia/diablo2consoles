package d2player

import (
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2inventory"
)

func testBelt(boxes int, expanded bool) *BeltPanel {
	return &BeltPanel{
		boxes: func() int { return boxes }, expanded: expanded, hoverCell: -1,
		left0: beltBox1Left, top0: beltBox1Top, strideX: beltStrideX, strideY: beltStrideY,
	}
}

// TestBeltCellHits proves every box of every belt size answers a click at its centre, rows above the front row
// answer only when expanded, and cells past the belt's box count never answer (w36: drops on rows 2-4).
func TestBeltCellHits(t *testing.T) {
	for _, boxes := range []int{4, 8, 12, 16} {
		for _, expanded := range []bool{false, true} {
			b := testBelt(boxes, expanded)

			for cell := 0; cell < d2inventory.BeltCells; cell++ {
				x, y, w, h := b.cellRect(cell)
				row, _ := d2inventory.BeltCell(cell)
				want := cell
				if cell >= boxes || (row > 0 && !expanded) {
					want = -1
				}

				// the four corners and the centre of the box are all inside the cell
				for _, p := range [][2]int{{x + w/2, y + h/2}, {x, y}, {x + w - 1, y}, {x, y + h - 1}, {x + w - 1, y + h - 1}} {
					if got := b.cellAt(p[0], p[1]); got != want {
						t.Errorf("boxes=%d expanded=%v cell %d point %v: got %d want %d", boxes, expanded, cell, p, got, want)
					}
				}
			}
		}
	}
}

// TestBeltCellGeometry pins the 800x600 positions of belts.txt "girdle2": front row at y 562, up 32 per row,
// 31 apart, boxes of 29 (the same numbers the layout golden holds).
func TestBeltCellGeometry(t *testing.T) {
	b := testBelt(16, true)

	for cell, want := range map[int][2]int{0: {423, 562}, 3: {516, 562}, 4: {423, 530}, 9: {454, 498}, 15: {516, 466}} {
		if x, y, w, h := b.cellRect(cell); x != want[0] || y != want[1] || w != 29 || h != 29 {
			t.Errorf("cell %d rect = %d,%d %dx%d, want %v 29x29", cell, x, y, w, h, want)
		}
	}
}

// TestBeltRowsOfBelts: the visible rows follow the belt size and the expand key.
func TestBeltRowsOfBelts(t *testing.T) {
	for boxes, rows := range map[int]int{4: 1, 8: 2, 12: 3, 16: 4} {
		b := testBelt(boxes, false)
		if b.visibleRows() != 1 || b.Rows() != rows {
			t.Errorf("boxes %d collapsed: visible %d rows %d", boxes, b.visibleRows(), b.Rows())
		}

		b.Toggle()

		if b.visibleRows() != rows {
			t.Errorf("boxes %d expanded: visible %d want %d", boxes, b.visibleRows(), rows)
		}
	}
}
