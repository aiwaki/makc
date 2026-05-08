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

// TypingProfile describes delays between per-rune text input events.
type TypingProfile struct {
	Interval IntervalProfile
}

// InstantTyping emits text without explicit pauses.
var InstantTyping = TypingProfile{}

// FixedTyping creates a typing profile with one fixed delay between runes.
func FixedTyping(delay time.Duration) TypingProfile {
	return TypingProfile{
		Interval: FixedInterval(delay),
	}
}

// VariableTyping creates a seeded typing profile with delays in [min, max].
func VariableTyping(min, max time.Duration, seed int64) TypingProfile {
	return TypingProfile{
		Interval: VariableInterval(min, max, seed),
	}
}

// Events returns per-rune text events with optional pauses between runes.
func (p TypingProfile) Events(text string) []KeyboardEvent {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	intervals := p.Interval.Durations(len(runes) - 1)
	events := make([]KeyboardEvent, 0, len(runes)*2)
	for i, r := range runes {
		events = append(events, TextEvent(string(r)))
		if i < len(intervals) && intervals[i] > 0 {
			events = append(events, KeyboardPauseEvent(intervals[i]))
		}
	}
	return events
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

// KeyTapEventsWithHold creates key-down and key-up events separated by a hold
// pause.
func KeyTapEventsWithHold(key Key, hold time.Duration) []KeyboardEvent {
	if hold <= 0 {
		return KeyTapEvents(key)
	}
	return []KeyboardEvent{
		KeyDownEvent(key),
		KeyboardPauseEvent(hold),
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
