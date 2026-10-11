package d2server

import (
	"sort"
	"sync"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2unitmsg"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

// The deferred per-unit hit list of Game.exe (UNIT\SUnitDmg.cpp, purge at
// 0x57a900, N16): a hit that connects on a unit is not delivered at once, it is
// appended to the defender's list (in order), the room flush delivers the lists
// later, and when the attacker is freed every entry that came from it is
// purged (matched on unit type and id, the order of the rest kept).
//
// Here the defenders are the players, the entries are the PvP hits relayed by
// onPvPHit, the flush runs every hitFlushEvery (a frame of the 25 fps server)
// and a player who leaves the game takes the hits he has not yet delivered
// with him. UNVERIFIED: the exact flush point of the exe.

// hitFlushEvery is one server frame.
const hitFlushEvery = 40 * time.Millisecond

// playerUnitType is the unit type of a player.
const playerUnitType = 0

type queuedHit struct {
	to  string
	pkt d2netpacket.NetPacket
}

// hitQueue holds the pending hits per defender.
type hitQueue struct {
	mu    sync.Mutex
	lists map[string]*d2unitmsg.HitList // by defender id
	pkts  map[int]queuedHit             // by Hit.Step
	seq   int
	ids   map[string]uint32 // numeric unit id of a player id
}

func newHitQueue() *hitQueue {
	return &hitQueue{lists: map[string]*d2unitmsg.HitList{}, pkts: map[int]queuedHit{}, ids: map[string]uint32{}}
}

func (q *hitQueue) unitID(player string) uint32 {
	id, ok := q.ids[player]
	if !ok {
		id = uint32(len(q.ids) + 1)
		q.ids[player] = id
	}

	return id
}

// Add queues a packet for the defender, coming from the attacker.
func (q *hitQueue) Add(attacker, defender string, pkt d2netpacket.NetPacket) {
	q.mu.Lock()
	defer q.mu.Unlock()

	l := q.lists[defender]
	if l == nil {
		l = &d2unitmsg.HitList{}
		q.lists[defender] = l
	}

	q.seq++
	q.pkts[q.seq] = queuedHit{to: defender, pkt: pkt}
	l.Add(d2unitmsg.Hit{AttackerType: playerUnitType, AttackerID: q.unitID(attacker), Step: q.seq})
}

// Pending is the number of hits waiting for the defender.
func (q *hitQueue) Pending(defender string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	if l := q.lists[defender]; l != nil {
		return l.Len()
	}

	return 0
}

// PurgeAttacker drops every pending hit that came from the attacker, in every
// defender's list, and returns how many.
func (q *hitQueue) PurgeAttacker(attacker string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	id, ok := q.ids[attacker]
	if !ok {
		return 0
	}

	n := 0

	for def, l := range q.lists {
		for _, h := range l.Entries() {
			if h.AttackerType == playerUnitType && h.AttackerID == id {
				delete(q.pkts, h.Step)
			}
		}

		n += l.PurgeAttacker(playerUnitType, id)

		if l.Len() == 0 {
			delete(q.lists, def)
		}
	}

	return n
}

// drain removes and returns every pending hit in delivery order (defenders by
// the order of their entries' sequence numbers).
func (q *hitQueue) drain() []queuedHit {
	q.mu.Lock()
	defer q.mu.Unlock()

	var steps []int

	for _, l := range q.lists {
		for _, h := range l.Entries() {
			steps = append(steps, h.Step)
		}
	}

	sort.Ints(steps)

	out := make([]queuedHit, 0, len(steps))
	for _, s := range steps {
		out = append(out, q.pkts[s])
		delete(q.pkts, s)
	}

	q.lists = map[string]*d2unitmsg.HitList{}

	return out
}

// flushHits delivers the pending hits to their defenders.
func (g *GameServer) flushHits() {
	if g.soc == nil {
		return
	}

	for _, h := range g.soc.hits.drain() {
		if c := g.connByID(h.to); c != nil {
			if err := c.SendPacketToClient(h.pkt); err != nil {
				g.Errorf("PVP hit delivery to %s: %v", h.to, err)
			}
		}
	}
}

// hitLoop is the server's room flush for the hit lists.
func (g *GameServer) hitLoop() {
	t := time.NewTicker(hitFlushEvery)
	defer t.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-t.C:
			g.flushHits()
		}
	}
}
