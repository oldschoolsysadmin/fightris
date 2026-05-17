package game

// Action represents a single player intent, decoupled from any input device.
type Action uint8

const (
	ActionNone Action = iota
	ActionLeft
	ActionRight
	ActionSoftDrop
	ActionHardDrop
	ActionRotateCW
	ActionQuit
)

// InputHandler applies Actions to State.
type InputHandler struct {
	state *State
}

func NewInputHandler(state *State) *InputHandler {
	return &InputHandler{state: state}
}

// Handle applies the action to the game state. Quit is handled by the caller.
func (h *InputHandler) Handle(a Action) {
	switch a {
	case ActionLeft:
		h.state.MoveLeft()
	case ActionRight:
		h.state.MoveRight()
	case ActionSoftDrop:
		h.state.MoveDown()
	case ActionHardDrop:
		h.state.HardDrop()
	case ActionRotateCW:
		h.state.RotateCW()
	}
}
