package d2server

import (
	"sort"
	"sync"
	"time"

	"github.com/OpenDiablo2/OpenDiablo2/d2common/d2unitmsg"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
)

// The deferred hit-sequence list of Game.exe (UNIT\SUnitDmg.cpp, N16). VERIFIED
// this wave (Ghidra, D2_1_14b): the list hangs on the ATTACKER (unit +0xac,
// "pHitSequenceList"), not on the defender. A melee swing appends an entry
// keyed (attacker type, attacker id, defender type, defender id) with a copy
// of the damage struct; the action-frame event resolves it (0x57b4b0
// SRVDO_ResolveQueuedHitSequence, found by that four-part key in the attacker's
// list) if the defender is still in range, else drops it. The purge 0x57a900
// compares each entry with the list owner's own type and id and unlinks the
// matches, and is called from only three places: every server unit mode change
// (SERVER_SetUnitModeWithEvents 0x5515c0, so an interrupted swing never lands),
// PLAYER_FreeServerPlayerUnit 0x532430 and MONAI_FreeServerMonsterData 0x5714d0
// (the unit is freed).
//
// Here the owners are the attacking players, the entries are the PvP hits
// relayed by onPvPHit (already resolved by the attacker's client), the flush
// runs every hitFlushEvery (a frame of the 25 fps server) and a player who
// leaves the game takes the hits he has not yet delivered with him (the free
// path). UNVERIFIED: the exact flush point of the exe. NOT ported: the purge on
// a mode change (the relay carries no attacker mode and the hits are already
// resolved, so dropping them on a mode change would lose real damage).

// hitFlushEvery is one server frame.
const hitFlushEvery = 40 * time.Millisecond

// playerUnitType is the unit type of a player.
const playerUnitType = 0

type queuedHit struct {
	to  string
	pkt d2netpacket.NetPacket
}

// hitQueue holds the pending hits per attacker (the owner of the list in the exe).
type hitQueue struct {
	mu    sync.Mutex
	lists map[string]*d2unitmsg.HitList // by attacker id
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

	l := q.lists[attacker]
	if l == nil {
		l = &d2unitmsg.HitList{}
		q.lists[attacker] = l
	}

	q.seq++
	q.pkts[q.seq] = queuedHit{to: defender, pkt: pkt}
	l.Add(d2unitmsg.Hit{AttackerType: playerUnitType, AttackerID: q.unitID(attacker), Step: q.seq})
}

// Pending is the number of hits waiting for the defender.
func (q *hitQueue) Pending(defender string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := 0

	for _, h := range q.pkts {
		if h.to == defender {
			n++
		}
	}

	return n
}

// PurgeAttacker is 0x57a900 for a unit that is being freed: the entries of the
// unit's own list whose attacker is that unit (all of them: a list only holds
// its owner's swings) are unlinked. It returns how many.
func (q *hitQueue) PurgeAttacker(attacker string) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	id, ok := q.ids[attacker]
	l := q.lists[attacker]

	if !ok || l == nil {
		return 0
	}

	for _, h := range l.Entries() {
		if h.AttackerType == playerUnitType && h.AttackerID == id {
			delete(q.pkts, h.Step)
		}
	}

	n := l.PurgeAttacker(playerUnitType, id)

	if l.Len() == 0 {
		delete(q.lists, attacker)
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
		// hitLoop runs beside the packet manager, which adds and removes connections:
		// look the defender up under the server lock (an unlocked map read is a
		// fatal "concurrent map read and map write")
		g.RLock()
		c := g.connByID(h.to)
		g.RUnlock()

		if c != nil {
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
