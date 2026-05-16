// Copyright 2023 Alex
// Licensed under the MIT License

package main

import (
	"log"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/netris/game"
	"github.com/oldschoolsysadmin/netris/game/piece"
	"github.com/oldschoolsysadmin/netris/render"
)

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

	// lockAndSpawn is the single path for locking a placed piece and spawning
	// the next one. Both gravity (EventInterrupt) and hard drop converge here
	// so lock-event hooks fire exactly once per piece.
	lockAndSpawn := func() bool {
		st.LockActive()
		return st.SpawnPiece(piece.I) // TODO: replace piece.I with bag randomizer
	}

	if !st.SpawnPiece(piece.I) {
		return
	}

	// Gravity: post an interrupt event on each tick so PollEvent unblocks.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			s.PostEvent(tcell.NewEventInterrupt(nil))
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
				if !lockAndSpawn() {
					return
				}
			}
		case *tcell.EventInterrupt:
			if !st.MoveDown() {
				if !lockAndSpawn() {
					return
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		draw()
	}
}
