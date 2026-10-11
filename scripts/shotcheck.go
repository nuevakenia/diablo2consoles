//go:build ignore

// shotcheck checks a screenshot of the game (verify scenarios, docs/DISPLAY.md).
//
//	go run scripts/shotcheck.go size   shot.png W H      the picture is exactly W x H
//	go run scripts/shotcheck.go menu   shot.png W H      menu screen: the non-black content is the centred 4:3 region
//	go run scripts/shotcheck.go menu-top shot.png W H    same, for art that ends in black (content may stop short at the bottom)
//
// It prints one line and exits 1 when the check fails. The rules are in d2common/d2display (shotcheck.go, tested).
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"strconv"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2display"
)

const (
	blackThreshold = 12   // a pixel at or below this level in every channel counts as black
	tolerance      = 12   // pixels of slack at the edges of the centred region
	minFill        = 0.80 // the content fills at least this much of the region
)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("usage: shotcheck size|menu|menu-top shot.png W H")
		os.Exit(2)
	}

	w, errW := strconv.Atoi(os.Args[3])
	h, errH := strconv.Atoi(os.Args[4])

	if errW != nil || errH != nil {
		fmt.Println("bad size")
		os.Exit(2)
	}

	f, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}

	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Println("FAIL:", err)
		os.Exit(1)
	}

	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		fmt.Printf("FAIL: %s is %dx%d, want %dx%d\n", os.Args[2], b.Dx(), b.Dy(), w, h)
		os.Exit(1)
	}

	if os.Args[1] == "size" {
		fmt.Printf("ok: %s is %dx%d\n", os.Args[2], w, h)

		return
	}

	content := d2display.NonBlackBounds(img, blackThreshold)

	check := d2display.CheckMenuCentred
	if os.Args[1] == "menu-top" {
		check = d2display.CheckMenuCentredTop
	}

	if err := check(content, w, h, tolerance, minFill); err != nil {
		fmt.Printf("FAIL: %s: %v\n", os.Args[2], err)
		os.Exit(1)
	}

	fmt.Printf("ok: %s content %v inside the centred region %v\n", os.Args[2], content, d2display.MenuRegion(w, h))
}
