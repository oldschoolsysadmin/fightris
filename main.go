// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/engine"
	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/render"
)

// garbageRows[n] = garbage lines sent to the opponent when n lines are cleared.
// Standard Tetris vs. table: 1→0, 2→1, 3→2, 4→4.
var garbageRows = [5]int{0, 0, 1, 2, 4}

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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in run1P: %v\n%s", r, debug.Stack())
			panic(r) // re-panic so the runtime prints to stderr and exits non-zero
		}
	}()

	eng := engine.New(s, 0)
	defer eng.Stop()
	if !eng.Start() {
		return
	}

	draw := func() {
		s.Clear()
		render.Draw(s, eng.State, 0, 0)
		s.Show()
	}
	draw()

gameLoop:
	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				log.Printf("[1P] quit via key")
				return
			}
			action := p2Keys.Lookup(ev) // arrows + space
			if action == game.ActionNone || eng.State.GameOver {
				break
			}
			if _, alive := eng.HandleAction(action); !alive {
				break gameLoop
			}
		case *tcell.EventInterrupt:
			switch data := ev.Data().(type) {
			case engine.GravityEvent:
				eng.HandleGravity()
			case engine.LockEvent:
				if _, alive := eng.HandleLock(data.Gen); !alive {
					break gameLoop
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		draw()
	}

	log.Printf("[1P] game over")
	draw()
	showOverlay(s, " GAME OVER  Press any key. ")
}

func run2P(s tcell.Screen) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in run2P: %v\n%s", r, debug.Stack())
			panic(r)
		}
	}()

	eng := [2]*engine.GameEngine{
		engine.New(s, 0),
		engine.New(s, 1),
	}
	defer eng[0].Stop()
	defer eng[1].Stop()

	for p := range eng {
		if !eng[p].Start() {
			return
		}
	}

	p2OriginX := render.TotalWidth + 2

	draw := func() {
		s.Clear()
		render.Draw(s, eng[0].State, 0, 0)
		render.Draw(s, eng[1].State, p2OriginX, 0)
		for i, ch := range "P1: WASD+E" {
			s.SetContent(i, 0, ch, nil, tcell.StyleDefault)
		}
		for i, ch := range "P2: Arrows+Space" {
			s.SetContent(p2OriginX+i, 0, ch, nil, tcell.StyleDefault)
		}
		s.Show()
	}
	draw()

	winner := -1

	// routeGarbage sends garbage rows to the opponent when a player clears lines.
	// Returns false (and sets winner) if the garbage kills the opponent.
	// Keeping this in main rather than inside the engine is intentional: for LAN,
	// garbage crosses a network boundary instead of calling AddGarbage directly.
	routeGarbage := func(scorer, cleared int) bool {
		opp := 1 - scorer
		g := garbageRows[min(cleared, 4)]
		log.Printf("[2P] P%d cleared %d lines → %d garbage to P%d", scorer+1, cleared, g, opp+1)
		if g > 0 && !eng[opp].State.GameOver {
			ok := eng[opp].State.AddGarbage(g)
			log.Printf("[2P] AddGarbage(%d) on P%d: ok=%v", g, opp+1, ok)
			if !ok {
				winner = scorer
				return false
			}
		}
		return true
	}

gameLoop:
	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			p, action := keyToPlayerAction(ev)
			if action == game.ActionQuit {
				log.Printf("[2P] quit via key")
				return
			}
			if action == game.ActionNone || eng[p].State.GameOver {
				break
			}
			log.Printf("[2P] P%d input: %v", p+1, action)
			cleared, alive := eng[p].HandleAction(action)
			if cleared > 0 && !routeGarbage(p, cleared) {
				break gameLoop
			}
			if !alive {
				winner = 1 - p
				log.Printf("[2P] P%d topped out (action), winner=P%d", p+1, winner+1)
				break gameLoop
			}
		case *tcell.EventInterrupt:
			switch data := ev.Data().(type) {
			case engine.GravityEvent:
				p := data.Player
				if !eng[p].State.GameOver {
					eng[p].HandleGravity()
				}
			case engine.LockEvent:
				p := data.Player
				if !eng[p].State.GameOver {
					cleared, alive := eng[p].HandleLock(data.Gen)
					if cleared > 0 && !routeGarbage(p, cleared) {
						break gameLoop
					}
					if !alive {
						winner = 1 - p
						log.Printf("[2P] P%d topped out (lock), winner=P%d", p+1, winner+1)
						break gameLoop
					}
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		draw()
	}

	log.Printf("[2P] game over, winner=%d", winner+1)
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

	// tcell owns the terminal — we can't write to stderr while the TUI is active.
	// Redirect log to a file before s.Init() so all log.Printf calls land there
	// instead of corrupting the display. The file survives even if the process
	// panics before tcell can restore the terminal.
	logFile, err := os.Create("fightris.log")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()
	log.SetOutput(logFile)
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("fightris starting: mode=%s pid=%d", os.Args[1], os.Getpid())

	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := s.Init(); err != nil {
		log.Fatal(err)
	}
	defer s.Fini()

	log.Printf("tcell screen init OK")

	if os.Args[1] == "-1p" {
		run1P(s)
	} else {
		run2P(s)
	}

	log.Printf("fightris exiting cleanly")
}
