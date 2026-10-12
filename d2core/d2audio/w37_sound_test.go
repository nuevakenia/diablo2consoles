package d2audio

import "testing"

func TestObjectModeSound(t *testing.T) {
	cases := []struct {
		row, mode int
		want      string
	}{
		{13, 0, "object_door_metal_close"}, {13, 2, "object_door_metal_open"}, {13, 1, ""},
		{294, 2, "object_door_wood_open"}, {16, 0, "object_door_wood_close"},
		{47, 2, "object_door_gate_open"}, {25, 0, "object_door_gate_close"},
		{27, 1, "object_door_wood_open"}, {27, 2, ""},
		{91, 0, "object_door_stone"}, {91, 1, "object_door_stone"}, {91, 2, ""},
		{229, 1, "object_door_slime"}, {129, 3, "object_door_secret"},
		{59, 1, "object_townportal"}, {60, 1, "object_townportal"}, {119, 1, "object_waypoint_open"},
		{540, 1, "object_waypoint_open"}, {119, 0, ""}, {1, 1, ""},
	}

	for _, c := range cases {
		if got := ObjectModeSound(c.row, c.mode); got != c.want {
			t.Errorf("ObjectModeSound(%d,%d) = %q, want %q", c.row, c.mode, got, c.want)
		}
	}
}

func TestAreaVoiceFor(t *testing.T) {
	cases := map[int]int{17: 0x26, 35: 0x28, 8: 0x29, 30: 0x2a, 27: 0x2b, 24: 0x2c, 38: 0x2e, 5: 0x2f, 74: 0x37,
		58: 0x38, 110: 0x4c, 120: 0x4e, 121: 0x4d, 132: 0x4f}

	for level, voice := range cases {
		r := AreaVoiceFor(level)
		if r == nil || r.Voice != voice {
			t.Errorf("level %d: %+v, want voice %#x", level, r, voice)
		}
	}

	if AreaVoiceFor(1) != nil || AreaVoiceFor(40) != nil {
		t.Error("the Rogue Encampment and Act 2 town have no entry voice")
	}
}

func TestHitGruntPlays(t *testing.T) {
	if !HitGruntPlays(false, false) || HitGruntPlays(true, false) || HitGruntPlays(false, true) {
		t.Error("the grunt needs silence and a living hero")
	}
}

func TestSampleTicks(t *testing.T) {
	if got := SamplesToTicks(3457024); got != 3919 {
		t.Errorf("SamplesToTicks = %d", got)
	}

	if TicksToSamples(25) != MusicSampleRate {
		t.Error("25 ticks are a second")
	}
}

func TestDoorOperateSound(t *testing.T) {
	cases := []struct {
		row     int
		opening bool
		want    string
	}{
		{13, true, "object_door_metal_open"}, {13, false, "object_door_metal_close"},
		{15, true, "object_door_wood_open"}, {27, true, "object_door_wood_open"}, {27, false, "object_door_wood_close"},
		{47, true, "object_door_gate_open"}, {91, true, "object_door_stone"}, {91, false, "object_door_stone"},
		{229, true, "object_door_slime"}, {229, false, ""}, {1, true, ""},
	}

	for _, c := range cases {
		if got := DoorOperateSound(c.row, c.opening); got != c.want {
			t.Errorf("DoorOperateSound(%d,%v) = %q, want %q", c.row, c.opening, got, c.want)
		}
	}
}

func TestQuestPlansAreQuestLines(t *testing.T) {
	for id, p := range QuestPlans {
		if id < VoiceQuestFirst || id > VoiceQuestLast {
			t.Errorf("plan id %#x is not a quest line", id)
		}

		if p.Silence <= 0 {
			t.Errorf("plan %#x has no silence", id)
		}
	}
}
