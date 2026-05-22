package engine

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/game"
)

// GravityEvent and LockEvent are posted to the tcell screen by timer goroutines
// and routed back to HandleGravity / HandleLock by the main event loop.
// The Player field lets a single event loop serve two independent engines.
type GravityEvent struct{ Player int }
type LockEvent struct{ Player, Gen int }

// LockDelay is the grace window after a piece lands before it locks.
// Any move or rotation within this window resets the timer.
const LockDelay = 500 * time.Millisecond

// gravityTable maps level (1-indexed) to drop interval.
// Index 0 is unused; values approximate the Tetris Guideline formula.
var gravityTable = [21]time.Duration{
	0,
	800 * time.Millisecond, // 1
	717 * time.Millisecond, // 2
	633 * time.Millisecond, // 3
	550 * time.Millisecond, // 4
	467 * time.Millisecond, // 5
	383 * time.Millisecond, // 6
	300 * time.Millisecond, // 7
	217 * time.Millisecond, // 8
	133 * time.Millisecond, // 9
	100 * time.Millisecond, // 10
	83 * time.Millisecond,  // 11
	83 * time.Millisecond,  // 12
	67 * time.Millisecond,  // 13
	67 * time.Millisecond,  // 14
	67 * time.Millisecond,  // 15
	50 * time.Millisecond,  // 16
	50 * time.Millisecond,  // 17
	50 * time.Millisecond,  // 18
	33 * time.Millisecond,  // 19
	17 * time.Millisecond,  // 20
}

func gravityInterval(level int) time.Duration {
	if level < 1 {
		level = 1
	}
	if level >= len(gravityTable) {
		level = len(gravityTable) - 1
	}
	return gravityTable[level]
}

// GameEngine manages one player's game state, gravity timer, and lock-delay timer.
// Timers run in background goroutines and post GravityEvent / LockEvent to the
// tcell screen; the main event loop routes those back here via HandleGravity and
// HandleLock.
//
// Garbage routing is left to the caller: locking methods return the number of lines
// cleared so the caller can decide what to do — call AddGarbage directly for local
// play, or send a network message for LAN.
type GameEngine struct {
	State  *game.State
	id     int
	screen tcell.Screen
	ih     *game.InputHandler

	lockTimer *time.Timer
	lockGen   int
	gravTimer *time.Timer
}

// New creates a GameEngine for the given player index. Call Start to begin play.
func New(s tcell.Screen, playerID int) *GameEngine {
	st := game.New()
	return &GameEngine{
		State:  st,
		id:     playerID,
		screen: s,
		ih:     game.NewInputHandler(st),
	}
}

// Start spawns the first piece and starts the gravity timer.
// Returns false only if the board is immediately blocked on a fresh state
// (shouldn't happen in practice).
func (e *GameEngine) Start() bool {
	if !e.State.SpawnNext() {
		return false
	}
	e.scheduleGravity()
	return true
}

// Stop cancels all pending timers. Safe to call multiple times.
func (e *GameEngine) Stop() {
	if e.gravTimer != nil {
		e.gravTimer.Stop()
	}
	if e.lockTimer != nil {
		e.lockTimer.Stop()
	}
}

// HandleAction applies a player input to the game state.
//
// HardDrop is the only action that locks a piece: the piece drops instantly,
// locks onto the board, and the next piece spawns. Returns the lines cleared
// by the lock and whether the game is still alive.
//
// All other actions (move, rotate, soft drop) update the piece position and
// adjust lock-delay timers as needed. Always return (0, true).
func (e *GameEngine) HandleAction(a game.Action) (cleared int, alive bool) {
	if a == game.ActionHardDrop {
		e.ih.Handle(a)
		return e.lockAndSpawn()
	}
	e.ih.Handle(a)
	if e.State.IsGrounded() {
		e.startLock()
	} else {
		e.cancelLock()
	}
	return 0, true
}

// HandleGravity advances the piece one row down and reschedules the next gravity
// tick. If the piece can't move down, the lock-delay timer is started.
func (e *GameEngine) HandleGravity() {
	if !e.State.MoveDown() {
		e.startLock()
	} else {
		e.cancelLock()
	}
	e.scheduleGravity()
}

// HandleLock processes a lock-delay timer event. If gen matches the current
// generation, the active piece is locked and the next is spawned.
// Returns lines cleared and whether the game is still alive.
// Stale events (gen mismatch, from timers reset by a move) return (0, true).
func (e *GameEngine) HandleLock(gen int) (cleared int, alive bool) {
	if gen != e.lockGen {
		return 0, true
	}
	return e.lockAndSpawn()
}

// lockAndSpawn locks the active piece, clears full rows, and spawns the next.
// Garbage routing is the caller's responsibility; this method only returns
// the line count so the caller can act on it.
func (e *GameEngine) lockAndSpawn() (cleared int, alive bool) {
	cleared = e.State.LockActive()
	e.cancelLock()
	if !e.State.SpawnNext() {
		return cleared, false
	}
	return cleared, true
}

func (e *GameEngine) startLock() {
	e.lockGen++
	gen := e.lockGen
	if e.lockTimer != nil {
		e.lockTimer.Stop()
	}
	e.lockTimer = time.AfterFunc(LockDelay, func() {
		e.screen.PostEvent(tcell.NewEventInterrupt(LockEvent{e.id, gen}))
	})
}

func (e *GameEngine) cancelLock() {
	if e.lockTimer != nil {
		e.lockTimer.Stop()
		e.lockTimer = nil
	}
	e.lockGen++
}

// scheduleGravity re-reads level and GravityMultiplier each time so that
// level-ups and powerup effects take hold on the very next tick.
func (e *GameEngine) scheduleGravity() {
	interval := time.Duration(float64(gravityInterval(e.State.Level)) / e.State.GravityMultiplier)
	e.gravTimer = time.AfterFunc(interval, func() {
		e.screen.PostEvent(tcell.NewEventInterrupt(GravityEvent{e.id}))
	})
}
