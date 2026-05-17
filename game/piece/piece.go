// file: fightris/game/piece.go

package piece

// PieceType identifies which of the 7 standard tetrominoes this is.
// The numeric value (1–7) doubles as the Cell fill value on the board,
// preserving color/identity after a piece locks.
type PieceType uint8

const (
	I PieceType = iota + 1 // 1
	O                      // 2
	T                      // 3
	S                      // 4
	Z                      // 5
	J                      // 6
	L                      // 7
)

// AllTypes lists every standard tetromino in spawn order.
var AllTypes = [7]PieceType{I, O, T, S, Z, J, L}

// Rotation is one of the four SRS rotation states.
type Rotation int

const (
	R0  Rotation = iota // spawn orientation
	R90                 // one clockwise turn
	R180
	R270
)

// Mino is a single square offset within a piece, relative to its pivot.
// The pivot is the rotation center; offsets use board coordinates (row up, col right).
type Mino struct {
	Row, Col int
}

// rotationTable maps each PieceType to its four rotation states.
// Each state is a list of 4 Mino offsets from the pivot point.
// These match the standard Tetris Guideline SRS definitions.
//
// Coordinate convention: Row increases upward, Col increases rightward.
// Pivot is at (0,0).  Offsets chosen so the piece spawns centered.
var rotationTable = map[PieceType][4][]Mino{
	// I: spans 4 columns; pivot is between cells 1 and 2
	I: {
		{{0, -1}, {0, 0}, {0, 1}, {0, 2}},  // R0  — horizontal
		{{-1, 1}, {0, 1}, {1, 1}, {2, 1}},  // R90 — vertical, right of pivot col
		{{-1, -1}, {-1, 0}, {-1, 1}, {-1, 2}}, // R180
		{{-1, 0}, {0, 0}, {1, 0}, {2, 0}},  // R270
	},
	// O: 2×2; no effective rotation (all states identical)
	O: {
		{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
	},
	T: {
		{{0, -1}, {0, 0}, {0, 1}, {1, 0}},  // R0
		{{-1, 0}, {0, 0}, {0, 1}, {1, 0}},  // R90
		{{-1, 0}, {0, -1}, {0, 0}, {0, 1}}, // R180
		{{-1, 0}, {0, -1}, {0, 0}, {1, 0}}, // R270
	},
	S: {
		{{0, -1}, {0, 0}, {1, 0}, {1, 1}},
		{{-1, 0}, {0, 0}, {0, 1}, {1, 1}},  // R90 — actually unused in classic but kept for symmetry
		{{-1, -1}, {-1, 0}, {0, 0}, {0, 1}},
		{{-1, -1}, {0, -1}, {0, 0}, {1, 0}},
	},
	Z: {
		{{0, 0}, {0, 1}, {1, -1}, {1, 0}},
		{{-1, 0}, {0, 0}, {0, 1}, {1, 1}},
		{{-1, 0}, {-1, 1}, {0, -1}, {0, 0}},
		{{-1, -1}, {0, -1}, {0, 0}, {1, 0}},
	},
	J: {
		{{0, -1}, {0, 0}, {0, 1}, {1, -1}}, // R0  — flat with left stub up
		{{-1, 0}, {0, 0}, {1, 0}, {1, 1}},  // R90
		{{-1, 1}, {0, -1}, {0, 0}, {0, 1}}, // R180
		{{-1, -1}, {-1, 0}, {0, 0}, {1, 0}},// R270
	},
	L: {
		{{0, -1}, {0, 0}, {0, 1}, {1, 1}},   // R0   — bar + top-right foot
		{{-1, 0}, {0, 0}, {1, 0}, {1, -1}},  // R90  — column + top-left foot
		{{-1, -1}, {0, -1}, {0, 0}, {0, 1}}, // R180 — bar + bottom-left foot
		{{-1, 1}, {-1, 0}, {0, 0}, {1, 0}},  // R270 — column + bottom-right foot
	},
}

// Miinos returns the 4 mino offsets for a given type and rotation.
func Minoes(pt PieceType, r Rotation) []Mino {
	return rotationTable[pt][r]
}

// -- SRS Wall Kick Data --------------------------------------------------
//
// When a rotation is blocked, the engine tries a sequence of (row, col)
// offsets in order; the first one that fits is used.  Failing all means
// the rotation is denied.
//
// kickData is indexed as kickData[pieceType][fromRotation][toRotation].
// "J" kicks apply to J, L, T, S, Z.  "I" has its own set.  O never kicks.

type KickKey struct {
	From, To Rotation
}

var jlstzKicks = map[KickKey][]Mino{
	{R0, R90}:   {{0, 0}, {0, -1}, {1, -1}, {-2, 0}, {-2, -1}},
	{R90, R0}:   {{0, 0}, {0, 1}, {-1, 1}, {2, 0}, {2, 1}},
	{R90, R180}: {{0, 0}, {0, 1}, {-1, 1}, {2, 0}, {2, 1}},
	{R180, R90}: {{0, 0}, {0, -1}, {1, -1}, {-2, 0}, {-2, -1}},
	{R180, R270}:{{0, 0}, {0, 1}, {1, 1}, {-2, 0}, {-2, 1}},
	{R270, R180}:{{0, 0}, {0, -1}, {-1, -1}, {2, 0}, {2, -1}},
	{R270, R0}:  {{0, 0}, {0, -1}, {-1, -1}, {2, 0}, {2, -1}},
	{R0, R270}:  {{0, 0}, {0, 1}, {1, 1}, {-2, 0}, {-2, 1}},
}

var iKicks = map[KickKey][]Mino{
	{R0, R90}:   {{0, 0}, {0, -2}, {0, 1}, {-1, -2}, {2, 1}},
	{R90, R0}:   {{0, 0}, {0, 2}, {0, -1}, {1, 2}, {-2, -1}},
	{R90, R180}: {{0, 0}, {0, -1}, {0, 2}, {2, -1}, {-1, 2}},
	{R180, R90}: {{0, 0}, {0, 1}, {0, -2}, {-2, 1}, {1, -2}},
	{R180, R270}:{{0, 0}, {0, 2}, {0, -1}, {1, 2}, {-2, -1}},
	{R270, R180}:{{0, 0}, {0, -2}, {0, 1}, {-1, -2}, {2, 1}},
	{R270, R0}:  {{0, 0}, {0, 1}, {0, -2}, {-2, 1}, {1, -2}},
	{R0, R270}:  {{0, 0}, {0, -1}, {0, 2}, {2, -1}, {-1, 2}},
}

// KickOffsets returns the SRS kick candidates for a given piece type and
// rotation transition, in the order they should be tried.
func KickOffsets(pt PieceType, from, to Rotation) []Mino {
	key := KickKey{from, to}
	switch pt {
	case I:
		return iKicks[key]
	case O:
		return []Mino{{0, 0}} // O never kicks; single no-op offset
	default:
		return jlstzKicks[key]
	}
}
