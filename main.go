// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"log"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/render"
)

// gravityEvent is posted by the gravity ticker on each interval.
// lockEvent is posted by the lock-delay timer; gen must match the current
// lock generation or the event is stale and should be ignored.
//
// Using typed structs as EventInterrupt payloads lets the event loop
// type-switch on them cleanly instead of comparing magic strings or
// trying to maintain separate channels.
type gravityEvent struct{}
type lockEvent struct{ gen int }

const lockDelay = 500 * time.Millisecond

func keyToAction(ev *tcell.EventKey) game.Action {
	switch ev.Key() {
	case tcell.KeyLeft:
		return game.ActionLeft
	case tcell.KeyRight:
		return game.ActionRight
	case tcell.KeyDown:
		return game.ActionSoftDrop
	case tcell.KeyUp:
		return game.ActionRotateCW
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return game.ActionQuit
	}
	switch ev.Rune() {
	case ' ':
		return game.ActionHardDrop
	case 'z', 'Z':
		return game.ActionRotateCCW
	case 'x', 'X':
		return game.ActionRotateCW
	case 'q', 'Q':
		return game.ActionQuit
	}
	return game.ActionNone
}

func main() {
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := s.Init(); err != nil {
		log.Fatal(err)
	}
	defer s.Fini()

	st := game.New()
	ih := game.NewInputHandler(st)

	// Lock-delay state — lives here, not in game.State, because timing is an
	// I/O concern; game.State is pure game logic with no awareness of time.
	var lockTimer *time.Timer
	lockGen := 0

	// startLock begins (or resets) the lock-delay grace period.
	// Bumping lockGen before capturing it in the closure means any lock event
	// already in the queue carries an old gen and will be ignored.
	startLock := func() {
		lockGen++
		gen := lockGen
		if lockTimer != nil {
			lockTimer.Stop()
		}
		lockTimer = time.AfterFunc(lockDelay, func() {
			s.PostEvent(tcell.NewEventInterrupt(lockEvent{gen}))
		})
	}

	// cancelLock stops any pending lock timer and invalidates in-flight events.
	cancelLock := func() {
		if lockTimer != nil {
			lockTimer.Stop()
			lockTimer = nil
		}
		lockGen++ // stale lock events in the queue will see gen != lockGen and be dropped
	}

	// lockAndSpawn is the single path for locking a placed piece and spawning
	// the next one. Both the lock-delay timer and hard drop converge here so
	// lock-event hooks fire exactly once per piece.
	lockAndSpawn := func() bool {
		st.LockActive()
		cancelLock() // reset lock state for the incoming piece
		return st.SpawnNext()
	}

	if !st.SpawnNext() {
		return
	}

	// Gravity ticker — posts gravityEvent so the loop can distinguish it from
	// lock events via type switch on the EventInterrupt payload.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			s.PostEvent(tcell.NewEventInterrupt(gravityEvent{}))
		}
	}()

	draw := func() {
		s.Clear()
		render.Draw(s, st, 0, 0)
		s.Show()
	}

	draw()

	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			action := keyToAction(ev)
			if !ih.Handle(action) || st.GameOver {
				return
			}
			if action == game.ActionHardDrop {
				// Hard drop bypasses lock delay — lock immediately.
				if !lockAndSpawn() {
					return
				}
			} else {
				// After any move or rotate: if the piece is grounded,
				// start/reset the lock timer; if it moved to a safe height,
				// cancel any pending lock.
				if st.IsGrounded() {
					startLock()
				} else {
					cancelLock()
				}
			}
		case *tcell.EventInterrupt:
			switch data := ev.Data().(type) {
			case gravityEvent:
				if !st.MoveDown() {
					startLock() // piece just landed — begin grace period
				} else {
					cancelLock() // piece is falling freely; no lock pending
				}
			case lockEvent:
				// A stale event (from a timer that was reset by a player move)
				// will have an old gen and is simply dropped.
				if data.gen == lockGen {
					if !lockAndSpawn() {
						return
					}
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		draw()
	}
}
