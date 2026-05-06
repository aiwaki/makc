package makc

import "testing"

func TestKeyString(t *testing.T) {
	tests := map[Key]string{
		KeyA:                  "a",
		KeyF12:                "f12",
		KeyLeftControl:        "controlleft",
		KeyRightSquareBracket: "]",
		KeyUnknown:            "unknown",
		Key(0xFFFF):           "unknown",
	}

	for key, want := range tests {
		if got := key.String(); got != want {
			t.Fatalf("%v.String() = %q, want %q", uint16(key), got, want)
		}
	}
}

func TestMouseButtonString(t *testing.T) {
	tests := map[MouseButton]string{
		ButtonLeft:        "left",
		ButtonRight:       "right",
		ButtonMiddle:      "middle",
		ButtonX1:          "x1",
		ButtonX2:          "x2",
		MouseButton(0xFF): "unknown",
	}

	for button, want := range tests {
		if got := button.String(); got != want {
			t.Fatalf("%v.String() = %q, want %q", uint8(button), got, want)
		}
	}
}

func TestInjectionBackendString(t *testing.T) {
	mouse := map[MouseInjectionBackend]string{
		MouseInjectionAuto:             "auto",
		MouseInjectionSendInput:        "sendinput",
		MouseInjectionInjectMouseInput: "injectmouseinput",
		MouseInjectionBackend(0xFF):    "unknown",
	}
	for backend, want := range mouse {
		if got := backend.String(); got != want {
			t.Fatalf("MouseInjectionBackend(%d).String() = %q, want %q", backend, got, want)
		}
	}

	keyboard := map[KeyboardInjectionBackend]string{
		KeyboardInjectionAuto:                "auto",
		KeyboardInjectionSendInput:           "sendinput",
		KeyboardInjectionInjectKeyboardInput: "injectkeyboardinput",
		KeyboardInjectionBackend(0xFF):       "unknown",
	}
	for backend, want := range keyboard {
		if got := backend.String(); got != want {
			t.Fatalf("KeyboardInjectionBackend(%d).String() = %q, want %q", backend, got, want)
		}
	}
}

func TestMouseMoveConstructors(t *testing.T) {
	if got := Abs(10, 20); got != (MouseMove{X: 10, Y: 20}) {
		t.Fatalf("Abs() = %+v", got)
	}
	if got := Rel(3, -4); got != (MouseMove{X: 3, Y: -4, Relative: true}) {
		t.Fatalf("Rel() = %+v", got)
	}
}
