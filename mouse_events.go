package makc

import (
	"math"
	"time"
)

// WheelDelta is one standard wheel detent in Windows mouse data units.
const WheelDelta = 120

// MouseEventKind describes the operation represented by a MouseEvent.
type MouseEventKind uint8

const (
	MouseEventMove MouseEventKind = iota + 1
	MouseEventButton
	MouseEventWheel
	MouseEventHWheel
	MouseEventPause
)

// MouseEvent is one mouse operation in a batch. Pause events are interpreted by
// the Go layer and are not sent to the backend.
type MouseEvent struct {
	Kind     MouseEventKind
	Move     MouseMove
	Button   MouseButton
	State    State
	Delta    int
	Duration time.Duration
}

// MouseMoveEvent creates a movement event.
func MouseMoveEvent(move MouseMove) MouseEvent {
	return MouseEvent{
		Kind: MouseEventMove,
		Move: move,
	}
}

// MouseButtonEvent creates a button state event.
func MouseButtonEvent(button MouseButton, state State) MouseEvent {
	return MouseEvent{
		Kind:   MouseEventButton,
		Button: button,
		State:  state,
	}
}

// MouseWheelEvent creates a vertical wheel event. The delta is in raw Windows
// units; use WheelDelta for one wheel detent.
func MouseWheelEvent(delta int) MouseEvent {
	return MouseEvent{
		Kind:  MouseEventWheel,
		Delta: delta,
	}
}

// MouseHWheelEvent creates a horizontal wheel event. The delta is in raw
// Windows units; use WheelDelta for one wheel detent.
func MouseHWheelEvent(delta int) MouseEvent {
	return MouseEvent{
		Kind:  MouseEventHWheel,
		Delta: delta,
	}
}

// MousePauseEvent creates a delay between event batches.
func MousePauseEvent(duration time.Duration) MouseEvent {
	return MouseEvent{
		Kind:     MouseEventPause,
		Duration: duration,
	}
}

// MovementCurve selects the deterministic curve used by MovementProfile.
type MovementCurve uint8

const (
	MovementLinear MovementCurve = iota
	MovementEaseInOut
)

// MovementProfile describes a deterministic absolute cursor path.
type MovementProfile struct {
	Steps    int
	Duration time.Duration
	Curve    MovementCurve
}

// InstantMovement jumps to the target in one event.
var InstantMovement = MovementProfile{Steps: 1}

// LinearMovement creates a straight deterministic movement profile.
func LinearMovement(steps int, duration time.Duration) MovementProfile {
	return MovementProfile{
		Steps:    steps,
		Duration: duration,
		Curve:    MovementLinear,
	}
}

// EaseInOutMovement creates a deterministic profile that accelerates and then
// decelerates along a straight path.
func EaseInOutMovement(steps int, duration time.Duration) MovementProfile {
	return MovementProfile{
		Steps:    steps,
		Duration: duration,
		Curve:    MovementEaseInOut,
	}
}

// Events returns the movement events from start to end.
func (p MovementProfile) Events(start, end Point) []MouseEvent {
	p = p.normalized()

	events := make([]MouseEvent, 0, p.Steps*2)
	delay := time.Duration(0)
	if p.Duration > 0 && p.Steps > 1 {
		delay = p.Duration / time.Duration(p.Steps-1)
	}

	for i := 1; i <= p.Steps; i++ {
		t := float64(i) / float64(p.Steps)
		t = p.applyCurve(t)

		x := start.X + int(math.Round(float64(end.X-start.X)*t))
		y := start.Y + int(math.Round(float64(end.Y-start.Y)*t))
		events = append(events, MouseMoveEvent(Abs(x, y)))

		if delay > 0 && i < p.Steps {
			events = append(events, MousePauseEvent(delay))
		}
	}

	return events
}

func (p MovementProfile) normalized() MovementProfile {
	if p.Steps < 1 {
		p.Steps = 1
	}
	if p.Duration < 0 {
		p.Duration = 0
	}
	return p
}

func (p MovementProfile) applyCurve(t float64) float64 {
	switch p.Curve {
	case MovementEaseInOut:
		return t * t * (3 - 2*t)
	default:
		return t
	}
}
