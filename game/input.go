// file: netris/game/input.go

package game

import (
	"github.com/oldschoolsysadmin/netris/game/piece"
)

// InputHandler manages keyboard input for the game.
type InputHandler struct {
	state *State
}

// NewInputHandler creates a new input handler attached to the given state.
func NewInputHandler(state *State) *InputHandler {
	return &InputHandler{state: state}
}

// Handle processes a single key press and updates the game state accordingly.
// Returns true if a move was made, false otherwise.
func (h *InputHandler) Handle(key rune) bool {
	switch key {
	case 'W', 'A', 'S', 'D', 'w', 'a', 's', 'd': // WASD movement/rotation
		return h.handleWASD(key)
	case 'Q', 'E', 'q', 'e': // Bidirectional rotation
		return h.handleRotation(key)
	case ' ': // Space for hard drop
		return true // Hard drop always succeeds (locks piece afterwards)
	case 27: // Escape to quit
		return false
	default:
		return false
	}
}

// handleWASD processes WASD keys for movement and rotation.
func (h *InputHandler) handleWASD(key rune) bool {
	switch key {
	case 'a', 'A': // Left
		if h.state.MoveLeft() {
			return true
		}
	case 'd', 'D': // Right
		if h.state.MoveRight() {
			return true
		}
	case 'w', 'W', 's', 'S': // Down (soft drop)
		if h.state.MoveDown() {
			return true
		}
	}
	return false
}

// handleRotation processes Q/E keys for bidirectional rotation.
func (h *InputHandler) handleRotation(key rune) bool {
	switch key {
	case 'q', 'Q': // Counter-clockwise rotation
		if h.state.RotateCCW() {
			return true
		}
	case 'e', 'E': // Clockwise rotation
		if h.state.RotateCW() {
			return true
		}
	}
	return false
}
