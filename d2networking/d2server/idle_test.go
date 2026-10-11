package d2server

import (
	"context"

	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
	"testing"
	"time"
)

func TestIdleTrackerStamp(t *testing.T) {
	clock := time.Unix(100, 0)
	tr := newIdleTracker(func() time.Time { return clock })

	if _, ok := tr.Last("a"); ok {
		t.Fatal("no stamp yet")
	}

	tr.Stamp("a")

	clock = clock.Add(5 * time.Second)
	tr.Stamp("b")

	if v, _ := tr.Last("a"); !v.Equal(time.Unix(100, 0)) {
		t.Fatalf("a: %v", v)
	}

	tr.Stamp("a")

	if v, _ := tr.Last("a"); !v.Equal(time.Unix(105, 0)) {
		t.Fatalf("restamp: %v", v)
	}

	tr.Forget("a")

	if _, ok := tr.Last("a"); ok {
		t.Fatal("forgotten")
	}
}

// TestIdleSweepKicksSilentClient is the wiring: the sweep's victims come from
// the tracker the packet path stamps, the kick saves/forgets the client and
// reports its departure like a client that said goodbye.
func TestIdleSweepKicksSilentClient(t *testing.T) {
	g, a, b := testServer()
	g.ctx, g.cancel = context.WithCancel(context.Background())
	g.packetManagerChan = make(chan ReceivedPacket, 4)

	defer g.cancel()

	clock := time.Unix(1000, 0)
	g.soc.idle = newIdleTracker(func() time.Time { return clock })

	g.soc.idle.Stamp("a")

	clock = clock.Add(8 * time.Second)
	g.soc.idle.Stamp("b") // b spoke later

	now := clock.Add(3 * time.Second) // a silent for 11 s, b for 3 s

	if v := g.idleVictims(now, 10*time.Second); len(v) != 1 || v[0].GetUniqueID() != "a" {
		t.Fatalf("victims %v", v)
	}

	if v := g.idleVictims(now, IdleKickAfter); len(v) != 0 {
		t.Fatalf("45 s limit kicked %v", v)
	}

	g.kickIdle(a)

	select {
	case p := <-g.packetManagerChan:
		if p.Client != ClientConnection(a) || p.Packet.PacketType != d2netpackettype.PlayerDisconnectionNotification {
			t.Fatalf("kick packet %+v", p)
		}
	default:
		t.Fatal("no departure reported")
	}

	if _, ok := g.soc.idle.Last("a"); ok {
		t.Fatal("kicked client still tracked")
	}

	_ = b
}

func TestIdleLimitEnv(t *testing.T) {
	if _, on := idleLimit(); on {
		t.Fatal("sweep must be off by default")
	}

	t.Setenv("OD2_IDLE_KICK_MS", "1500")

	if d, on := idleLimit(); !on || d != 1500*time.Millisecond {
		t.Fatalf("%v %v", d, on)
	}
}

func TestIdleExpired(t *testing.T) {
	now := time.Unix(1000, 0)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	tests := []struct {
		name    string
		last    time.Time
		slow    bool
		backlog int
		marked  bool
		want    bool
	}{
		{"fresh", ago(time.Second), false, 0, false, false},
		{"45 s exactly", ago(45 * time.Second), false, 0, false, false},
		{"45 s and a bit", ago(45*time.Second + time.Millisecond), false, 0, false, true},
		{"slow, short, backlog", ago(11 * time.Second), true, 11, false, true},
		{"slow, short, backlog 10", ago(11 * time.Second), true, 10, false, false},
		{"not slow", ago(30 * time.Second), false, 99, false, false},
		{"marked", ago(time.Second), false, 0, true, true},
		{"stamp in the future", now.Add(time.Hour), true, 99, true, false},
		{"stamp now", now, true, 99, true, false},
	}

	for _, tc := range tests {
		if got := IdleExpired(now, tc.last, tc.slow, tc.backlog, tc.marked); got != tc.want {
			t.Errorf("%s: %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestSaveChunkOK(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{{0, true}, {4096, true}, {0x1fff, true}, {0x2000, false}, {0x10000, false}, {-1, false}}
	for _, tc := range tests {
		if SaveChunkOK(tc.n) != tc.want {
			t.Errorf("%#x", tc.n)
		}
	}

	if !SaveChunkOK(chunkSize) {
		t.Fatal("the server's own chunk size must fit")
	}
}
