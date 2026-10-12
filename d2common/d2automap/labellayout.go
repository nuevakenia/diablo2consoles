package d2automap

// Label placement for the Alt ground labels: first box (LabelRect), the screen clamp (ClampLabel) and
// the overlap avoidance of INV_ClampGroundLabelToScreen 0x4bcf20.
//
// VERIFIED (w37, INV_ClampGroundLabelToScreen 0x4bcf20 and its caller): up to 12 tries; each try clamps the
// box, then looks for the first earlier label it touches (inclusive edges). Tries 0..6 move the box
// vertically: to sit 3 pixels above that label (bottom = its top - 3), or 5 pixels below it when the box top
// is under the screen height - 0x30; tries 7..11 move it horizontally to start 1 pixel right of that label
// (the "left of it" branch needs the right bound <= the box left, which does not happen). After the twelfth
// try the label is dropped, as is one whose box ends beyond the bounds.

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

// PlacedLabel is a label that found room: Index is its position in the request slice.
type PlacedLabel struct {
	Index int
	Box   LabelBox
}

// LayoutLabels places up to MaxGroundLabels labels in order. viewL, viewR, viewH are the clamp limits
// (left edge, right edge, view height). The returned slice has one box per placed request, in order;
// requests past the cap or without room are dropped (use PlaceLabels to know which).
func LayoutLabels(reqs []LabelRequest, viewL, viewR, viewH int) []LabelBox {
	placed := PlaceLabels(reqs, viewL, viewR, viewH)
	out := make([]LabelBox, len(placed))

	for i, p := range placed {
		out[i] = p.Box
	}

	return out
}

// PlaceLabels is LayoutLabels with the request index of every placed label.
func PlaceLabels(reqs []LabelRequest, viewL, viewR, viewH int) []PlacedLabel {
	placed := make([]PlacedLabel, 0, len(reqs))
	boxes := make([]LabelBox, 0, len(reqs))

	for i, r := range reqs {
		if len(boxes) >= MaxGroundLabels {
			break
		}

		box, ok := avoidOverlap(LabelRect(r.ItemX, r.ItemY, r.TextW, r.TextH), boxes, viewL, viewR, viewH)
		if !ok {
			continue
		}

		boxes = append(boxes, box)
		placed = append(placed, PlacedLabel{Index: i, Box: box})
	}

	return placed
}

// firstHit is the first earlier label the box touches.
func firstHit(b LabelBox, placed []LabelBox) (LabelBox, bool) {
	for _, p := range placed {
		if b.overlaps(p) {
			return p, true
		}
	}

	return LabelBox{}, false
}

func avoidOverlap(box LabelBox, placed []LabelBox, viewL, viewR, viewH int) (LabelBox, bool) {
	for try := 0; try < MaxLabelMoves; try++ {
		box = ClampLabel(box, viewL, viewR, viewH)

		hit, ok := firstHit(box, placed)
		if !ok {
			// the caller also drops a label that still ends outside the bounds
			return box, box.R <= viewR && box.L >= viewL
		}

		if try < 7 { //nolint:gomnd // exe constant
			var d int

			if (box.B-box.T)/2 != 0 && box.T <= viewH-0x30 {
				d = hit.T - box.B - 3
			} else {
				d = hit.B - box.T + 5
			}

			box.T += d
			box.B += d

			continue
		}

		d := hit.R - box.L + 1
		box.L += d
		box.R += d
	}

	return box, false
}
