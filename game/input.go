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
	case 'a', 'w', 's', 'd': // WASD movement/rotation
		return h.handleWASD(key)
	case 'q', 'e': // Bidirectional rotation
		return h.handleRotation(key)
	case ' ': // Space for hard drop
		return true // Hard drop always succeeds (locks piece afterwards)
	case 'w', 'A', 'D', 'Q', 'E': // Allow uppercase variants
		return h.handleWASDOrRotation(key)
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
			h.updateActiveRotationDisplay()
			return true
		}
	case 'd', 'D': // Right
		if h.state.MoveRight() {
			h.updateActiveRotationDisplay()
			return true
		}
	case 'w', 'W', 's', 'S': // Down (soft drop)
		if h.state.MoveDown() {
			h.updateActiveRotationDisplay()
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
			h.updateActiveRotationDisplay()
			return true
		}
	case 'e', 'E': // Clockwise rotation
		if h.state.RotateCW() {
			h.updateActiveRotationDisplay()
			return true
		}
	}
	return false
}

// handleWASDOrRotation handles both lowercase and uppercase variants.
func (h *InputHandler) handleWASDOrRotation(key rune) bool {
	switch key {
	case 'a', 'A': // Left
		if h.state.MoveLeft() {
			h.updateActiveRotationDisplay()
			return true
		}
	case 'd', 'D': // Right
		if h.state.MoveRight() {
			h.updateActiveRotationDisplay()
			return true
		}
	case 'w', 'W', 's', 'S': // Down (soft drop)
		if h.state.MoveDown() {
			h.updateActiveRotationDisplay()
			return true
		}
	case 'q', 'Q': // Counter-clockwise rotation
		if h.state.RotateCCW() {
			h.updateActiveRotationDisplay()
			return true
		}
	case 'e', 'E': // Clockwise rotation
		if h.state.RotateCW() {
			h.updateActiveRotationDisplay()
			return true
		}
	}
	return false
}

// updateActiveRotationDisplay updates any display information about the active piece's rotation.
func (h *InputHandler) updateActiveRotationDisplay() {
	// This can be used to update a UI or log the current rotation state.
	rotationName := [4]string{"R0", "R90", "R180", "R270"}
	h.Log("Current piece rotation: %s", rotationName[s.Active.CurrentRotation()])
}

// Log prints a message for debugging purposes (can be replaced with proper logging).
func (h *InputHandler) Log(format string, args ...interface{}) {
	// Placeholder for logging - implement as needed
}
