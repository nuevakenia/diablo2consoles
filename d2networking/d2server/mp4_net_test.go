package d2server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2util"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// netServer is a GameServer able to run the JSON connection path in a test (no game data).
func netServer(t *testing.T) *GameServer {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	g := &GameServer{connections: map[string]ClientConnection{}, soc: newSocial(), Logger: d2util.NewLogger(),
		maxConnections: 8, ctx: ctx, cancel: cancel, packetManagerChan: make(chan ReceivedPacket)}

	go g.packetManager()
	t.Cleanup(cancel)

	return g
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()

	for i := 0; i < 300; i++ {
		if cond() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timeout waiting for %s", what)
}

func joinJSON(t *testing.T, g *GameServer, id, name string) (net.Conn, *json.Encoder) {
	t.Helper()

	srv, cli := net.Pipe()

	go io.Copy(io.Discard, cli) // the client side drains what the server sends
	go g.handleConnection(srv)

	req, err := d2netpacket.CreatePlayerConnectionRequestPacket(id, hero(name, 10, 0))
	if err != nil {
		t.Fatal(err)
	}

	enc := json.NewEncoder(cli)
	if err := enc.Encode(req); err != nil {
		t.Fatal(err)
	}

	return cli, enc
}

func nConns(g *GameServer) int {
	g.RLock()
	defer g.RUnlock()

	return len(g.connections)
}

// A client that drops without a goodbye must free its id and name so it can reconnect; a
// client that says goodbye and then drops is removed once.
func TestDropAndReconnectJSON(t *testing.T) {
	g := netServer(t)

	c1, _ := joinJSON(t, g, "p1", "Ann")
	waitFor(t, "join", func() bool { return nConns(g) == 1 })

	_ = c1.Close() // vanishes
	waitFor(t, "drop removed", func() bool { return nConns(g) == 0 })

	c2, enc := joinJSON(t, g, "p1", "Ann") // reconnect, same id and name
	waitFor(t, "rejoin", func() bool { return nConns(g) == 1 })

	bye, _ := d2netpacket.CreatePlayerDisconnectRequestPacket("p1")
	_ = enc.Encode(bye)
	waitFor(t, "goodbye", func() bool { return nConns(g) == 0 })
	_ = c2.Close()
	time.Sleep(50 * time.Millisecond)

	if nConns(g) != 0 {
		t.Fatal("double removal left a stale entry")
	}
}

// Something other than a join as the first packet closes the connection without registering.
func TestFirstPacketNotAJoin(t *testing.T) {
	g := netServer(t)
	srv, cli := net.Pipe()

	done := make(chan struct{})

	go func() { g.handleConnection(srv); close(done) }()

	chat, _ := d2netpacket.CreateChatPacket("x", "x", "hi")

	go func() { _ = json.NewEncoder(cli).Encode(chat) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("connection not closed")
	}

	if nConns(g) != 0 {
		t.Fatal("registered")
	}
}

// Three heroes join, one leaves in the middle, a name clash is refused, and a late packet of the
// leaver is dropped.
func TestJoinLeaveWithThreeHeroes(t *testing.T) {
	g := netServer(t)

	var conns []net.Conn

	for i, n := range []string{"Ann", "Bob", "Cy"} {
		c, _ := joinJSON(t, g, fmt.Sprintf("p%d", i), n)
		conns = append(conns, c)
	}

	waitFor(t, "3 heroes", func() bool { return nConns(g) == 3 })

	dup, _ := joinJSON(t, g, "p9", "bob") // same name, other case
	time.Sleep(50 * time.Millisecond)

	if nConns(g) != 3 {
		t.Fatalf("name clash accepted: %d", nConns(g))
	}

	_ = dup.Close()
	_ = conns[1].Close()
	waitFor(t, "Bob gone", func() bool { return nConns(g) == 2 })

	ghost := &fakeConn{id: "p1", state: hero("Bob", 1, 0)}
	pk := d2netpacket.NetPacket{PacketType: d2netpackettype.MovePlayer, PacketData: []byte("{}")}

	if err := g.OnPacketReceived(ghost, pk); err == nil {
		t.Fatal("late packet of a left hero was processed")
	}
}
