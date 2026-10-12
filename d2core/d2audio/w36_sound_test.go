package d2audio

import "testing"

func TestFormMonSoundsID(t *testing.T) {
	for token, want := range map[string]string{"40": "werewolf", "TG": "werebear", "AM": "", "": ""} {
		if got := FormMonSoundsID(token); got != want {
			t.Errorf("FormMonSoundsID(%q) = %q, want %q", token, got, want)
		}
	}
}
