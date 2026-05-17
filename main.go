// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"fmt"
	"log"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/render"
)

// gravityEvent is posted by the gravity ticker on each interval.
// lockEvent carries the player index and a generation counter so stale events
// (from timers reset by a player move) can be dropped on arrival.
type gravityEvent struct{}
type lockEvent struct{ player, gen int }

const lockDelay = 500 * time.Millisecond

// Keymap maps hardware keys and runes to Actions for one player.
// Rune lookup is case-insensitive (stored lowercase); key lookup is exact.
type Keymap struct {
	keys  map[tcell.Key]game.Action
	runes map[rune]game.Action
}

func (km Keymap) Lookup(ev *tcell.EventKey) game.Action {
	if a, ok := km.keys[ev.Key()]; ok {
		return a
	}
	if ev.Key() == tcell.KeyRune {
		if a, ok := km.runes[unicode.ToLower(ev.Rune())]; ok {
			return a
		}
	}
	return game.ActionNone
}

var (
	// P1: WASD cluster — W=rotateCW, A=left, S=softDrop, D=right, E=hardDrop
	p1Keys = Keymap{
		keys: map[tcell.Key]game.Action{},
		runes: map[rune]game.Action{
			'a': game.ActionLeft,
			'd': game.ActionRight,
			's': game.ActionSoftDrop,
			'w': game.ActionRotateCW,
			'e': game.ActionHardDrop,
		},
	}

	// P2: arrow keys + space=hardDrop
	p2Keys = Keymap{
		keys: map[tcell.Key]game.Action{
			tcell.KeyLeft:  game.ActionLeft,
			tcell.KeyRight: game.ActionRight,
			tcell.KeyDown:  game.ActionSoftDrop,
			tcell.KeyUp:    game.ActionRotateCW,
		},
		runes: map[rune]game.Action{
			' ': game.ActionHardDrop,
		},
	}
)

// keyToPlayerAction returns which player (0 or 1) pressed what Action.
// Escape and Ctrl-C are a global quit regardless of player.
func keyToPlayerAction(ev *tcell.EventKey) (int, game.Action) {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return 0, game.ActionQuit
	}
	if a := p1Keys.Lookup(ev); a != game.ActionNone {
		return 0, a
	}
	if a := p2Keys.Lookup(ev); a != game.ActionNone {
		return 1, a
	}
	return 0, game.ActionNone
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
