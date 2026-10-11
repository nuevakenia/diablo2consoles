package d2server

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2client/d2clientconnectiontype"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2gs"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

// Idle tracking of the clients, after Game.exe's CCMD_ProcessClientGameMessage
// (0x53d120) and GAME_KickTimedOutClients (0x52adc0), both VERIFIED by
// decompile: every in-game message stamps the sender's last-message time
// (ClientData +0x3d8), and a periodic sweep kicks clients that fell silent.
//
// The stamp is wired into OnPacketReceived and the sweep into GameServer.Start
// (idleLoop). The exe runs the sweep only when two global switches are set
// (their meaning is unknown), so here it is opt-in: OD2_IDLE_KICK=1 turns it on
// with the exe's limits, and the test-only OD2_IDLE_KICK_MS=<ms> turns it on with
// a shorter silent limit. The slow/backlog/marked inputs of the test are not
// modelled (always false/0). A kick saves the hero and closes the connection;
// the host's own (local) client is never kicked.

// Idle limits of the exe.
const (
	// IdleKickAfter kicks a silent client outright.
	IdleKickAfter = 45 * time.Second
	// IdleSlowKickAfter is the shorter limit of a client flagged as slow
	// (flag test CHARINFO_TestFlagBits, meaning unverified) with a backlog.
	IdleSlowKickAfter = 10 * time.Second
	// IdleBacklogLimit is the backlog (ClientData +0x54, meaning unverified)
	// above which the short limit applies.
	IdleBacklogLimit = 10

	// MaxSaveChunk is the exclusive upper bound of a save-upload chunk (lobby
	// opcode 0x6c: a chunk length of 0x2000 or more is a fatal error).
	MaxSaveChunk = d2gs.MaxSaveChunk
)

// idleTracker holds the last-message time per client; the clock is injectable.
type idleTracker struct {
	mu   sync.Mutex
	now  func() time.Time
	last map[string]time.Time
}

func newIdleTracker(now func() time.Time) *idleTracker {
	if now == nil {
		now = time.Now
	}

	return &idleTracker{now: now, last: map[string]time.Time{}}
}

// Stamp records a message of the client.
func (t *idleTracker) Stamp(id string) {
	t.mu.Lock()
	t.last[id] = t.now()
	t.mu.Unlock()
}

// Forget drops a client that left.
func (t *idleTracker) Forget(id string) {
	t.mu.Lock()
	delete(t.last, id)
	t.mu.Unlock()
}

// Last returns the last message time of a client.
func (t *idleTracker) Last(id string) (time.Time, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	v, ok := t.last[id]

	return v, ok
}

// IdleExpired is the exe's kick test for one client: the last message must lie
// in the past, and then the client goes when it was silent for more than
// IdleKickAfter, or for more than IdleSlowKickAfter while flagged slow with a
// backlog above IdleBacklogLimit, or when it is marked (ClientData +0x58,
// meaning unverified).
func IdleExpired(now, last time.Time, slow bool, backlog int, marked bool) bool {
	return idleExpiredAfter(now, last, IdleKickAfter, slow, backlog, marked)
}

// idleExpiredAfter is IdleExpired with the silent limit as a parameter.
func idleExpiredAfter(now, last time.Time, limit time.Duration, slow bool, backlog int, marked bool) bool {
	if !last.Before(now) {
		return false
	}

	idle := now.Sub(last)

	return (slow && idle > IdleSlowKickAfter && backlog > IdleBacklogLimit) || idle > limit || marked
}

// idleLimit is the silent limit of the sweep and whether the sweep runs.
func idleLimit() (time.Duration, bool) {
	if s := os.Getenv("OD2_IDLE_KICK_MS"); s != "" {
		if ms, err := strconv.Atoi(s); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond, true
		}
	}

	return IdleKickAfter, os.Getenv("OD2_IDLE_KICK") != ""
}

// idleVictims lists the clients (not the local host) silent for longer than limit.
func (g *GameServer) idleVictims(now time.Time, limit time.Duration) []ClientConnection {
	g.RLock()
	defer g.RUnlock()

	var out []ClientConnection

	for id, c := range g.connections {
		if c.GetConnectionType() == d2clientconnectiontype.Local {
			continue
		}

		if last, ok := g.soc.idle.Last(id); ok && idleExpiredAfter(now, last, limit, false, 0, false) {
			out = append(out, c)
		}
	}

	return out
}

// closer is implemented by connections that can be closed by the server.
type closer interface{ Close() error }

// kickIdle removes a silent client: its hero is saved, its connection closed.
func (g *GameServer) kickIdle(c ClientConnection) {
	g.Infof("IDLE KICK name=%q id=%s (no message for the idle limit)", playerName(c), c.GetUniqueID())

	if st := c.GetPlayerState(); st != nil {
		g.saveD2S(st)
	}

	g.soc.idle.Forget(c.GetUniqueID())

	if cl, ok := c.(closer); ok {
		_ = cl.Close()
	}

	if _, isGS := c.(*gsClientConnection); isGS {
		return // its read loop sees the close and reports the departure
	}

	if bye, err := d2netpacket.CreatePlayerDisconnectRequestPacket(c.GetUniqueID()); err == nil {
		select {
		case <-g.ctx.Done():
		case g.packetManagerChan <- ReceivedPacket{Client: c, Packet: bye}:
		}
	}
}

// idleLoop runs the sweep once a second while the server lives.
func (g *GameServer) idleLoop() {
	limit, on := idleLimit()
	if !on {
		return
	}

	t := time.NewTicker(time.Second / 2)
	defer t.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case now := <-t.C:
			for _, c := range g.idleVictims(now, limit) {
				g.kickIdle(c)
			}
		}
	}
}

// SaveChunkOK reports whether a save-upload chunk length is acceptable (the exe
// treats 0x2000 and above as fatal). The Go server sends its saves whole or in
// 4096-byte chunks (chunkSize), well below the cap.
func SaveChunkOK(n int) bool { return n >= 0 && n < d2gs.MaxSaveChunk }
