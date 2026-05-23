// Package netplay is the LAN transport for Fightris: a tiny UDP protocol that
// lets two machines play head-to-head.
//
// Design in one breath: each machine simulates ONLY its own board, authoritatively.
// It broadcasts SNAPSHOTS of that board so the peer can render it, and it sends
// EFFECTS — apply-once commands — when something it does should change the peer's
// game (garbage today; powerups later). Nothing the peer sends is ever simulated
// locally beyond applying an effect; the opponent's board on your screen is a
// picture, not a running game. That means no determinism, no seeded bag, and no
// lock-step — a lost packet is simply superseded (snapshots) or resent (effects).
package netplay

import (
	"encoding/json"

	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/game/board"
	"github.com/oldschoolsysadmin/fightris/game/piece"
)

// Packet kinds. These three are the STABLE core of the protocol. Powerups extend
// the protocol by adding Effect kinds (see Effect), never new Packet kinds — the
// envelope stays frozen while effects are the open-ended extension point.
const (
	KindHello    = "hello"    // handshake; no payload
	KindSnapshot = "snapshot" // sender's board, for the peer to render (latest-wins)
	KindEffect   = "effect"   // a command the receiver applies to its own game (apply-once)
)

// Packet is the top-level UDP envelope. Exactly one payload pointer is set,
// selected by Kind. We use JSON: a 10x22 board is a few hundred bytes, trivial on
// a LAN, and being human-readable means we can log raw packets and just read them.
type Packet struct {
	Kind     string    `json:"kind"`
	Snapshot *Snapshot `json:"snapshot,omitempty"`
	Effect   *Effect   `json:"effect,omitempty"`
}

// Encode marshals a packet to a single UDP datagram's worth of bytes.
func (p *Packet) Encode() ([]byte, error) { return json.Marshal(p) }

// Decode parses a datagram into a Packet. Callers must treat the result as
// untrusted network input and nil-check the payload for the kind they expect.
func Decode(b []byte) (*Packet, error) {
	var p Packet
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Snapshot is a renderable copy of one player's board. It is idempotent: the
// receiver keeps only the highest Seq seen and discards anything older, so a lost
// snapshot is just superseded by the next one. The receiver never simulates this —
// it overwrites a shadow State wholesale and draws it.
type Snapshot struct {
	Seq uint64 `json:"seq"` // monotonic per sender; receiver drops seq <= last seen

	// Cells is the board grid, row-major: index = row*Width + col, len = Width*Height.
	// A flat slice keeps the JSON compact versus a nested array.
	Cells []uint8 `json:"cells"`

	// Active piece (all fields exported on piece.Active, so reconstruction is exact).
	Type     uint8 `json:"type"`
	Rot      int   `json:"rot"`
	PivotRow int   `json:"pivotRow"`
	PivotCol int   `json:"pivotCol"`

	Next     uint8 `json:"next"` // next-piece preview type
	Score    int   `json:"score"`
	Level    int   `json:"level"`
	Lines    int   `json:"lines"`
	GameOver bool  `json:"gameOver"` // doubles as the deathmatch signal: true => sender lost
}

// NewSnapshot copies the renderable parts of a live State into a Snapshot.
// Reads only exported State fields; the bag and other internals stay home.
func NewSnapshot(seq uint64, st *game.State) *Snapshot {
	w, h := st.Board.Width, st.Board.Height
	cells := make([]uint8, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells[r*w+c] = uint8(st.Board.Get(r, c))
		}
	}
	return &Snapshot{
		Seq:      seq,
		Cells:    cells,
		Type:     uint8(st.Active.Type),
		Rot:      int(st.Active.Rot),
		PivotRow: st.Active.PivotRow,
		PivotCol: st.Active.PivotCol,
		Next:     uint8(st.NextPiece),
		Score:    st.Score,
		Level:    st.Level,
		Lines:    st.LinesCleared,
		GameOver: st.GameOver,
	}
}

// ApplyTo overwrites a shadow State in place from the snapshot. We mutate an
// existing State rather than allocate a new one each frame — it's only ever
// touched by the single event-loop goroutine, so this is safe and allocation-free.
// Malformed packets (wrong cell count) are ignored rather than panicking, since
// the input is untrusted.
func (s *Snapshot) ApplyTo(st *game.State) {
	w, h := st.Board.Width, st.Board.Height
	if len(s.Cells) != w*h {
		return
	}
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			st.Board.Set(r, c, board.Cell(s.Cells[r*w+c]))
		}
	}
	st.Active = piece.Active{
		Type:     piece.PieceType(s.Type),
		Rot:      piece.Rotation(s.Rot),
		PivotRow: s.PivotRow,
		PivotCol: s.PivotCol,
	}
	st.NextPiece = piece.PieceType(s.Next)
	st.Score = s.Score
	st.Level = s.Level
	st.LinesCleared = s.Lines
	st.GameOver = s.GameOver
}

// Effect kinds. Garbage is the only one today; each future powerup that touches
// the opponent (reverse their controls, flip their display, swap queues, ...) is a
// new constant here plus a payload struct plus a case in the receiver's dispatch.
const (
	EffectGarbage = "garbage"
	// Milestone 4 (sketch): EffectReverseControls, EffectFlipDisplay, EffectSwapQueue, ...
)

// Effect is a command from the peer to apply to the RECEIVER's own game exactly
// once. Delivery is apply-once: the receiver dedups by ID and ignores repeats (the
// --spam knob sends each effect N times so loss is unlikely to drop an attack).
//
// Data is the kind-specific payload, left as RawMessage so the envelope doesn't
// need to know every effect type up front — this is the protocol's extension seam
// for powerups. Decode it into the matching payload struct based on Kind.
type Effect struct {
	ID   uint64          `json:"id"`             // unique per sender; the dedup key
	Kind string          `json:"kind"`           // EffectGarbage, ...
	Data json.RawMessage `json:"data,omitempty"` // payload, decoded per Kind
}

// GarbageEffect is the EffectGarbage payload: how many junk rows to push onto the
// receiver's board. The hole column is chosen locally by the receiver's
// AddGarbage, so it never crosses the wire — only the count does.
type GarbageEffect struct {
	Rows int `json:"rows"`
}

// NewGarbageEffect builds an apply-once garbage attack with the given dedup ID.
func NewGarbageEffect(id uint64, rows int) (*Effect, error) {
	data, err := json.Marshal(GarbageEffect{Rows: rows})
	if err != nil {
		return nil, err
	}
	return &Effect{ID: id, Kind: EffectGarbage, Data: data}, nil
}
