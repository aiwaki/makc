package makc

// MouseButton identifies a physical mouse button.
type MouseButton uint8

const (
	ButtonLeft MouseButton = iota + 1
	ButtonRight
	ButtonMiddle
	ButtonX1
	ButtonX2

	// ButtonSide is kept as a readable alias for the first extended button.
	ButtonSide = ButtonX1
)

func (b MouseButton) String() string {
	switch b {
	case ButtonLeft:
		return "left"
	case ButtonRight:
		return "right"
	case ButtonMiddle:
		return "middle"
	case ButtonX1:
		return "x1"
	case ButtonX2:
		return "x2"
	default:
		return "unknown"
	}
}
