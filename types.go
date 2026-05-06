package makc

// Point is a screen coordinate in pixels.
type Point struct {
	X int
	Y int
}

// State describes whether a key or mouse button is up or down.
type State uint8

const (
	Up State = iota
	Down
)

func (s State) Bool() bool {
	return s == Down
}

func (s State) String() string {
	switch s {
	case Up:
		return "up"
	case Down:
		return "down"
	default:
		return "unknown"
	}
}

func (s State) valid() bool {
	return s == Up || s == Down
}

// MouseMove describes one mouse movement operation.
type MouseMove struct {
	X        int
	Y        int
	Relative bool
}

// Abs moves the cursor to an absolute screen coordinate.
func Abs(x, y int) MouseMove {
	return MouseMove{X: x, Y: y}
}

// Rel moves the cursor by a relative delta.
func Rel(dx, dy int) MouseMove {
	return MouseMove{X: dx, Y: dy, Relative: true}
}
