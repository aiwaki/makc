package buttons

import (
	"testing"

	"github.com/aiwaki/makc"
)

func TestLegacyButtonAliases(t *testing.T) {
	var button makc.MouseButton = Left
	if button != makc.ButtonLeft {
		t.Fatalf("unexpected left button alias: %v", button)
	}
	if Side != makc.ButtonX1 {
		t.Fatalf("unexpected side button alias: %v", Side)
	}
}
