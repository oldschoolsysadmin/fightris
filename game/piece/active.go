// file: fightris/game/active.go

package piece

// Active is a piece currently in play: a type, a rotation state,
// and a position for its pivot on the board.
// PivotRow/PivotCol are in board coordinates (row 0 = bottom).
type Active struct {
	Type     PieceType
	Rot      Rotation
	PivotRow int
	PivotCol int
}

// NewActive creates a new active piece at the given pivot position.
func NewActive(pt PieceType, pivotRow, pivotCol int) Active {
	return Active{
		Type:     pt,
		Rot:      R0,
		PivotRow: pivotRow,
		PivotCol: pivotCol,
	}
}

// AbsoluteMinoes returns the board-coordinate positions of all 4 minoes,
// by adding the pivot position to each relative offset.
func (a Active) AbsoluteMinoes() [4]Mino {
	offsets := Minoes(a.Type, a.Rot)
	var result [4]Mino
	for i, o := range offsets {
		result[i] = Mino{
			Row: a.PivotRow + o.Row,
			Col: a.PivotCol + o.Col,
		}
	}
	return result
}

// Moved returns a new Active translated by (dRow, dCol), without modifying the receiver.
func (a Active) Moved(dRow, dCol int) Active {
	a.PivotRow += dRow
	a.PivotCol += dCol
	return a
}

// RotatedCW returns a new Active rotated one step clockwise, without modifying the receiver.
func (a Active) RotatedCW() Active {
	a.Rot = (a.Rot + 1) % 4
	return a
}

// RotatedCCW returns a new Active rotated one step counter-clockwise.
func (a Active) RotatedCCW() Active {
	a.Rot = (a.Rot + 3) % 4 // +3 mod 4 == -1 mod 4
	return a
}

// FromRotation / ToRotation helpers for kick lookup — just expose Rot directly,
// but these named methods make call sites at the game layer self-documenting.
func (a Active) CurrentRotation() Rotation  { return a.Rot }
