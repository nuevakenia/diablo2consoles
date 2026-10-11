package d2display

import (
	"fmt"
	"image"
	"image/color"
)

// MenuRegion is where the 800x600 menu picture lands in a window of w x h pixels: the largest 4:3 rectangle
// that fits, centred (the rest is black bars). It is what the renderer's menu layout produces.
func MenuRegion(w, h int) image.Rectangle {
	if w*BaseH >= h*BaseW { // the window is wider than 4:3: bars left and right
		rw := h * BaseW / BaseH

		return image.Rect((w-rw)/2, 0, (w-rw)/2+rw, h)
	}

	rh := w * BaseH / BaseW

	return image.Rect(0, (h-rh)/2, w, (h-rh)/2+rh)
}

// NonBlackBounds is the bounding box of the pixels whose red, green or blue is above threshold (0..255);
// the empty rectangle when the picture is black.
func NonBlackBounds(img image.Image, threshold uint8) image.Rectangle {
	b := img.Bounds()
	out := image.Rectangle{}
	limit := uint32(threshold) * 0x101

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBA64Model.Convert(img.At(x, y)).(color.RGBA64)
			if uint32(c.R) > limit || uint32(c.G) > limit || uint32(c.B) > limit {
				out = out.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}

	return out
}

// CheckMenuCentred checks a screenshot of a menu screen of a w x h window: the content (non-black pixels) must
// stay inside the centred 4:3 region (tol pixels of slack), must be centred in the window (equal margins left /
// right and top / bottom, within tol) and must fill at least minFill (0..1) of the region's width and height.
// It returns nil when the picture is as expected.
func CheckMenuCentred(content image.Rectangle, w, h, tol int, minFill float64) error {
	if content.Empty() {
		return fmt.Errorf("the screenshot is black")
	}

	region := MenuRegion(w, h)

	if !content.In(region.Inset(-tol)) {
		return fmt.Errorf("content %v leaves the centred 4:3 region %v", content, region)
	}

	left, right := content.Min.X, w-content.Max.X
	top, bottom := content.Min.Y, h-content.Max.Y

	if abs(left-right) > tol || abs(top-bottom) > tol {
		return fmt.Errorf("content %v is not centred in %dx%d (margins l=%d r=%d t=%d b=%d)", content, w, h, left, right, top, bottom)
	}

	if float64(content.Dx()) < minFill*float64(region.Dx()) || float64(content.Dy()) < minFill*float64(region.Dy()) {
		return fmt.Errorf("content %v fills less than %.0f%% of the region %v", content, minFill*100, region)
	}

	return nil
}

// CheckMenuCentredTop is CheckMenuCentred for a menu picture whose bottom edge is dark art (the hero creation
// screen ends in a black campfire ground), so the content may stop short of the region's bottom: the content must
// stay inside the region, start at its top edge, be centred horizontally and fill its width.
func CheckMenuCentredTop(content image.Rectangle, w, h, tol int, minFill float64) error {
	if content.Empty() {
		return fmt.Errorf("the screenshot is black")
	}

	region := MenuRegion(w, h)

	if !content.In(region.Inset(-tol)) {
		return fmt.Errorf("content %v leaves the centred 4:3 region %v", content, region)
	}

	if abs(content.Min.X-(w-content.Max.X)) > tol || content.Min.Y-region.Min.Y > tol {
		return fmt.Errorf("content %v is not centred / at the top of the region %v in %dx%d", content, region, w, h)
	}

	if float64(content.Dx()) < minFill*float64(region.Dx()) || float64(content.Dy()) < minFill*float64(region.Dy()) {
		return fmt.Errorf("content %v fills less than %.0f%% of the region %v", content, minFill*100, region)
	}

	return nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
