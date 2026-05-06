package makc

import (
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
