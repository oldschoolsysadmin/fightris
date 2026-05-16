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
	ActionRotateCCW
	ActionQuit
)

// InputHandler applies Actions to State.
// To add weapon plugins later: introduce a Filter func(Action) Action type and
// run the action through a []Filter pipeline here before the switch below.
type InputHandler struct {
	state *State
}

func NewInputHandler(state *State) *InputHandler {
	return &InputHandler{state: state}
}

// Handle applies the action to the game state.
// Returns false if the game should end.
func (h *InputHandler) Handle(a Action) bool {
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
	case ActionRotateCCW:
		h.state.RotateCCW()
	case ActionQuit:
		return false
	}
	return true
}
