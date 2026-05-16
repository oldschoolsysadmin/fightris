package render

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/netris/game"
	"github.com/oldschoolsysadmin/netris/game/board"
)

const (
	offX  = 2 // col offset from panel origin to board's left edge
	offY  = 1 // row offset from panel origin to board's top visible row
	cellW = 2 // terminal columns per board column
)

// PanelWidth is the total terminal columns consumed by one player panel.
const PanelWidth = offX + game.BoardWidth*cellW + 2 // +2 for both border pipes

// Draw renders one player's game state at (originX, originY) on the screen.
// The caller is responsible for calling s.Clear() before the first Draw and
// s.Show() after the last Draw in a frame.
func Draw(s tcell.Screen, st *game.State, originX, originY int) {
	drawBorder(s, originX, originY)
	drawBoard(s, st, originX, originY)
	drawActive(s, st, originX, originY)
	drawGhost(s, st, originX, originY)
	drawScore(s, st, originX, originY)
}

func boardToScreen(boardRow, boardCol, originX, originY int) (sx, sy int) {
	return originX + offX + boardCol*cellW, originY + offY + (game.VisibleRows - 1 - boardRow)
}

func drawCell(s tcell.Screen, sx, sy int, style tcell.Style) {
	s.SetContent(sx, sy, '█', nil, style)
	s.SetContent(sx+1, sy, '█', nil, style)
}

func drawBorder(s tcell.Screen, originX, originY int) {
	for row := 0; row < game.VisibleRows; row++ {
		_, sy := boardToScreen(row, 0, originX, originY)
		s.SetContent(originX+offX-1, sy, '|', nil, tcell.StyleDefault)
		s.SetContent(originX+offX+game.BoardWidth*cellW, sy, '|', nil, tcell.StyleDefault)
	}
	bottom := originY + offY + game.VisibleRows
	for col := originX + offX - 1; col <= originX+offX+game.BoardWidth*cellW; col++ {
		s.SetContent(col, bottom, '-', nil, tcell.StyleDefault)
	}
}

func drawBoard(s tcell.Screen, st *game.State, originX, originY int) {
	for row := 0; row < game.VisibleRows; row++ {
		for col := 0; col < game.BoardWidth; col++ {
			if st.Board.Get(row, col) != board.Empty {
				sx, sy := boardToScreen(row, col, originX, originY)
				drawCell(s, sx, sy, tcell.StyleDefault)
			}
		}
	}
}

func drawActive(s tcell.Screen, st *game.State, originX, originY int) {
	for _, m := range st.Active.AbsoluteMinoes() {
		if m.Row >= game.VisibleRows {
			continue
		}
		sx, sy := boardToScreen(m.Row, m.Col, originX, originY)
		drawCell(s, sx, sy, tcell.StyleDefault)
	}
}

func drawGhost(s tcell.Screen, st *game.State, originX, originY int) {
	ghostRow := st.GhostRow()
	if ghostRow == st.Active.PivotRow {
		return // piece is already at the bottom; ghost would overlap active
	}
	offsets := st.Active.AbsoluteMinoes()
	dRow := ghostRow - st.Active.PivotRow
	ghostStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	for _, m := range offsets {
		r := m.Row + dRow
		if r >= game.VisibleRows {
			continue
		}
		sx, sy := boardToScreen(r, m.Col, originX, originY)
		drawCell(s, sx, sy, ghostStyle)
	}
}

func drawScore(s tcell.Screen, st *game.State, originX, originY int) {
	str := fmt.Sprintf("Score: %d  Level: %d  Lines: %d", st.Score, st.Level, st.LinesCleared)
	sx := originX + offX + game.BoardWidth*cellW + 2
	for i, ch := range str {
		s.SetContent(sx+i, originY+offY, ch, nil, tcell.StyleDefault)
	}
}
