package render

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/oldschoolsysadmin/fightris/game"
	"github.com/oldschoolsysadmin/fightris/game/board"
	"github.com/oldschoolsysadmin/fightris/game/piece"
)

const (
	offX  = 2 // col offset from panel origin to board's left edge
	offY  = 1 // row offset from panel origin to board's top visible row
	cellW = 2 // terminal columns per board column
)

// PanelWidth is the terminal columns consumed by the board + its border.
// TotalWidth adds the side panel so the caller knows where to place a second board.
const (
	PanelWidth     = offX + game.BoardWidth*cellW + 2 // +2 for both border pipes
	SidePanelWidth = 16
	TotalWidth     = PanelWidth + SidePanelWidth
)

// pieceColors maps PieceType values (1–7) to standard Tetris Guideline colors.
// Indexed directly by board.Cell / piece.PieceType (both are uint8, values 1–7).
// An array is used instead of a switch so the lookup is a single index op, and
// adding a new piece type is one line here rather than a new case everywhere.
// Index 0 (board.Empty) is never read; ColorDefault is a safe zero value.
var pieceColors = [9]tcell.Color{
	tcell.ColorDefault, // 0 = empty, unused
	tcell.ColorAqua,    // 1 = I — cyan
	tcell.ColorYellow,  // 2 = O — yellow
	tcell.ColorFuchsia, // 3 = T — purple (Fuchsia is the closest named tcell color)
	tcell.ColorGreen,   // 4 = S — green
	tcell.ColorRed,     // 5 = Z — red
	tcell.ColorBlue,    // 6 = J — blue
	tcell.ColorOrange,  // 7 = L — orange
	tcell.ColorGray,    // 8 = garbage rows from opponent
}

// Draw renders one player's game state at (originX, originY) on the screen.
// The caller is responsible for calling s.Clear() before the first Draw and
// s.Show() after the last Draw in a frame.
func Draw(s tcell.Screen, st *game.State, originX, originY int) {
	drawBorder(s, originX, originY)
	drawBoard(s, st, originX, originY)
	drawActive(s, st, originX, originY)
	drawGhost(s, st, originX, originY)
	drawSidePanel(s, st, originX, originY)
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
			cell := st.Board.Get(row, col)
			if cell != board.Empty {
				sx, sy := boardToScreen(row, col, originX, originY)
				style := tcell.StyleDefault.Foreground(pieceColors[cell])
				drawCell(s, sx, sy, style)
			}
		}
	}
}

func drawActive(s tcell.Screen, st *game.State, originX, originY int) {
	style := tcell.StyleDefault.Foreground(pieceColors[st.Active.Type])
	for _, m := range st.Active.AbsoluteMinoes() {
		if m.Row >= game.VisibleRows {
			continue
		}
		sx, sy := boardToScreen(m.Row, m.Col, originX, originY)
		drawCell(s, sx, sy, style)
	}
}

func drawGhost(s tcell.Screen, st *game.State, originX, originY int) {
	ghostRow := st.GhostRow()
	if ghostRow == st.Active.PivotRow {
		return // piece is already at the bottom; ghost would overlap active
	}
	offsets := st.Active.AbsoluteMinoes()
	dRow := ghostRow - st.Active.PivotRow
	// Ghost stays gray regardless of piece color: it's a landing guide, not a
	// colored cell. Matching the piece color would blend with the active piece
	// and make it harder to read depth at a glance.
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

// drawSidePanel renders the right-hand panel: next-piece preview then stats.
// Layout (rows relative to originY+offY):
//
//	0: "NEXT:"
//	2: next piece (bottom row of the piece in spawn orientation)
//	1: next piece (top row, for pieces with a second row like T/J/L)
//	5: score line
//	6: level line
//	7: lines-cleared line
func drawSidePanel(s tcell.Screen, st *game.State, originX, originY int) {
	// panelX: column where the side panel begins — just past the right border.
	panelX := originX + offX + game.BoardWidth*cellW + 2
	topY := originY + offY

	// --- Next-piece preview ---
	label := "NEXT:"
	for i, ch := range label {
		s.SetContent(panelX+i, topY, ch, nil, tcell.StyleDefault)
	}

	// Render the next piece at its spawn orientation (R0).
	// The pivot is placed 2 rows below the label so the piece has vertical room.
	// Board row increases upward, but screen Y increases downward, so:
	//   screen Y = pivotY - mino.Row
	// This maps row 0 (bottom of piece) to the pivot line and row 1 to one line above.
	pivotScreenX := panelX + 3 // center horizontally; I piece (cols -1..2) fits in panelX+1..+7
	pivotScreenY := topY + 2
	style := tcell.StyleDefault.Foreground(pieceColors[st.NextPiece])
	for _, m := range piece.Minoes(st.NextPiece, piece.R0) {
		sx := pivotScreenX + m.Col*cellW
		sy := pivotScreenY - m.Row
		drawCell(s, sx, sy, style)
	}

	// --- Stats ---
	stats := []string{
		fmt.Sprintf("Score: %d", st.Score),
		fmt.Sprintf("Level: %d", st.Level),
		fmt.Sprintf("Lines: %d", st.LinesCleared),
	}
	for i, line := range stats {
		for j, ch := range line {
			s.SetContent(panelX+j, topY+5+i, ch, nil, tcell.StyleDefault)
		}
	}
}
