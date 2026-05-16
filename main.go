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
	st.SpawnPiece(piece.I)
	ih := game.NewInputHandler(st)

	// Gravity: post an interrupt event on each tick so PollEvent unblocks.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			s.PostEvent(tcell.NewEventInterrupt(nil))
		}
	}()

	render.Draw(s, st)
	s.Show()

	for {
		switch ev := s.PollEvent().(type) {
		case *tcell.EventKey:
			if !ih.Handle(keyToAction(ev)) || st.GameOver {
				return
			}
		case *tcell.EventInterrupt:
			if !st.MoveDown() {
				st.LockActive()
				if !st.SpawnPiece(piece.I) {
					return
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
		render.Draw(s, st)
		s.Show()
	}
}
