// file: fightris/game/state.go

package game

import (
	"github.com/oldschoolsysadmin/fightris/game/board"
	"github.com/oldschoolsysadmin/fightris/game/piece"
)

const (
	BoardWidth   = 10
	BoardHeight  = 22 // 20 visible + 2 buffer rows at top for spawning
	VisibleRows  = 20
	SpawnRow     = 20 // pivot row for newly spawned pieces (0-indexed from bottom)
	SpawnCol     = 4  // pivot col — centers most pieces on a 10-wide board
)

// State holds everything needed to represent a game in progress.
// The loop layer will own a State and drive it via the methods below.
type State struct {
	Board        *board.Board
	Active       piece.Active
	NextPiece    piece.PieceType // the piece that will spawn after Active locks; read by the renderer
	Score        int
	LinesCleared int
	Level        int
	GameOver     bool

	// bag is private: callers read NextPiece instead of reaching into the bag directly.
	// This keeps State as the single source of truth the renderer and game loop talk to.
	bag *Bag
}

// New creates a fresh game state with an empty board and no active piece.
// Call SpawnNext before starting the loop.
func New() *State {
	bag := NewBag()
	return &State{
		Board:     board.New(BoardWidth, BoardHeight),
		Level:     1,
		bag:       bag,
		NextPiece: bag.Next(), // seed the preview before the first spawn
	}
}

// -- Collision -----------------------------------------------------------

// collides returns true if the given Active piece overlaps any filled cell
// or lies out of bounds on the board.
func (s *State) collides(a piece.Active) bool {
	for _, m := range a.AbsoluteMinoes() {
		if !s.Board.InBounds(m.Row, m.Col) {
			return true
		}
		if s.Board.Get(m.Row, m.Col) != board.Empty {
			return true
		}
	}
	return false
}

// -- Piece Lifecycle -----------------------------------------------------

// SpawnNext promotes NextPiece to the active piece, then draws the following
// piece from the bag so the preview is always one ahead.
// Returns false (and sets GameOver) if the spawn position is blocked — game over.
func (s *State) SpawnNext() bool {
	a := piece.NewActive(s.NextPiece, SpawnRow, SpawnCol)
	if s.collides(a) {
		s.GameOver = true
		return false
	}
	s.Active = a
	s.NextPiece = s.bag.Next() // draw the upcoming piece so NextPiece is always "what's after Active"
	return true
}

// LockActive writes the active piece's minoes onto the board as filled cells,
// then clears any full rows and updates score/level.
// Call this after the piece can no longer move down.
func (s *State) LockActive() int {
	for _, m := range s.Active.AbsoluteMinoes() {
		s.Board.Set(m.Row, m.Col, board.Cell(s.Active.Type))
	}
	cleared := s.Board.ClearFullRows()
	s.LinesCleared += cleared
	s.Score += scoreForClears(cleared, s.Level)
	s.Level = (s.LinesCleared / 10) + 1
	return cleared
}

// -- Movement ------------------------------------------------------------

// MoveLeft attempts to shift the active piece one column left.
// Returns true if the move succeeded.
func (s *State) MoveLeft() bool {
	return s.tryMove(s.Active.Moved(0, -1))
}

// MoveRight attempts to shift the active piece one column right.
func (s *State) MoveRight() bool {
	return s.tryMove(s.Active.Moved(0, 1))
}

// MoveDown attempts to drop the active piece one row.
// Returns true if successful; false means the piece has landed.
func (s *State) MoveDown() bool {
	return s.tryMove(s.Active.Moved(-1, 0))
}

// HardDrop instantly drops the piece to its lowest valid position.
// Returns the number of rows dropped. Caller must call LockActive + SpawnPiece.
func (s *State) HardDrop() int {
	dropped := 0
	for s.tryMove(s.Active.Moved(-1, 0)) {
		dropped++
	}
	return dropped
}

// tryMove applies the candidate position if it doesn't collide.
func (s *State) tryMove(candidate piece.Active) bool {
	if s.collides(candidate) {
		return false
	}
	s.Active = candidate
	return true
}

// -- Rotation ------------------------------------------------------------

// RotateCW attempts a clockwise rotation with SRS wall kicks.
// Returns true if a valid position was found.
func (s *State) RotateCW() bool {
	return s.tryRotate(s.Active.RotatedCW())
}

// RotateCCW attempts a counter-clockwise rotation with SRS wall kicks.
func (s *State) RotateCCW() bool {
	return s.tryRotate(s.Active.RotatedCCW())
}

// tryRotate tests each SRS kick offset for the given rotated candidate.
func (s *State) tryRotate(rotated piece.Active) bool {
	from := s.Active.CurrentRotation()
	to := rotated.CurrentRotation()
	kicks := piece.KickOffsets(s.Active.Type, from, to)

	for _, k := range kicks {
		candidate := rotated.Moved(k.Row, k.Col)
		if !s.collides(candidate) {
			s.Active = candidate
			return true
		}
	}
	return false
}

// -- Ghost Piece ---------------------------------------------------------

// GhostRow returns the lowest row the active piece can reach — used by the
// renderer to draw the ghost/shadow piece.
func (s *State) GhostRow() int {
	candidate := s.Active
	for {
		next := candidate.Moved(-1, 0)
		if s.collides(next) {
			break
		}
		candidate = next
	}
	return candidate.PivotRow
}

// -- Scoring -------------------------------------------------------------

// scoreForClears returns points for clearing n lines at the given level,
// using the standard Tetris Guideline scoring table.
func scoreForClears(lines, level int) int {
	base := [5]int{0, 100, 300, 500, 800}
	if lines > 4 {
		lines = 4
	}
	return base[lines] * level
}
