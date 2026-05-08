package makc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestInputStepConstructors(t *testing.T) {
	move := MoveStep(Rel(3, -2))
	if move.Kind != InputStepMouse || len(move.Mouse) != 1 || move.Mouse[0] != MouseMoveEvent(Rel(3, -2)) {
		t.Fatalf("MoveStep() = %+v", move)
	}

	click := ClickStep(ButtonLeft, ClickWithHold(10*time.Millisecond))
	wantClick := []MouseEvent{
		MouseButtonEvent(ButtonLeft, Down),
		MousePauseEvent(10 * time.Millisecond),
		MouseButtonEvent(ButtonLeft, Up),
	}
	if click.Kind != InputStepMouse || !reflect.DeepEqual(click.Mouse, wantClick) {
		t.Fatalf("ClickStep() = %+v, want %+v", click, wantClick)
	}

	doubleClick := DoubleClickStep(ButtonLeft, 10*time.Millisecond, 80*time.Millisecond)
	if doubleClick.Kind != InputStepMouse || len(doubleClick.Mouse) != 7 {
		t.Fatalf("DoubleClickStep() = %+v", doubleClick)
	}

	keyTap := KeyTapStep(KeyEnter, 5*time.Millisecond)
	wantKeyTap := []KeyboardEvent{
		KeyDownEvent(KeyEnter),
		KeyboardPauseEvent(5 * time.Millisecond),
		KeyUpEvent(KeyEnter),
	}
	if keyTap.Kind != InputStepKeyboard || !reflect.DeepEqual(keyTap.Keyboard, wantKeyTap) {
		t.Fatalf("KeyTapStep() = %+v, want %+v", keyTap, wantKeyTap)
	}

	combo := ComboStep(KeyControl, KeyA)
	if combo.Kind != InputStepKeyboard || !reflect.DeepEqual(combo.Keyboard, ComboEvents(KeyControl, KeyA)) {
		t.Fatalf("ComboStep() = %+v", combo)
	}

	text := TextStep("ab", FixedTyping(10*time.Millisecond))
	wantText := []KeyboardEvent{
		TextEvent("a"),
		KeyboardPauseEvent(10 * time.Millisecond),
		TextEvent("b"),
	}
	if text.Kind != InputStepKeyboard || !reflect.DeepEqual(text.Keyboard, wantText) {
		t.Fatalf("TextStep() = %+v, want %+v", text, wantText)
	}
}

func TestInputSequenceAppendCopiesSteps(t *testing.T) {
	mouseEvents := []MouseEvent{MouseMoveEvent(Rel(1, 1))}
	keyboardEvents := []KeyboardEvent{KeyDownEvent(KeyA)}

	sequence := NewInputSequence(MouseStep(mouseEvents...)).
		Append(KeyboardStep(keyboardEvents...)).
		WithPause(10 * time.Millisecond)

	mouseEvents[0] = MouseMoveEvent(Rel(9, 9))
	keyboardEvents[0] = KeyDownEvent(KeyB)

	if len(sequence.Steps) != 3 {
		t.Fatalf("len(sequence.Steps) = %d, want 3", len(sequence.Steps))
	}
	if got := sequence.Steps[0].Mouse[0]; got != MouseMoveEvent(Rel(1, 1)) {
		t.Fatalf("mouse event = %+v, want original value", got)
	}
	if got := sequence.Steps[1].Keyboard[0]; got != KeyDownEvent(KeyA) {
		t.Fatalf("keyboard event = %+v, want original value", got)
	}
	if got := sequence.Steps[2]; got.Kind != InputStepPause || got.Duration != 10*time.Millisecond {
		t.Fatalf("pause step = %+v", got)
	}
}

