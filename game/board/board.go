// file: fightris/game/board.go

package board

// Cell represents a single grid cell.
// Empty = 0; filled cells store the PieceType value (1–7) so we retain color info.
type Cell uint8

const (
	Empty Cell = 0
)

// Board is row 0 = bottom, row (Height-1) = top.
// Columns run left (0) to right (Width-1).
type Board struct {
	Width  int
	Height int
	cells  [][]Cell // cells[row][col]
}

func New(width, height int) *Board {
	cells := make([][]Cell, height)
	for r := range cells {
		cells[r] = make([]Cell, width)
	}
	return &Board{Width: width, Height: height, cells: cells}
}

// Get returns the cell value at (row, col). Row 0 is the bottom.
// Returns Empty for out-of-bounds — callers can use this for boundary checks.
func (b *Board) Get(row, col int) Cell {
	if row < 0 || row >= b.Height || col < 0 || col >= b.Width {
		return Empty
	}
	return b.cells[row][col]
}

// Set writes a cell value. Out-of-bounds writes are silently ignored.
func (b *Board) Set(row, col int, c Cell) {
	if row < 0 || row >= b.Height || col < 0 || col >= b.Width {
		return
	}
	b.cells[row][col] = c
}

// InBounds returns true if (row, col) is a valid cell coordinate.
func (b *Board) InBounds(row, col int) bool {
	return row >= 0 && row < b.Height && col >= 0 && col < b.Width
}

// IsRowFull returns true if every cell in the given row is non-empty.
func (b *Board) IsRowFull(row int) bool {
	for col := 0; col < b.Width; col++ {
		if b.cells[row][col] == Empty {
			return false
		}
	}
	return true
}

// ClearFullRows removes all full rows and drops everything above down.
// Returns the number of rows cleared (used for scoring).
func (b *Board) ClearFullRows() int {
	cleared := 0
	writeRow := 0 // next row to write into (from bottom)

	for readRow := 0; readRow < b.Height; readRow++ {
		if b.IsRowFull(readRow) {
			cleared++
			continue // skip this row — don't copy it
		}
		if writeRow != readRow {
			copy(b.cells[writeRow], b.cells[readRow])
		}
		writeRow++
	}

	// Zero out the rows above the written content
	for r := writeRow; r < b.Height; r++ {
		for col := range b.cells[r] {
			b.cells[r][col] = Empty
		}
	}

	return cleared
}

// IsEmpty returns true if every cell on the board is empty.
func (b *Board) IsEmpty() bool {
	for r := 0; r < b.Height; r++ {
		for c := 0; c < b.Width; c++ {
			if b.cells[r][c] != Empty {
				return false
			}
		}
	}
	return true
}
