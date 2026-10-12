package d2server

import (
	"bufio"
	"errors"
	"net"
	"sync"

	"github.com/OpenDiablo2/OpenDiablo2/d2core/d2hero"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2client/d2clientconnectiontype"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2gs"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2gsnet"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket"
	"github.com/OpenDiablo2/OpenDiablo2/d2networking/d2netpacket/d2netpackettype"
)

// gsClientConnection is a remote client that speaks the Diablo II game
// protocol (OD2_PROTO=d2gs): engine packets are translated by d2gsnet and sent
// as Huffman compressed, length prefixed blobs.
type gsClientConnection struct {
	id          string
	conn        net.Conn
	side        *d2gsnet.ServerSide
	mu          sync.Mutex
	playerState *d2hero.HeroState
}

// Close closes the connection (the idle kick).
func (c *gsClientConnection) Close() error { return c.conn.Close() }

func (c *gsClientConnection) GetUniqueID() string { return c.id }

func (c *gsClientConnection) GetConnectionType() d2clientconnectiontype.ClientConnectionType {
	return d2clientconnectiontype.LANClient
}

func (c *gsClientConnection) GetPlayerState() *d2hero.HeroState { return c.playerState }

func (c *gsClientConnection) SetPlayerState(s *d2hero.HeroState) { c.playerState = s }

func (c *gsClientConnection) SendPacketToClient(p d2netpacket.NetPacket) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	pkts, err := c.side.Encode(p)
	if err != nil {
		return err
	}

	blob, err := d2gs.EncodeStream(pkts...)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(blob)

	return err
}

// handleConnectionD2GS is handleConnection for the game protocol. The first
// thing a client must send is its join (0x68 + the tunnelled hero).
func (g *GameServer) handleConnectionD2GS(conn net.Conn) {
	g.Infof("Accepting connection: %s (d2gs)", conn.RemoteAddr().String())

	defer func() {
		_ = conn.Close()
	}()

	var (
		client ClientConnection
		left   bool
		reader = bufio.NewReader(conn)
		side   = &d2gsnet.ServerSide{IDs: g.unitIDs, Info: g.hostInfo}
	)

	// a client that vanishes without saying goodbye is removed like one that left
	defer func() {
		if client == nil {
			return
		}

		select {
		case <-g.ctx.Done():
		default:
			bye, err := d2netpacket.CreatePlayerDisconnectRequestPacket(client.GetUniqueID())
			if err == nil && !left {
				g.packetManagerChan <- ReceivedPacket{Client: client, Packet: bye}
			}
		}
	}()

	for {
		plain, err := d2gs.ReadBlob(reader)
		if err != nil {
			if !d2gsnet.IsClosed(err) {
				g.Errorf("d2gs read from %s: %v", conn.RemoteAddr(), err)
			}

			return
		}

		g.refreshInfo(side)

		packets, ignored, err := side.Decode(plain)
		if err != nil {
			g.Errorf("d2gs decode from %s: %v", conn.RemoteAddr(), err)
			return
		}

		if ignored > 0 {
			g.Debugf("d2gs: %d bytes of unused/unknown packets skipped", ignored)
		}

		for _, packet := range packets {
			if client == nil {
				// the exe compares the version field of the join with 0xE (0x53cd50)
				if j := side.Joined(); j.Name != "" && j.Version != d2gs.GameVersion {
					g.Infof("JOIN refused: version %d, want %d", j.Version, d2gs.GameVersion)
					g.rejectJoin(conn, d2gs.JoinErrVersion)

					return
				}

				if packet.PacketType != d2netpackettype.PlayerConnectionRequest {
					g.Infof("Closing connection with %s: did not receive new player connection request...", conn.RemoteAddr())
					return
				}

				gs := &gsClientConnection{conn: conn, side: side}

				client, err = g.registerConnection(packet.PacketData, conn, func(id string) ClientConnection {
					gs.id = id
					return gs
				})
				if err != nil {
					g.rejectJoin(conn, joinErrCode(err))

					return
				}

				side.Pos = func() (float64, float64) {
					st := gs.GetPlayerState()
					return st.X, st.Y
				}
			}

			if packet.PacketType == d2netpackettype.PlayerDisconnectionNotification {
				left = true
			}

			select {
			case <-g.ctx.Done():
				return
			default:
				g.packetManagerChan <- ReceivedPacket{Client: client, Packet: packet}
			}
		}
	}
}

// refreshInfo keeps the game info of a session in step with the host's
// (written once, when the host's hero connects).
func (g *GameServer) refreshInfo(side *d2gsnet.ServerSide) {
	g.RLock()
	side.Info = g.hostInfo
	g.RUnlock()
}

// joinErrCode maps a registration error to the exe's refusal code (0 for
// errors that have none: bad request, no hero).
func joinErrCode(err error) uint32 {
	switch {
	case errors.Is(err, errServerFull):
		return d2gs.JoinErrFull
	case errors.Is(err, errNameInUse), errors.Is(err, errPlayerAlreadyExists):
		return d2gs.JoinErrNameOrGame
	}

	return 0
}

// rejectJoin tells a refused client why (packet 0xB4) before the connection
// is closed, as the exe does (SCMD_SendOpB4 then a disconnect).
func (g *GameServer) rejectJoin(conn net.Conn, code uint32) {
	if code == 0 {
		return
	}

	blob, err := d2gs.EncodeStream(d2gs.JoinReject{Code: code}.Marshal())
	if err != nil {
		g.Errorf("join reject: %v", err)
		return
	}

	if _, err := conn.Write(blob); err != nil {
		g.Debugf("join reject write: %v", err)
	}
}
