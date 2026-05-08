package makc

import (
	"context"
	"time"
)

// Mouse exposes mouse state and injection operations.
type Mouse struct {
	client *Client
}

// InjectionBackend returns the backend currently used for mouse injection.
func (m *Mouse) InjectionBackend() MouseInjectionBackend {
	if m == nil || m.client == nil || m.client.backend == nil {
		return MouseInjectionAuto
	}
	return m.client.backend.MouseInjection()
}

// Position returns the current cursor position.
func (m *Mouse) Position(ctx context.Context) (Point, error) {
	if err := m.ensureReady(ctx); err != nil {
		return Point{}, err
	}
	return m.client.backend.CursorPos(ctx)
}

// ScreenSize returns the primary screen size in pixels.
func (m *Mouse) ScreenSize(ctx context.Context) (Point, error) {
	if err := m.ensureReady(ctx); err != nil {
		return Point{}, err
	}
	return m.client.backend.ScreenSize(ctx)
}

// State returns the current state of a mouse button.
func (m *Mouse) State(ctx context.Context, button MouseButton) (State, error) {
	if err := m.ensureReady(ctx); err != nil {
		return Up, err
	}
	return m.client.backend.MouseButtonState(ctx, button)
}

// Down reports whether a mouse button is currently pressed.
func (m *Mouse) Down(ctx context.Context, button MouseButton) (bool, error) {
	state, err := m.State(ctx, button)
	return state.Bool(), err
}

// Move injects one mouse movement operation.
func (m *Mouse) Move(ctx context.Context, move MouseMove) error {
	return m.Inject(ctx, MouseMoveEvent(move))
}

// MoveTo moves the cursor to an absolute screen coordinate.
func (m *Mouse) MoveTo(ctx context.Context, x, y int) error {
	return m.Move(ctx, Abs(x, y))
}

// MoveBy moves the cursor by a relative delta.
func (m *Mouse) MoveBy(ctx context.Context, dx, dy int) error {
	return m.Move(ctx, Rel(dx, dy))
}

// Press injects a button-down event.
func (m *Mouse) Press(ctx context.Context, button MouseButton) error {
	return m.Inject(ctx, MouseButtonEvent(button, Down))
}

// Release injects a button-up event.
func (m *Mouse) Release(ctx context.Context, button MouseButton) error {
	return m.Inject(ctx, MouseButtonEvent(button, Up))
}

// Click injects a button-down followed by a button-up event.
func (m *Mouse) Click(ctx context.Context, button MouseButton) error {
	return m.Inject(ctx,
		MouseButtonEvent(button, Down),
		MouseButtonEvent(button, Up),
	)
}

// ClickWithProfile injects one or more clicks using explicit hold and
// between-click timing.
func (m *Mouse) ClickWithProfile(ctx context.Context, button MouseButton, profile ClickProfile) error {
	return m.Inject(ctx, profile.Events(button)...)
}

// DoubleClick injects two clicks with a fixed interval between them.
func (m *Mouse) DoubleClick(ctx context.Context, button MouseButton, hold, interval time.Duration) error {
	return m.ClickWithProfile(ctx, button, DoubleClick(hold, interval))
}

// Wheel injects vertical wheel movement in detents.
func (m *Mouse) Wheel(ctx context.Context, detents int) error {
	return m.Inject(ctx, MouseWheelEvent(detents*WheelDelta))
}

// HWheel injects horizontal wheel movement in detents.
func (m *Mouse) HWheel(ctx context.Context, detents int) error {
	return m.Inject(ctx, MouseHWheelEvent(detents*WheelDelta))
}

// Inject sends mouse events. Consecutive non-pause events are sent as one
// backend batch; pause events flush the current batch and then sleep.
func (m *Mouse) Inject(ctx context.Context, events ...MouseEvent) error {
	if err := m.ensureReady(ctx); err != nil {
		return err
	}

	batch := make([]MouseEvent, 0, len(events))
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := m.client.backend.InjectMouse(ctx, batch)
		batch = batch[:0]
		return err
	}

	for _, event := range events {
		if err := validateMouseEvent(event); err != nil {
			return err
		}
		if event.Kind != MouseEventPause {
			batch = append(batch, event)
			continue
		}
		if err := flush(); err != nil {
			return err
		}
		if err := sleepContext(ctx, event.Duration); err != nil {
			return err
		}
	}

	return flush()
}

// MoveToProfile moves to a point using a deterministic movement profile.
func (m *Mouse) MoveToProfile(ctx context.Context, to Point, profile MovementProfile) error {
	from, err := m.Position(ctx)
	if err != nil {
		return err
	}
	return m.Inject(ctx, profile.Events(from, to)...)
}

// Drag presses a button at the current cursor position, follows a deterministic
// path to the target point, and releases the button.
func (m *Mouse) Drag(ctx context.Context, button MouseButton, to Point, profile MovementProfile) error {
	from, err := m.Position(ctx)
	if err != nil {
		return err
	}
	return m.DragFrom(ctx, button, from, to, profile)
}

// DragFrom moves to the start point, presses a button, follows a deterministic
// path to the end point, and releases the button.
func (m *Mouse) DragFrom(ctx context.Context, button MouseButton, from, to Point, profile MovementProfile) error {
	events := []MouseEvent{
		MouseMoveEvent(Abs(from.X, from.Y)),
		MouseButtonEvent(button, Down),
	}
	events = append(events, profile.Events(from, to)...)
	events = append(events, MouseButtonEvent(button, Up))
	return m.Inject(ctx, events...)
}

// DragBy drags by a relative delta from the current cursor position.
func (m *Mouse) DragBy(ctx context.Context, button MouseButton, dx, dy int, profile MovementProfile) error {
	from, err := m.Position(ctx)
	if err != nil {
		return err
	}
	to := Point{X: from.X + dx, Y: from.Y + dy}
	return m.DragFrom(ctx, button, from, to, profile)
}

func (m *Mouse) ensureReady(ctx context.Context) error {
	if m == nil {
		return ErrClosed
	}
	return m.client.ensureReady(ctx)
}

func validateMouseEvent(event MouseEvent) error {
	switch event.Kind {
	case MouseEventMove, MouseEventWheel, MouseEventHWheel, MouseEventPause:
		return nil
	case MouseEventButton:
		if !event.State.valid() {
			return unsupported("unknown mouse button state")
		}
		_, err := mouseButtonName(event.Button)
		return err
	default:
		return unsupported("unknown mouse event")
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	if ctx == nil {
		time.Sleep(duration)
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func mouseButtonName(button MouseButton) (string, error) {
	name := button.String()
	if name == "unknown" {
		return "", unsupported("unknown mouse button")
	}
	return name, nil
}
