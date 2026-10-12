package d2skill

import "testing"

// A hold at the port's cadence: the cast animation takes 7 frames from the start to the Do and the next run starts one
// frame after the action ended (InfernoLen 1), so a run starts every 8 frames, more than the 6 frame window of the
// start. Only the first run of the hold is a wind-up; every later one is a flame, and a pause longer than the window
// starts over. (Before w42 the runs alternated wind-up / flame.)
func TestChannelHoldAtAnimationCadence(t *testing.T) {
	reg := loadRealRegistry(t)
	id := reg.ByName("Inferno").ID
	frame := 0
	p := &Pipeline{Skills: reg, Frame: func() int { return frame }}
	u := newUnit(reg, map[string]int{"Inferno": 5})
	tg := Target{X: 10, Y: 10}

	const startToDo, period = 7, 8

	for run := 0; run < 12; run++ {
		st := p.Start(u, id, tg)
		if !st.OK {
			t.Fatalf("run %d: %+v", run, st)
		}

		if want := run == 0; st.ChannelFirst != want {
			t.Fatalf("run %d at frame %d: ChannelFirst=%v, want %v", run, frame, st.ChannelFirst, want)
		}

		frame += startToDo

		if flame := p.channelDo(u); flame != (run > 0) {
			t.Fatalf("run %d: flame=%v, want %v", run, flame, run > 0)
		}

		if got, want := p.ChannelActionEnd(u), frame+1; got != want {
			t.Fatalf("run %d: action end %d, want Do + InfernoLen = %d", run, got, want)
		}

		frame += period - startToDo
	}

	// the button is released: after the window the next press is a wind-up again
	frame += ChannelHoldFrames + 2

	if st := p.Start(u, id, tg); !st.ChannelFirst {
		t.Fatal("a new press after the window must start with a wind-up")
	}
}
