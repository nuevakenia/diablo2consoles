package d2automap

// Label placement for the Alt ground labels: first box (LabelRect), the screen clamp (ClampLabel) and
// the overlap avoidance of INV_ClampGroundLabelToScreen 0x4bcf20.
//
// VERIFIED (notes w25-automap): a label that overlaps an earlier one is moved, 12 tries, up and down
// alternating. UNVERIFIED: the step of one try (this code uses the box height plus a 1 pixel gap, the
// notes do not record the exe's constant) and that each candidate is clamped again.

// MaxLabelMoves is the number of overlap tries (12 in the exe).
const MaxLabelMoves = 12

// LabelRequest is one label to place: the item's screen point and the text size.
type LabelRequest struct {
	ItemX, ItemY int
	TextW, TextH int
}

func (a LabelBox) overlaps(b LabelBox) bool {
	return a.L <= b.R && b.L <= a.R && a.T <= b.B && b.T <= a.B
}

// LayoutLabels places up to MaxGroundLabels labels in order. viewL, viewR, viewH are the clamp limits
// (left edge, right edge, view height). The returned slice has one box per placed request, in order;
// requests past the cap are dropped.
func LayoutLabels(reqs []LabelRequest, viewL, viewR, viewH int) []LabelBox {
	placed := make([]LabelBox, 0, len(reqs))

	for _, r := range reqs {
		if len(placed) >= MaxGroundLabels {
			break
		}

		box := ClampLabel(LabelRect(r.ItemX, r.ItemY, r.TextW, r.TextH), viewL, viewR, viewH)
		placed = append(placed, avoidOverlap(box, placed, r.TextH, viewL, viewR, viewH))
	}

	return placed
}

func avoidOverlap(box LabelBox, placed []LabelBox, textH, viewL, viewR, viewH int) LabelBox {
	if !hitsAny(box, placed) {
		return box
	}

	step := box.B - box.T + 1 + 1
	if step < 1 {
		step = textH
	}

	for try := 1; try <= MaxLabelMoves; try++ {
		d := ((try + 1) / 2) * step // 1,1,2,2,3,3 ... steps
		if try%2 == 1 {
			d = -d // up first
		}

		cand := box
		cand.T += d
		cand.B += d
		cand = ClampLabel(cand, viewL, viewR, viewH)

		if !hitsAny(cand, placed) {
			return cand
		}
	}

	return box
}

func hitsAny(b LabelBox, placed []LabelBox) bool {
	for _, p := range placed {
		if b.overlaps(p) {
			return true
		}
	}

	return false
}
