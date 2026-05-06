// Package buttons keeps compatibility aliases for the pre-v2 API.
//
// Deprecated: import github.com/aiwaki/makc and use makc.MouseButton
// constants directly.
package buttons

import "github.com/aiwaki/makc"

// Button is an alias for makc.MouseButton.
//
// Deprecated: use makc.MouseButton.
type Button = makc.MouseButton

const (
	// Deprecated: use makc.ButtonLeft.
	Left = makc.ButtonLeft
	// Deprecated: use makc.ButtonRight.
	Right = makc.ButtonRight
	// Deprecated: use makc.ButtonMiddle.
	Middle = makc.ButtonMiddle
	// Deprecated: use makc.ButtonSide or makc.ButtonX1.
	Side = makc.ButtonSide
)

// Buttons contains the legacy button set.
//
// Deprecated: use makc.ButtonLeft, makc.ButtonRight, makc.ButtonMiddle,
// makc.ButtonX1, and makc.ButtonX2 directly.
var Buttons = []Button{
	Left,
	Right,
	Middle,
	Side,
}
