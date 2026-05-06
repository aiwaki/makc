package keys

import (
	"testing"

	"github.com/aiwaki/makc"
)

func TestLegacyKeyAliases(t *testing.T) {
	var key makc.Key = A
	if key != makc.KeyA {
		t.Fatalf("unexpected A key alias: %v", key)
	}
	if ControlLeft != makc.KeyLeftControl {
		t.Fatalf("unexpected left control key alias: %v", ControlLeft)
	}
	if len(Keys) == 0 {
		t.Fatal("expected legacy key list to be populated")
	}
}
