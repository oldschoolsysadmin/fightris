package netplay

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/game"
)

// Incoming carries a decoded, deduplicated packet from the receive goroutine to
// the main event loop. We hand it over through tcell's own event queue (see Run),
// so the loop type-switches on it inside its EventInterrupt case exactly like it
// does for engine.GravityEvent / engine.LockEvent. The whole point: every mutation
// of game state happens on the one loop goroutine, so game.State needs no locks.
type Incoming struct {
	Packet *Packet
}

// Conn is one end of a two-player UDP link.
//
// Concurrency contract:
//   - send methods (SendSnapshot, SendGarbage) are called ONLY from the event-loop
//     goroutine, so the send-side counters need no lock.
//   - the dedup state (lastSnapSeq, seenEffects) is touched ONLY by the receive
//     goroutine started in Run.
//
// Those two sets never overlap, so the whole struct is lock-free by construction.
type Conn struct {
	udp  *net.UDPConn
	peer *net.UDPAddr
	spam int

	// send-side counters (event-loop goroutine only)
	sendSeq uint64 // snapshot sequence; lets the peer drop stale snapshots
	sendID  uint64 // effect id; the peer's apply-once dedup key

	// receive-side dedup (recv goroutine only)
	lastSnapSeq uint64
	seenEffects map[uint64]bool
}

func newConn(udp *net.UDPConn, peer *net.UDPAddr, spam int) *Conn {
	if spam < 1 {
		spam = 1
	}
	return &Conn{
		udp:         udp,
		peer:        peer,
		spam:        spam,
		seenEffects: make(map[uint64]bool),
	}
}

// Listen is the host side of the handshake. It binds the given UDP address, then
// BLOCKS until the first datagram arrives — whoever sent it becomes our peer. It
// replies with a HELLO so the joiner knows we're live and stops resending.
//
// Blocking here is deliberate: the caller runs this before initializing tcell, so
// the terminal is still normal and it can print a friendly "waiting..." line.
func Listen(addr string, spam int) (*Conn, error) {
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udp, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		return nil, err
	}
	log.Printf("[net] host listening on %s, waiting for peer...", addr)

	buf := make([]byte, 65535)
	n, peer, err := udp.ReadFromUDP(buf)
	if err != nil {
		udp.Close()
		return nil, err
	}
	log.Printf("[net] host: peer connected from %s (first packet %d bytes)", peer, n)

	c := newConn(udp, peer, spam)
	c.send(&Packet{Kind: KindHello}) // ack so the joiner can stop spamming HELLO
	return c, nil
}

// dialAttempts caps how long Dial retries before giving up (attempts * 500ms).
const dialAttempts = 20

// Dial is the joiner side of the handshake. It binds an ephemeral local port and
// resends HELLO to the host until it gets any datagram back (or gives up). UDP is
// connectionless, so "connecting" just means both ends learn each other's address.
func Dial(addr string, spam int) (*Conn, error) {
	peer, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	udp, err := net.ListenUDP("udp", nil) // nil local addr => OS picks a free port
	if err != nil {
		return nil, err
	}

	hello, err := (&Packet{Kind: KindHello}).Encode()
	if err != nil {
		udp.Close()
		return nil, err
	}

	buf := make([]byte, 65535)
	for attempt := 1; ; attempt++ {
		if _, err := udp.WriteToUDP(hello, peer); err != nil {
			udp.Close()
			return nil, err
		}
		udp.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, from, err := udp.ReadFromUDP(buf)
		if err == nil {
			log.Printf("[net] join: connected to host %s (reply %d bytes)", from, n)
			break
		}
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			if attempt >= dialAttempts {
				udp.Close()
				return nil, fmt.Errorf("no response from host %s after %d attempts", peer, attempt)
			}
			log.Printf("[net] join: no reply (attempt %d/%d), resending HELLO", attempt, dialAttempts)
			continue
		}
		udp.Close()
		return nil, err
	}
	udp.SetReadDeadline(time.Time{}) // clear the deadline for the long-lived recv loop

	return newConn(udp, peer, spam), nil
}

// Run starts the receive loop in a background goroutine. Each datagram is decoded
// and deduplicated HERE, then the surviving packet is posted to the event loop via
// tcell's queue. Doing dedup in the goroutine means the loop only ever sees fresh
// messages and never has to think about ordering or duplicates.
func (c *Conn) Run(screen tcell.Screen) {
	go func() {
		buf := make([]byte, 65535)
		for {
			n, _, err := c.udp.ReadFromUDP(buf)
			if err != nil {
				// Close() unblocks ReadFromUDP with an error; that's our exit signal.
				log.Printf("[net] recv loop ending: %v", err)
				return
			}
			pkt, err := Decode(buf[:n])
			if err != nil {
				log.Printf("[net] decode error (%d bytes): %v", n, err)
				continue
			}

			switch pkt.Kind {
			case KindHello:
				continue // handshake only; nothing to do once we're playing
			case KindSnapshot:
				if pkt.Snapshot == nil || pkt.Snapshot.Seq <= c.lastSnapSeq {
					continue // stale or malformed; latest-wins
				}
				c.lastSnapSeq = pkt.Snapshot.Seq
			case KindEffect:
				if pkt.Effect == nil || c.seenEffects[pkt.Effect.ID] {
					continue // duplicate; apply-once
				}
				c.seenEffects[pkt.Effect.ID] = true
			default:
				log.Printf("[net] unknown packet kind %q", pkt.Kind)
				continue
			}

			screen.PostEvent(tcell.NewEventInterrupt(Incoming{Packet: pkt}))
		}
	}()
}

// send writes a packet to the peer `spam` times. UDP gives no delivery guarantee;
// redundant copies ARE our reliability strategy — snapshots are latest-wins and
// effects are deduped by ID, so duplicates are harmless and loss is unlikely to
// drop everything. Call only from the event-loop goroutine.
func (c *Conn) send(p *Packet) {
	b, err := p.Encode()
	if err != nil {
		log.Printf("[net] encode error: %v", err)
		return
	}
	for i := 0; i < c.spam; i++ {
		if _, err := c.udp.WriteToUDP(b, c.peer); err != nil {
			log.Printf("[net] write error: %v", err)
			return
		}
	}
}

// SendSnapshot broadcasts our current board for the peer to render.
func (c *Conn) SendSnapshot(st *game.State) {
	c.sendSeq++
	c.send(&Packet{Kind: KindSnapshot, Snapshot: NewSnapshot(c.sendSeq, st)})
}

// SendGarbage sends an apply-once garbage attack of n rows to the peer.
func (c *Conn) SendGarbage(rows int) {
	if rows <= 0 {
		return
	}
	c.sendID++
	eff, err := NewGarbageEffect(c.sendID, rows)
	if err != nil {
		log.Printf("[net] garbage encode error: %v", err)
		return
	}
	c.send(&Packet{Kind: KindEffect, Effect: eff})
	log.Printf("[net] sent garbage rows=%d id=%d", rows, c.sendID)
}

// Close shuts the socket, which unblocks and ends the receive goroutine.
func (c *Conn) Close() error { return c.udp.Close() }
