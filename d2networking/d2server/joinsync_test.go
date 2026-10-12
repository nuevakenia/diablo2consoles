package d2server

import (
	"testing"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2level"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

func TestServerJoinSync(t *testing.T) {
	g, a, _ := testServer()
	g.sendJoinSync(a)

	var got *d2netpacket.JoinSyncPacket

	for _, pk := range a.got {
		if pk.PacketType == d2netpackettype.JoinSync {
			p, err := d2netpacket.UnmarshalJoinSync(pk.PacketData)
			if err != nil {
				t.Fatal(err)
			}

			got = &p
		}
	}

	if got == nil || got.Player != "a" || got.Act != 1 || got.Sync.RevealLevel != 1 || len(got.Sync.Record) != 96 {
		t.Fatalf("join sync %+v", got)
	}
}

// Trade requests follow 0x564b60: town only and no new trade within 5 s of the last.
func TestTradeRequestRules(t *testing.T) {
	g, a, b := testServer()
	now := time.Unix(1000, 0)
	g.soc.now = func() time.Time { return now }

	g.soc.mu.Lock()
	g.soc.roster.SetArea("b", 2) // Blood Moor
	err := g.checkTradeRequestLocked("a", "b")
	g.soc.mu.Unlock()

	if err == nil {
		t.Fatal("a trade with a hero outside the town must be refused")
	}

	g.soc.mu.Lock()
	g.soc.roster.SetArea("b", d2level.RogueEncampment)
	g.soc.lastTrade["a"] = now.Add(-3 * time.Second)
	err = g.checkTradeRequestLocked("a", "b")
	g.soc.mu.Unlock()

	if err == nil {
		t.Fatal("a trade 3 s after the last one must be refused")
	}

	now = now.Add(3 * time.Second)

	g.soc.mu.Lock()
	err = g.checkTradeRequestLocked("a", "b")
	g.soc.mu.Unlock()

	if err != nil {
		t.Fatalf("after the cooldown: %v", err)
	}

	_, _ = a, b
}
