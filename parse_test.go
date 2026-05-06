package makc

import "testing"

func TestParseMouseButton(t *testing.T) {
	tests := map[string]MouseButton{
		"left":        ButtonLeft,
		"Right":       ButtonRight,
		"middle":      ButtonMiddle,
		"x1":          ButtonX1,
		"side":        ButtonX1,
		"button-side": ButtonX1,
		"x2":          ButtonX2,
	}
	for name, want := range tests {
		got, err := ParseMouseButton(name)
		if err != nil {
			t.Fatalf("ParseMouseButton(%q): %v", name, err)
		}
		if got != want {
			t.Fatalf("ParseMouseButton(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseMouseButtonUnknown(t *testing.T) {
	if _, err := ParseMouseButton("missing"); err == nil {
		t.Fatal("expected unknown mouse button to fail")
	}
}

func TestParseKey(t *testing.T) {
	tests := map[string]Key{
		"a":             KeyA,
		"0x41":          KeyA,
		"Left-Control":  KeyLeftControl,
		"ctrl":          KeyControl,
		"esc":           KeyEscape,
		"page up":       KeyPageUp,
		"question_mark": KeyQuestionMark,
		"prtsc":         KeyPrintScreen,
		"numpad5":       KeyNumpad5,
	}
	for name, want := range tests {
		got, err := ParseKey(name)
		if err != nil {
			t.Fatalf("ParseKey(%q): %v", name, err)
		}
		if got != want {
			t.Fatalf("ParseKey(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseKeyUnknown(t *testing.T) {
	for _, name := range []string{"missing", "0x0"} {
		if _, err := ParseKey(name); err == nil {
			t.Fatalf("expected unknown key %q to fail", name)
		}
	}
}
