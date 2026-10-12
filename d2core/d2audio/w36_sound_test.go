package d2audio

import "testing"

func TestDoorSound(t *testing.T) {
	cases := []struct {
		row     int
		opening bool
		want    string
	}{
		{13, true, "object_door_wood_open"},
		{13, false, "object_door_wood_close"},
		{47, true, "object_door_gate_open"},
		{47, false, "object_door_gate_close"},
		{91, true, "object_door_stone"},
		{91, false, "object_door_stone"},
		{229, true, "object_door_secret"},
		{290, false, "object_door_metal_close"},
	}

	for _, c := range cases {
		if got := DoorSound(c.row, c.opening); got != c.want {
			t.Errorf("DoorSound(%d,%v) = %s, want %s", c.row, c.opening, got, c.want)
		}
	}
}

func TestMusicResume(t *testing.T) {
	var m MusicResume

	if m.Position(7) != 0 {
		t.Fatal("unknown song must start at 0")
	}

	m.Store(7, 300, 0) // last cue point at 250
	if got := m.Position(7); got != 250 {
		t.Errorf("position %d, want 250", got)
	}

	m.Store(7, 1300, 1000) // wraps the track length
	if got := m.Position(7); got != 250 {
		t.Errorf("wrapped position %d, want 250", got)
	}

	m.Store(0, 500, 0)
	m.Store(8, -1, 0)

	if m.Position(0) != 0 || m.Position(8) != 0 {
		t.Error("song 0 and a negative time must not store")
	}
}

func TestFormMonSoundsID(t *testing.T) {
	for token, want := range map[string]string{"40": "werewolf", "TG": "werebear", "AM": "", "": ""} {
		if got := FormMonSoundsID(token); got != want {
			t.Errorf("FormMonSoundsID(%q) = %q, want %q", token, got, want)
		}
	}
}

func TestUnitSoundsNil(t *testing.T) {
	var u UnitSounds

	u.Note(1, nil)

	if u.Playing(1) {
		t.Error("nothing noted, nothing playing")
	}

	var s *Sound
	if s.Alive() {
		t.Error("nil sound is not alive")
	}

	u.Forget(1)
}
