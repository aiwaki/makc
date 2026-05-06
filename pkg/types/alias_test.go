package types

import (
	"testing"

	"github.com/NeuralTeam/makc"
)

func TestLegacyTypeAliases(t *testing.T) {
	var state makc.State = Down
	if !state.Bool() {
		t.Fatal("expected Down alias to behave like makc.Down")
	}

	var point makc.Point = Point{X: 10, Y: 20}
	if point.X != 10 || point.Y != 20 {
		t.Fatalf("unexpected point alias value: %+v", point)
	}
}
