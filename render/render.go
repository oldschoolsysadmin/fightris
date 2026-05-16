package render

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/netris/game"
	"github.com/oldschoolsysadmin/netris/game/board"
)

const (
	offX  = 2 // screen col of board's left edge
	offY  = 1 // screen row of board's top visible row
	cellW = 2 // terminal columns per board column
)

// Draw renders the full game state to the screen.
func Draw(s tcell.Screen, st *game.State) {
	s.Clear()
	drawBorder(s)
	drawBoard(s, st)
	drawActive(s, st)
	drawScore(s, st)
}

func boardToScreen(boardRow, boardCol int) (sx, sy int) {
	return offX + boardCol*cellW, offY + (game.VisibleRows - 1 - boardRow)
}

func cell(s tcell.Screen, sx, sy int) {
	s.SetContent(sx, sy, '█', nil, tcell.StyleDefault)
	s.SetContent(sx+1, sy, '█', nil, tcell.StyleDefault)
}

func drawBorder(s tcell.Screen) {
	for row := 0; row < game.VisibleRows; row++ {
		_, sy := boardToScreen(row, 0)
		s.SetContent(offX-1, sy, '|', nil, tcell.StyleDefault)
		s.SetContent(offX+game.BoardWidth*cellW, sy, '|', nil, tcell.StyleDefault)
	}
	bottom := offY + game.VisibleRows
	for col := offX - 1; col <= offX+game.BoardWidth*cellW; col++ {
		s.SetContent(col, bottom, '-', nil, tcell.StyleDefault)
	}
}

func drawBoard(s tcell.Screen, st *game.State) {
	for row := 0; row < game.VisibleRows; row++ {
		for col := 0; col < game.BoardWidth; col++ {
			if st.Board.Get(row, col) != board.Empty {
				sx, sy := boardToScreen(row, col)
				cell(s, sx, sy)
			}
		}
	}
}

func drawActive(s tcell.Screen, st *game.State) {
	for _, m := range st.Active.AbsoluteMinoes() {
		if m.Row >= game.VisibleRows {
			continue
		}
		sx, sy := boardToScreen(m.Row, m.Col)
		cell(s, sx, sy)
	}
}

func drawScore(s tcell.Screen, st *game.State) {
	str := fmt.Sprintf("Score: %d  Level: %d  Lines: %d", st.Score, st.Level, st.LinesCleared)
	sx := offX + game.BoardWidth*cellW + 2
	for i, ch := range str {
		s.SetContent(sx+i, offY, ch, nil, tcell.StyleDefault)
	}
}
