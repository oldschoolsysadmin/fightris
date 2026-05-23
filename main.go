// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/engine"
	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/netplay"
	"github.com/oldschoolsysadmin/fightris/render"
)

// defaultPort is the UDP port a host binds and a joiner assumes when the join
// address omits one.
const defaultPort = "4000"

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

// runLAN drives a networked match: one local authoritative game plus a render-only
// shadow of the opponent fed entirely by snapshots over conn.
//
// The structure mirrors run2P, but the "opponent" is replaced by the wire:
//   - line clears send a garbage Effect instead of calling the opponent's AddGarbage
//   - the opponent's board is a shadow State overwritten by incoming snapshots
//   - the match ends when either side's snapshot reports GameOver
func runLAN(s tcell.Screen, conn *netplay.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in runLAN: %v\n%s", r, debug.Stack())
			panic(r)
		}
	}()
	defer conn.Close()

	// Local authoritative game: driven by our inputs, gravity, lock, and any effects
	// the peer sends us. This is the ONLY State we simulate.
	eng := engine.New(s, 0)
	defer eng.Stop()
	if !eng.Start() {
		return
	}

	// Remote shadow: a render-only copy of the peer's board, overwritten wholesale by
	// each snapshot and never simulated. game.New gives us a valid empty State to
	// overwrite (its bag goes unused).
	remote := game.New()

	conn.Run(s) // start the receive goroutine now that the screen exists

	p2OriginX := render.TotalWidth + 2
	draw := func() {
		s.Clear()
		render.Draw(s, eng.State, 0, 0)
		render.Draw(s, remote, p2OriginX, 0)
		for i, ch := range "YOU (arrows+space)" {
			s.SetContent(i, 0, ch, nil, tcell.StyleDefault)
		}
		for i, ch := range "OPPONENT" {
			s.SetContent(p2OriginX+i, 0, ch, nil, tcell.StyleDefault)
		}
		s.Show()
	}

	// Send-on-change: after anything that alters our board, snapshot it to the peer.
	// The data is tiny, so favoring responsiveness over a fixed tick rate is fine.
	pushLocal := func() {
		conn.SendSnapshot(eng.State)
		draw()
	}
	pushLocal() // initial state so the peer can render us immediately

	winner := -1 // 0 = local player wins, 1 = remote player wins

gameLoop:
	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				log.Printf("[lan] quit via key")
				return
			}
			action := p2Keys.Lookup(ev) // the local human plays on arrows+space
			if action == game.ActionNone || eng.State.GameOver {
				break
			}
			log.Printf("[lan] local input: %v", action)
			cleared, alive := eng.HandleAction(action)
			if cleared > 0 {
				conn.SendGarbage(garbageRows[min(cleared, 4)])
			}
			if !alive {
				winner = 1
				log.Printf("[lan] local topped out (action), remote wins")
				break gameLoop
			}
			pushLocal()
		case *tcell.EventInterrupt:
			switch data := ev.Data().(type) {
			case engine.GravityEvent:
				if !eng.State.GameOver {
					eng.HandleGravity()
					pushLocal()
				}
			case engine.LockEvent:
				if !eng.State.GameOver {
					cleared, alive := eng.HandleLock(data.Gen)
					if cleared > 0 {
						conn.SendGarbage(garbageRows[min(cleared, 4)])
					}
					if !alive {
						winner = 1
						log.Printf("[lan] local topped out (lock), remote wins")
						break gameLoop
					}
					pushLocal()
				}
			case netplay.Incoming:
				w, localChanged := applyIncoming(eng, remote, data.Packet)
				if w >= 0 {
					winner = w
					break gameLoop
				}
				if localChanged {
					pushLocal() // garbage landed on us; let the peer see the result
				} else {
					draw()
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
	}

	log.Printf("[lan] match over, winner=%s", map[int]string{0: "local", 1: "remote"}[winner])
	// Re-broadcast our final state several times so the peer reliably learns the
	// outcome even on a lossy link (each call also obeys the --spam multiplier).
	for i := 0; i < 5; i++ {
		conn.SendSnapshot(eng.State)
	}
	draw()
	if winner == 0 {
		showOverlay(s, " YOU WIN!  Press any key. ")
	} else if winner == 1 {
		showOverlay(s, " YOU LOSE.  Press any key. ")
	}
}

