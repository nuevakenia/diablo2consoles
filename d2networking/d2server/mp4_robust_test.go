package d2server

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// allTypes is every packet type a client may send to the server, plus the ones it must not.
func allTypes() []d2netpackettype.NetPacketType {
	var out []d2netpackettype.NetPacketType
	for t := d2netpackettype.UpdateServerInfo; t <= d2netpackettype.JoinSync; t++ {
		out = append(out, t)
	}

	return append(out, 400, d2netpackettype.UnknownPacketType)
}

func noPanic(t *testing.T, what string, f func()) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic in %s: %v", what, r)
		}
	}()

	f()
}

// Garbage bodies for every packet type from a registered client must be errors or ignored, never panics.
func TestMalformedPacketsDoNotPanic(t *testing.T) {
	bodies := [][]byte{nil, {}, []byte("{"), []byte("null"), []byte("[]"), []byte(`"x"`), []byte("0"),
		[]byte(`{"amount":-5,"xp":-100,"x":1e999}`), []byte(`{"offer":null,"pair":null}`), []byte{0xff, 0xfe, 0}}

	for _, typ := range allTypes() {
		if typ == d2netpackettype.SavePlayer || typ == d2netpackettype.PlayerDisconnectionNotification {
			continue // need the hero factory / end the session: covered separately
		}

		for i, b := range bodies {
			g, a, _ := testServer()
			pk := d2netpacket.NetPacket{PacketType: typ, PacketData: b}

			noPanic(t, fmt.Sprintf("%s body %d", typ, i), func() { _ = g.OnPacketReceived(a, pk) })
		}
	}
}

// A packet from a connection the server does not know (a late packet after a leave, or a hand made one) is dropped.
func TestPacketsFromUnregisteredClientDoNotPanic(t *testing.T) {
	for _, typ := range allTypes() {
		g, _, _ := testServer()
		ghost := &fakeConn{id: "ghost", state: hero("Ghost", 1, 0)}
		pk := d2netpacket.NetPacket{PacketType: typ, PacketData: []byte("{}")}

		noPanic(t, "ghost "+typ.String(), func() { _ = g.OnPacketReceived(ghost, pk) })
		noPanic(t, "nil client "+typ.String(), func() { _ = g.OnPacketReceived(nil, pk) })
	}
}

// Random type and body from a registered client: the server stays up.
func TestRandomPacketsDoNotPanic(t *testing.T) {
	rng := rand.New(rand.NewSource(41))
	g, a, _ := testServer()
	types := allTypes()

	for i := 0; i < 3000; i++ {
		b := make([]byte, rng.Intn(40))
		rng.Read(b)

		if rng.Intn(2) == 0 { // often valid JSON objects with random fields
			b = []byte(fmt.Sprintf(`{"op":%q,"target":%q,"amount":%d,"xp":%d,"accept":%v,"level":%d,"x":%d}`,
				[]string{"invite", "accept", "request", "respond", "offer", "cancel", "hostile", ""}[rng.Intn(8)],
				[]string{"a", "b", "Bob", "nobody", ""}[rng.Intn(5)], rng.Intn(1<<20)-1<<19, rng.Intn(1<<20)-1<<19,
				rng.Intn(2) == 0, rng.Intn(200), rng.Intn(100)))
		}

		typ := types[rng.Intn(len(types))]
		if typ == d2netpackettype.PlayerDisconnectionNotification || typ == d2netpackettype.SavePlayer {
			continue
		}

		noPanic(t, typ.String(), func() { _ = g.OnPacketReceived(a, d2netpacket.NetPacket{PacketType: typ, PacketData: b}) })
	}
}
