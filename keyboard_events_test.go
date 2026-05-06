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