func TestClientRunInputSequence(t *testing.T) {
	backend := &sequenceTestBackend{}
	client := newSequenceTestClient(backend)

	sequence := NewInputSequence(
		MouseStep(MouseMoveEvent(Rel(1, 2))),
		PauseStep(0),
		KeyboardStep(KeyDownEvent(KeyA), KeyUpEvent(KeyA)),
	)

	if err := client.Run(context.Background(), sequence); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !reflect.DeepEqual(backend.order, []string{"mouse", "keyboard"}) {
		t.Fatalf("order = %v, want mouse then keyboard", backend.order)
	}
	if got := backend.mouseEvents; !reflect.DeepEqual(got, [][]MouseEvent{{MouseMoveEvent(Rel(1, 2))}}) {
		t.Fatalf("mouse events = %+v", got)
	}
	if got := backend.keyboardEvents; !reflect.DeepEqual(got, [][]KeyboardEvent{{KeyDownEvent(KeyA), KeyUpEvent(KeyA)}}) {
		t.Fatalf("keyboard events = %+v", got)
	}
}

func TestClientRunSteps(t *testing.T) {
	backend := &sequenceTestBackend{}
	client := newSequenceTestClient(backend)

	if err := client.RunSteps(context.Background(), MoveStep(Rel(1, 0)), KeyTapStep(KeyEnter, 0)); err != nil {
		t.Fatalf("RunSteps() error = %v", err)
	}
	if !reflect.DeepEqual(backend.order, []string{"mouse", "keyboard"}) {
		t.Fatalf("order = %v, want mouse then keyboard", backend.order)
	}
}

func TestClientRunInputSequenceCanceled(t *testing.T) {
	client := newSequenceTestClient(&sequenceTestBackend{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.Run(ctx, NewInputSequence(PauseStep(time.Second)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
}

func TestClientRunInputSequenceUnknownStep(t *testing.T) {
	client := newSequenceTestClient(&sequenceTestBackend{})

	err := client.Run(context.Background(), NewInputSequence(InputStep{Kind: 99}))
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Run() error = %v, want ErrUnsupported", err)
	}
}

type sequenceTestBackend struct {
	order          []string
	mouseEvents    [][]MouseEvent
	keyboardEvents [][]KeyboardEvent
}

func newSequenceTestClient(backend *sequenceTestBackend) *Client {
	client := &Client{backend: backend}
	client.Mouse = &Mouse{client: client}
	client.Keyboard = &Keyboard{client: client}
	return client
}

func (b *sequenceTestBackend) Close() error { return nil }

func (b *sequenceTestBackend) MouseInjection() MouseInjectionBackend {
	return MouseInjectionAuto
}

func (b *sequenceTestBackend) KeyboardInjection() KeyboardInjectionBackend {
	return KeyboardInjectionAuto
}

func (b *sequenceTestBackend) InputTag() uintptr { return 0 }

func (b *sequenceTestBackend) ScreenSize(context.Context) (Point, error) {
	return Point{}, ErrUnsupported
}

func (b *sequenceTestBackend) CursorPos(context.Context) (Point, error) {
	return Point{}, ErrUnsupported
}

func (b *sequenceTestBackend) MouseButtonState(context.Context, MouseButton) (State, error) {
	return Up, nil
}

func (b *sequenceTestBackend) InjectMouse(_ context.Context, events []MouseEvent) error {
	b.order = append(b.order, "mouse")
	b.mouseEvents = append(b.mouseEvents, append([]MouseEvent(nil), events...))
	return nil
}

func (b *sequenceTestBackend) MoveMouse(context.Context, MouseMove) error { return nil }

func (b *sequenceTestBackend) SetMouseButton(context.Context, MouseButton, State) error {
	return nil
}

func (b *sequenceTestBackend) KeyState(context.Context, Key) (State, error) {
	return Up, nil
}

func (b *sequenceTestBackend) InjectKeyboard(_ context.Context, events []KeyboardEvent) error {
	b.order = append(b.order, "keyboard")
	b.keyboardEvents = append(b.keyboardEvents, append([]KeyboardEvent(nil), events...))
	return nil
}

func (b *sequenceTestBackend) SetKey(context.Context, Key, State) error { return nil }

func (b *sequenceTestBackend) ListenInput(context.Context, ListenOptions) (*Listener, error) {
	return nil, ErrUnsupported
}
