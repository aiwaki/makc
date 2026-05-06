package makc

import "time"

// KeyboardEventKind describes the operation represented by a KeyboardEvent.
type KeyboardEventKind uint8

const (
	KeyboardEventKey KeyboardEventKind = iota + 1
	KeyboardEventScanCode
	KeyboardEventText
	KeyboardEventPause
)

// KeyboardEvent is one keyboard operation in a batch. Pause events are
// interpreted by the Go layer and are not sent to the backend.
type KeyboardEvent struct {
	Kind     KeyboardEventKind
	Key      Key
	ScanCode uint16
	State    State
	Extended bool
	Text     string
	Duration time.Duration
}

// KeyEvent creates a virtual-key event.
func KeyEvent(key Key, state State) KeyboardEvent {
	return KeyboardEvent{
		Kind:  KeyboardEventKey,
		Key:   key,
		State: state,
	}
}

// KeyDownEvent creates a virtual-key down event.
func KeyDownEvent(key Key) KeyboardEvent {
	return KeyEvent(key, Down)
}

// KeyUpEvent creates a virtual-key up event.
func KeyUpEvent(key Key) KeyboardEvent {
	return KeyEvent(key, Up)
}

// ScanCodeEvent creates a layout-independent scan-code event.
func ScanCodeEvent(scanCode uint16, state State, extended bool) KeyboardEvent {
	return KeyboardEvent{
		Kind:     KeyboardEventScanCode,
		ScanCode: scanCode,
		State:    state,
		Extended: extended,
	}
}

// TextEvent creates a Unicode text input event.
func TextEvent(text string) KeyboardEvent {
	return KeyboardEvent{
		Kind: KeyboardEventText,
		Text: text,
	}
}

// KeyboardPauseEvent creates a delay between event batches.
func KeyboardPauseEvent(duration time.Duration) KeyboardEvent {
	return KeyboardEvent{
		Kind:     KeyboardEventPause,
		Duration: duration,
	}
}

// KeyTapEvents creates key-down and key-up events.
func KeyTapEvents(key Key) []KeyboardEvent {
	return []KeyboardEvent{
		KeyDownEvent(key),
		KeyUpEvent(key),
	}
}

// ComboEvents presses keys in order and releases them in reverse order.
func ComboEvents(keys ...Key) []KeyboardEvent {
	events := make([]KeyboardEvent, 0, len(keys)*2)
	for _, key := range keys {
		events = append(events, KeyDownEvent(key))
	}
	for i := len(keys) - 1; i >= 0; i-- {
		events = append(events, KeyUpEvent(keys[i]))
	}
	return events
}
