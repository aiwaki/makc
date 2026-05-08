// Package types keeps compatibility aliases for the legacy API.
//
// Deprecated: import github.com/aiwaki/makc and use makc.State,
// makc.Point, and related root-package types directly.
package types

import "github.com/aiwaki/makc"

// State is an alias for makc.State.
//
// Deprecated: use makc.State.
type State = makc.State

const (
	// Deprecated: use makc.Up.
	Up = makc.Up
	// Deprecated: use makc.Down.
	Down = makc.Down
)

// States contains all known state values.
//
// Deprecated: use makc.Up and makc.Down directly.
var States = []State{
	Up,
	Down,
}
