// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"fmt"
	"log"
	"os"
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
	// p1Keys: WASD cluster — W=rotateCW, A=left, S=softDrop, D=right, E=hardDrop
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

	// p2Keys: arrow keys + space=hardDrop
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

func showOverlay(s tcell.Screen, msg string) {
	w, h := s.Size()
	x := (w - len(msg)) / 2
	y := h / 2
	style := tcell.StyleDefault.Reverse(true)
	for i, ch := range msg {
		s.SetContent(x+i, y, ch, nil, style)
	}
	s.Show()
	for {
		if _, ok := s.PollEvent().(*tcell.EventKey); ok {
			return
		}
	}
}

func run1P(s tcell.Screen) {
	st := game.New()
	ih := game.NewInputHandler(st)

	var lockTimer *time.Timer
	lockGen := 0

	startLock := func() {
		lockGen++
		gen := lockGen
		if lockTimer != nil {
			lockTimer.Stop()
		}
		lockTimer = time.AfterFunc(lockDelay, func() {
			s.PostEvent(tcell.NewEventInterrupt(lockEvent{0, gen}))
		})
	}

	cancelLock := func() {
		if lockTimer != nil {
			lockTimer.Stop()
			lockTimer = nil
		}
		lockGen++
	}

	gameOver := false
	lockAndSpawn := func() bool {
		st.LockActive()
		cancelLock()
		if !st.SpawnNext() {
			gameOver = true
			return false
		}
		return true
	}

	if !st.SpawnNext() {
		return
	}

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

gameLoop:
	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return
			}
			action := p2Keys.Lookup(ev) // arrows + space
			if action == game.ActionNone || st.GameOver {
				break
			}
			ih.Handle(action)
			if action == game.ActionHardDrop {
				if !lockAndSpawn() {
					break gameLoop
				}
			} else {
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
					startLock()
				} else {
					cancelLock()
				}
			case lockEvent:
				if data.gen == lockGen {
					if !lockAndSpawn() {
						break gameLoop
					}
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		draw()
	}

	draw()
	if gameOver {
		showOverlay(s, " GAME OVER  Press any key. ")
	}
}

func run2P(s tcell.Screen) {
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
		lockGen[p]++
	}

	winner := -1

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

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			s.PostEvent(tcell.NewEventInterrupt(gravityEvent{}))
		}
	}()

	p2OriginX := render.TotalWidth + 2

	draw := func() {
		s.Clear()
		render.Draw(s, st[0], 0, 0)
		render.Draw(s, st[1], p2OriginX, 0)
		for i, ch := range "P1: WASD+E" {
			s.SetContent(i, 0, ch, nil, tcell.StyleDefault)
		}
		for i, ch := range "P2: Arrows+Space" {
			s.SetContent(p2OriginX+i, 0, ch, nil, tcell.StyleDefault)
		}
		s.Show()
	}
	draw()

gameLoop:
	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			p, action := keyToPlayerAction(ev)
			if action == game.ActionQuit {
				return
			}
			if action == game.ActionNone || st[p].GameOver {
				break
			}
			ih[p].Handle(action)
			if action == game.ActionHardDrop {
				if !lockAndSpawn(p) {
					break gameLoop
				}
			} else {
				if st[p].IsGrounded() {
					startLock(p)
				} else {
					cancelLock(p)
				}
			}
		case *tcell.EventInterrupt:
			switch data := ev.Data().(type) {
			case gravityEvent:
				for p := range st {
					if st[p].GameOver {
						continue
					}
					if !st[p].MoveDown() {
						startLock(p)
					} else {
						cancelLock(p)
					}
				}
			case lockEvent:
				if data.gen == lockGen[data.player] && !st[data.player].GameOver {
					if !lockAndSpawn(data.player) {
						break gameLoop
					}
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		draw()
	}

	draw()
	if winner >= 0 {
		showOverlay(s, fmt.Sprintf(" PLAYER %d WINS!  Press any key. ", winner+1))
	}
}

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "-1p" && os.Args[1] != "-2p") {
		fmt.Fprintln(os.Stderr, "usage: fightris -1p | -2p")
		os.Exit(1)
	}

	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := s.Init(); err != nil {
		log.Fatal(err)
	}
	defer s.Fini()

	if os.Args[1] == "-1p" {
		run1P(s)
	} else {
		run2P(s)
	}
}
