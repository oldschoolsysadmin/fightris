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

// garbageRows[n] = garbage lines sent to the opponent when n lines are cleared.
// Standard Tetris vs. table: 1→0, 2→1, 3→2, 4→4.
var garbageRows = [5]int{0, 0, 1, 2, 4}

func main() {
	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := s.Init(); err != nil {
		log.Fatal(err)
	}
	defer s.Fini()

	st := [2]*game.State{game.New(), game.New()}
	ih := [2]*game.InputHandler{game.NewInputHandler(st[0]), game.NewInputHandler(st[1])}

	var lockTimer [2]*time.Timer
	lockGen := [2]int{}

	startLock := func(p int) {
		lockGen[p]++
		gen := lockGen[p]
		if lockTimer[p] != nil {
			lockTimer[p].Stop()
		}
		lockTimer[p] = time.AfterFunc(lockDelay, func() {
			s.PostEvent(tcell.NewEventInterrupt(lockEvent{p, gen}))
		})
	}

	cancelLock := func(p int) {
		if lockTimer[p] != nil {
			lockTimer[p].Stop()
			lockTimer[p] = nil
		}
		lockGen[p]++ // invalidate any in-flight lock event for this player
	}

	winner := -1 // -1 = no winner yet; 0 or 1 = that player won

	// lockAndSpawn locks p's active piece, sends garbage to the opponent if lines
	// were cleared, then spawns p's next piece. Returns false when the game ends.
	lockAndSpawn := func(p int) bool {
		cleared := st[p].LockActive()
		cancelLock(p)
		if cleared > 0 {
			opp := 1 - p
			if g := garbageRows[min(cleared, 4)]; g > 0 && !st[opp].GameOver {
				st[opp].AddGarbage(g)
				if st[opp].GameOver {
					winner = p
					return false
				}
			}
		}
		if !st[p].SpawnNext() {
			winner = 1 - p
			return false
		}
		return true
	}

	for p := range st {
		if !st[p].SpawnNext() {
			return
		}
	}

	// One shared gravity ticker — both boards fall at the same rate.
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
