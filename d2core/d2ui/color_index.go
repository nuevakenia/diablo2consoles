package d2ui

// ColorTokenForIndex returns the token of an engine text colour index (0 white .. 10), the index the
// exe passes to the text routines, or ColorTokenWhite for an unknown index.
func ColorTokenForIndex(i int) ColorToken {
	if i < 0 || i > int(':'-'0') {
		return ColorTokenWhite
	}

	if tok, ok := ColorCodeTokens[byte('0'+i)]; ok {
		return tok
	}

	return ColorTokenWhite
}
