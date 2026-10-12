package d2ui

import (
	"image/color"
	"testing"
)

func TestColorBlindPaletteSeparatesItemKinds(t *testing.T) {
	defer SetColorBlind(false)

	tokens := []ColorToken{ColorTokenMagicItem, ColorTokenRareItem, ColorTokenSetItem, ColorTokenUniqueItem, ColorTokenCraftedItem}

	normal := getColor(ColorTokenBlue)

	SetColorBlind(true)

	if getColor(ColorTokenBlue) == normal {
		t.Fatal("the palette must replace the original blue")
	}

	seen := map[color.Color]ColorToken{}

	for _, tok := range tokens {
		c := getColor(tok)
		if c == nil {
			t.Fatalf("%s has no colour", tok)
		}

		if other, dup := seen[c]; dup {
			t.Fatalf("%s and %s share a colour", tok, other)
		}

		seen[c] = tok
	}

	if getColor(ColorTokenWhite) == nil || getColor(ColorTokenRed) == nil {
		t.Fatal("tokens outside the palette must keep their colours")
	}

	SetColorBlind(false)

	if getColor(ColorTokenBlue) != normal || ColorBlind() {
		t.Fatal("turning the option off must restore the original colours")
	}
}

func TestCompareColours(t *testing.T) {
	defer SetColorBlind(false)

	SetColorBlind(false)

	if CompareBetter() != ColorTokenBlue || CompareWorse() != ColorTokenRed || CompareEqual() != ColorTokenWhite || CompareWords() {
		t.Fatal("the normal comparison colours are blue, red and white")
	}

	SetColorBlind(true)

	if CompareWorse() != ColorTokenOrange || !CompareWords() {
		t.Fatal("the colour-blind option uses vermillion for worse and spells the words")
	}

	if getColor(CompareBetter()) == getColor(CompareWorse()) || getColor(CompareWorse()) == getColor(CompareEqual()) {
		t.Fatal("better, worse and equal must stay apart")
	}
}
