package game

import (
	"math/rand"

	"github.com/oldschoolsysadmin/fightris/game/piece"
)

// Bag implements the 7-bag randomizer: every tetromino appears exactly once per
// bag before any type can repeat. This guarantees a player never waits more than
// 12 pieces for any given type — a core Tetris Guideline requirement.
//
// Design: kept separate from State (single-responsibility). State calls bag.Next()
// without knowing how randomization works, making the bag independently testable
// and easy to swap out (e.g. seeded bag for replays, weighted bag for a powerup).
type Bag struct {
	queue []piece.PieceType
}

// NewBag returns a Bag seeded with one shuffled set of all 7 piece types.
func NewBag() *Bag {
	b := &Bag{}
	b.refill()
	return b
}

// Next draws and returns the next piece type, refilling with a fresh shuffled
// bag when the current one is exhausted.
func (b *Bag) Next() piece.PieceType {
	if len(b.queue) == 0 {
		b.refill()
	}
	pt := b.queue[0]
	b.queue = b.queue[1:]
	return pt
}

func (b *Bag) refill() {
	next := make([]piece.PieceType, len(piece.AllTypes))
	copy(next, piece.AllTypes[:])
	// rand.Shuffle is Fisher-Yates under the hood — idiomatic Go, no manual loop needed.
	rand.Shuffle(len(next), func(i, j int) { next[i], next[j] = next[j], next[i] })
	b.queue = append(b.queue, next...)
}
