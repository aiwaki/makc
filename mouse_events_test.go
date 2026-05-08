package makc

import (
	"reflect"
	"testing"
	"time"
)

func TestMovementProfileEvents(t *testing.T) {
	events := LinearMovement(3, 30*time.Millisecond).Events(
		Point{X: 0, Y: 0},
		Point{X: 9, Y: 6},
	)

	if len(events) != 5 {
		t.Fatalf("len(events) = %d, want 5", len(events))
	}
	wantMoves := []Point{
		{X: 3, Y: 2},
		{X: 6, Y: 4},
		{X: 9, Y: 6},
	}

	moveIndex := 0
	for _, event := range events {
		switch event.Kind {
		case MouseEventMove:
			want := wantMoves[moveIndex]
			if event.Move.Relative {
				t.Fatalf("event %d is relative", moveIndex)
			}
			if event.Move.X != want.X || event.Move.Y != want.Y {
				t.Fatalf("move %d = %d,%d, want %d,%d", moveIndex, event.Move.X, event.Move.Y, want.X, want.Y)
			}
			moveIndex++
		case MouseEventPause:
			if event.Duration != 15*time.Millisecond {
				t.Fatalf("pause = %s, want 15ms", event.Duration)
			}
		default:
			t.Fatalf("unexpected event kind %d", event.Kind)
		}
	}
}

func TestNaturalMovementProfileEventsAreSeeded(t *testing.T) {
	start := Point{X: 0, Y: 0}
	end := Point{X: 400, Y: 120}

	a := NaturalMovement(12, 90*time.Millisecond, 42).Events(start, end)
	b := NaturalMovement(12, 90*time.Millisecond, 42).Events(start, end)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("expected same seed to produce the same movement events")
	}

	c := NaturalMovement(12, 90*time.Millisecond, 43).Events(start, end)
	if reflect.DeepEqual(a, c) {
		t.Fatal("expected different seeds to produce different movement events")
	}
}

func TestNaturalMovementProfileEndsAtTargetAndKeepsDuration(t *testing.T) {
	start := Point{X: 10, Y: 20}
	end := Point{X: 210, Y: 80}
	duration := 75 * time.Millisecond

	events := NaturalMovementWithJitter(8, duration, 12, 7).Events(start, end)
	if len(events) != 15 {
		t.Fatalf("len(events) = %d, want 15", len(events))
	}

	var lastMove MouseMove
	var totalPause time.Duration
	moveCount := 0
	for _, event := range events {
		switch event.Kind {
		case MouseEventMove:
			if event.Move.Relative {
				t.Fatalf("natural movement emitted relative move: %+v", event)
			}
			lastMove = event.Move
			moveCount++
		case MouseEventPause:
			totalPause += event.Duration
		default:
			t.Fatalf("unexpected event kind %d", event.Kind)
		}
	}

	if moveCount != 8 {
		t.Fatalf("move count = %d, want 8", moveCount)
	}
	if got := (Point{X: lastMove.X, Y: lastMove.Y}); got != end {
		t.Fatalf("last move = %+v, want %+v", got, end)
	}
	if totalPause != duration {
		t.Fatalf("total pause = %s, want %s", totalPause, duration)
	}
}

func TestClickProfileEvents(t *testing.T) {
	events := DoubleClick(10*time.Millisecond, 80*time.Millisecond).Events(ButtonLeft)
	want := []MouseEvent{
		MouseButtonEvent(ButtonLeft, Down),
		MousePauseEvent(10 * time.Millisecond),
		MouseButtonEvent(ButtonLeft, Up),
		MousePauseEvent(80 * time.Millisecond),
		MouseButtonEvent(ButtonLeft, Down),
		MousePauseEvent(10 * time.Millisecond),
		MouseButtonEvent(ButtonLeft, Up),
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %+v, want %+v", events, want)
	}
}

func TestClickProfileNormalizesCountAndHold(t *testing.T) {
	events := MultiClick(0, -10*time.Millisecond, FixedInterval(25*time.Millisecond)).Events(ButtonRight)
	want := []MouseEvent{
		MouseButtonEvent(ButtonRight, Down),
		MouseButtonEvent(ButtonRight, Up),
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %+v, want %+v", events, want)
	}
}

func TestMouseEventConstructors(t *testing.T) {
	move := MouseMoveEvent(Rel(1, -2))
	if move.Kind != MouseEventMove || !move.Move.Relative || move.Move.X != 1 || move.Move.Y != -2 {
		t.Fatalf("MouseMoveEvent() = %+v", move)
	}

	button := MouseButtonEvent(ButtonX2, Down)
	if button.Kind != MouseEventButton || button.Button != ButtonX2 || button.State != Down {
		t.Fatalf("MouseButtonEvent() = %+v", button)
	}

	wheel := MouseWheelEvent(-WheelDelta)
	if wheel.Kind != MouseEventWheel || wheel.Delta != -WheelDelta {
		t.Fatalf("MouseWheelEvent() = %+v", wheel)
	}

	hwheel := MouseHWheelEvent(WheelDelta)
	if hwheel.Kind != MouseEventHWheel || hwheel.Delta != WheelDelta {
		t.Fatalf("MouseHWheelEvent() = %+v", hwheel)
	}
}
