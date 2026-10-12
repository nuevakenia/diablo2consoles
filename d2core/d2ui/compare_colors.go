package d2ui

// The item comparison lines of a tooltip (an item against the one it would replace) use three colours: blue for
// better, red for worse (the owner's request) and white for equal. With the colour-blind option the worse colour is
// the vermillion of the Okabe-Ito set instead of the light red, which keeps apart from the sky blue for every common
// kind of colour blindness; CompareWords says whether the lines also carry the words "better" and "worse", so the
// meaning never rests on the colour alone.

// CompareBetter is the token of a figure that is higher (better) than the worn item's.
func CompareBetter() ColorToken { return ColorTokenBlue }

// CompareWorse is the token of a figure that is worse than the worn item's.
func CompareWorse() ColorToken {
	if ColorBlind() {
		return ColorTokenOrange
	}

	return ColorTokenRed
}

// CompareEqual is the token of a figure that is the same.
func CompareEqual() ColorToken { return ColorTokenWhite }

// CompareWords reports whether the comparison lines spell out better and worse (the colour-blind option).
func CompareWords() bool { return ColorBlind() }