// applyIncoming applies a deduped peer packet to local/remote state. It returns the
// winner index (0 local, 1 remote) if the packet ends the match, else -1; and
// whether OUR board changed (so the caller knows to snapshot it back to the peer).
func applyIncoming(eng *engine.GameEngine, remote *game.State, pkt *netplay.Packet) (winner int, localChanged bool) {
	switch pkt.Kind {
	case netplay.KindSnapshot:
		if pkt.Snapshot == nil {
			return -1, false
		}
		pkt.Snapshot.ApplyTo(remote)
		if remote.GameOver {
			log.Printf("[lan] remote reports game over, local wins")
			return 0, false
		}
	case netplay.KindEffect:
		if pkt.Effect == nil {
			return -1, false
		}
		switch pkt.Effect.Kind {
		case netplay.EffectGarbage:
			var g netplay.GarbageEffect
			if err := json.Unmarshal(pkt.Effect.Data, &g); err != nil {
				log.Printf("[lan] bad garbage payload: %v", err)
				return -1, false
			}
			log.Printf("[lan] received garbage rows=%d id=%d", g.Rows, pkt.Effect.ID)
			if !eng.State.AddGarbage(g.Rows) {
				log.Printf("[lan] garbage topped us out, remote wins")
				return 1, true
			}
			return -1, true
		default:
			// Future powerup effects dispatch here. Unknown kinds are ignored so an
			// older client stays compatible with a newer peer's extra effects.
			log.Printf("[lan] unhandled effect kind %q", pkt.Effect.Kind)
		}
	}
	return -1, false
}

func main() {
	oneP := flag.Bool("1p", false, "single player")
	twoP := flag.Bool("2p", false, "two players sharing one terminal (WASD+E vs arrows+space)")
	host := flag.Bool("host", false, "host a LAN match and wait for a joiner")
	join := flag.String("join", "", "join a LAN match at host address, e.g. 192.168.1.5:4000")
	port := flag.String("port", defaultPort, "UDP port to host on (with -host)")
	spam := flag.Int("spam", 1, "send N redundant copies of each packet (raise on lossy links)")
	flag.Parse()

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
	log.Printf("fightris starting pid=%d", os.Getpid())

	// Establish the network link BEFORE tcell takes over the terminal, so the
	// blocking handshake can print plain status the player can read.
	var conn *netplay.Conn
	switch {
	case *host:
		fmt.Printf("Hosting on :%s — waiting for opponent to join...\n", *port)
		conn, err = netplay.Listen(":"+*port, *spam)
	case *join != "":
		addr := withDefaultPort(*join)
		fmt.Printf("Joining %s...\n", addr)
		conn, err = netplay.Dial(addr, *spam)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "network error: %v\n", err)
		log.Fatalf("network error: %v", err)
	}

	s, err := tcell.NewScreen()
	if err != nil {
		log.Fatal(err)
	}
	if err := s.Init(); err != nil {
		log.Fatal(err)
	}
	defer s.Fini()
	log.Printf("tcell screen init OK")

	switch {
	case conn != nil:
		runLAN(s, conn)
	case *oneP:
		run1P(s)
	case *twoP:
		run2P(s)
	default:
		s.Fini()
		fmt.Fprintln(os.Stderr, "usage: fightris -1p | -2p | -host [-port N] | -join <addr> [-spam N]")
		os.Exit(1)
	}

	log.Printf("fightris exiting cleanly")
}

// withDefaultPort appends the default UDP port if addr has none (e.g. "192.168.1.5"
// becomes "192.168.1.5:4000"). Bare IPv6 literals must already include a port.
func withDefaultPort(addr string) string {
	if strings.Contains(addr, ":") {
		return addr
	}
	return addr + ":" + defaultPort
}
