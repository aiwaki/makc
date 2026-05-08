package types

import "github.com/aiwaki/makc"

// Point is an alias for makc.Point.
//
// Deprecated: use makc.Point.
type Point = makc.Point

// FPoint is kept for source compatibility with older code.
//
// Deprecated: use integer screen coordinates through makc.Point.
type FPoint struct {
	X, Y float64
}
