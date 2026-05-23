package netplay

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/game/board"
)

// TestSnapshotRoundTrip is the most important guard: a board encoded to JSON and
// decoded back must reconstruct exactly, since the receiver renders this verbatim.
func TestSnapshotRoundTrip(t *testing.T) {
	src := game.New()
	if !src.SpawnNext() {
		t.Fatal("SpawnNext failed on fresh state")
	}
	// Dirty the board a little so we're not just round-tripping zeros.
	src.Board.Set(0, 0, board.Cell(3))
	src.Board.Set(0, 9, board.Garbage)
	src.Board.Set(5, 4, board.Cell(1))
	src.Score, src.Level, src.LinesCleared = 1234, 7, 65

	wire, err := (&Packet{Kind: KindSnapshot, Snapshot: NewSnapshot(42, src)}).Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	pkt, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pkt.Kind != KindSnapshot || pkt.Snapshot == nil {
		t.Fatalf("wrong kind/payload: %+v", pkt)
	}

	dst := game.New()
	pkt.Snapshot.ApplyTo(dst)

	for r := 0; r < src.Board.Height; r++ {
		for c := 0; c < src.Board.Width; c++ {
			if got, want := dst.Board.Get(r, c), src.Board.Get(r, c); got != want {
				t.Fatalf("cell (%d,%d): got %d want %d", r, c, got, want)
			}
		}
	}
	if dst.Active != src.Active {
		t.Fatalf("active: got %+v want %+v", dst.Active, src.Active)
	}
	if dst.NextPiece != src.NextPiece || dst.Score != src.Score ||
		dst.Level != src.Level || dst.LinesCleared != src.LinesCleared {
		t.Fatalf("stats mismatch: got next=%d score=%d level=%d lines=%d",
			dst.NextPiece, dst.Score, dst.Level, dst.LinesCleared)
	}
}

// TestGarbageEffectRoundTrip checks the extensible Effect envelope: encode a
// garbage effect, decode the envelope, then decode the kind-specific payload.
func TestGarbageEffectRoundTrip(t *testing.T) {
	eff, err := NewGarbageEffect(99, 4)
	if err != nil {
		t.Fatalf("NewGarbageEffect: %v", err)
	}
	wire, err := (&Packet{Kind: KindEffect, Effect: eff}).Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	pkt, err := Decode(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pkt.Effect == nil || pkt.Effect.Kind != EffectGarbage || pkt.Effect.ID != 99 {
		t.Fatalf("bad effect envelope: %+v", pkt.Effect)
	}
	var g GarbageEffect
	if err := json.Unmarshal(pkt.Effect.Data, &g); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if g.Rows != 4 {
		t.Fatalf("rows: got %d want 4", g.Rows)
	}
}

// TestHandshake spins up a real localhost host+joiner and confirms both ends
// complete the UDP handshake. Dial retries, so a slightly-late Listen is fine.
func TestHandshake(t *testing.T) {
	const addr = "127.0.0.1:47654"

	hostCh := make(chan *Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		c, err := Listen(addr, 1)
		if err != nil {
			errCh <- err
			return
		}
		hostCh <- c
	}()

	joinConn, err := Dial(addr, 1)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer joinConn.Close()

	select {
	case hostConn := <-hostCh:
		defer hostConn.Close()
		if hostConn.peer == nil || joinConn.peer == nil {
			t.Fatal("peer address not learned on both ends")
		}
	case err := <-errCh:
		t.Fatalf("Listen: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("handshake timed out")
	}
}
