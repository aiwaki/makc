package makc

import (
	"testing"
	"time"
)

func TestComboEvents(t *testing.T) {
	events := ComboEvents(KeyControl, KeyA)
	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}

	want := []KeyboardEvent{
		KeyDownEvent(KeyControl),
		KeyDownEvent(KeyA),
		KeyUpEvent(KeyA),
		KeyUpEvent(KeyControl),
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, events[i], want[i])
		}
	}
}

func TestKeyTapEventsWithHold(t *testing.T) {
	events := KeyTapEventsWithHold(KeyEnter, 15*time.Millisecond)
	want := []KeyboardEvent{
		KeyDownEvent(KeyEnter),
		KeyboardPauseEvent(15 * time.Millisecond),
		KeyUpEvent(KeyEnter),
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, events[i], want[i])
		}
	}

	events = KeyTapEventsWithHold(KeyEnter, 0)
	want = KeyTapEvents(KeyEnter)
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, events[i], want[i])
		}
	}
}

func TestTypingProfileEvents(t *testing.T) {
	events := FixedTyping(20 * time.Millisecond).Events("aБ")
	want := []KeyboardEvent{
		TextEvent("a"),
		KeyboardPauseEvent(20 * time.Millisecond),
		TextEvent("Б"),
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("event %d = %+v, want %+v", i, events[i], want[i])
		}
	}
}

func TestVariableTypingProfileEventsAreSeeded(t *testing.T) {
	a := VariableTyping(10*time.Millisecond, 20*time.Millisecond, 42).Events("abcd")
	b := VariableTyping(10*time.Millisecond, 20*time.Millisecond, 42).Events("abcd")
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("expected same seed to produce same typing events")
		}
	}

	c := VariableTyping(10*time.Millisecond, 20*time.Millisecond, 43).Events("abcd")
	equal := len(a) == len(c)
	for i := range a {
		if a[i] != c[i] {
			equal = false
			break
		}
	}
	if equal {
		t.Fatal("expected different seeds to produce different typing events")
	}
}

func TestKeyboardEventConstructors(t *testing.T) {
	key := KeyEvent(KeyEnter, Down)
	if key.Kind != KeyboardEventKey || key.Key != KeyEnter || key.State != Down {
		t.Fatalf("KeyEvent() = %+v", key)
	}

	scan := ScanCodeEvent(0x1C, Up, true)
	if scan.Kind != KeyboardEventScanCode || scan.ScanCode != 0x1C || scan.State != Up || !scan.Extended {
		t.Fatalf("ScanCodeEvent() = %+v", scan)
	}

	text := TextEvent("hi")
	if text.Kind != KeyboardEventText || text.Text != "hi" {
		t.Fatalf("TextEvent() = %+v", text)
	}

	pause := KeyboardPauseEvent(25 * time.Millisecond)
	if pause.Kind != KeyboardEventPause || pause.Duration != 25*time.Millisecond {
		t.Fatalf("KeyboardPauseEvent() = %+v", pause)
	}
}
