package makc

import (
	"context"
	"time"
)

// InputStepKind describes one step in an InputSequence.
type InputStepKind uint8

const (
	InputStepNone InputStepKind = iota
	InputStepMouse
	InputStepKeyboard
	InputStepPause
)

// InputStep is one executable sequence step. Mouse and keyboard steps may
// contain pause events; they are executed by the corresponding device API.
type InputStep struct {
	Kind     InputStepKind
	Mouse    []MouseEvent
	Keyboard []KeyboardEvent
	Duration time.Duration
}

// InputSequence is an ordered, cancelable list of input steps.
type InputSequence struct {
	Steps []InputStep
}

// NewInputSequence creates a sequence from steps.
func NewInputSequence(steps ...InputStep) InputSequence {
	return InputSequence{Steps: cloneInputSteps(steps)}
}

// Append returns a copy of the sequence with steps appended.
func (s InputSequence) Append(steps ...InputStep) InputSequence {
	next := InputSequence{Steps: make([]InputStep, 0, len(s.Steps)+len(steps))}
	next.Steps = append(next.Steps, cloneInputSteps(s.Steps)...)
	next.Steps = append(next.Steps, cloneInputSteps(steps)...)
	return next
}

// WithPause returns a copy of the sequence with a pause step appended.
func (s InputSequence) WithPause(duration time.Duration) InputSequence {
	return s.Append(PauseStep(duration))
}

// MouseStep creates a mouse event step.
func MouseStep(events ...MouseEvent) InputStep {
	return InputStep{
		Kind:  InputStepMouse,
		Mouse: append([]MouseEvent(nil), events...),
	}
}

// KeyboardStep creates a keyboard event step.
func KeyboardStep(events ...KeyboardEvent) InputStep {
	return InputStep{
		Kind:     InputStepKeyboard,
		Keyboard: append([]KeyboardEvent(nil), events...),
	}
}

// PauseStep creates a sequence-level pause step.
func PauseStep(duration time.Duration) InputStep {
	return InputStep{
		Kind:     InputStepPause,
		Duration: duration,
	}
}

// MoveStep creates a mouse movement step.
func MoveStep(move MouseMove) InputStep {
	return MouseStep(MouseMoveEvent(move))
}

// ClickStep creates a mouse click step using a click profile.
func ClickStep(button MouseButton, profile ClickProfile) InputStep {
	return MouseStep(profile.Events(button)...)
}

// DoubleClickStep creates a double-click step with fixed hold and interval
// timing.
func DoubleClickStep(button MouseButton, hold, interval time.Duration) InputStep {
	return ClickStep(button, DoubleClick(hold, interval))
}

// KeyTapStep creates a key tap step with an optional hold pause.
func KeyTapStep(key Key, hold time.Duration) InputStep {
	return KeyboardStep(KeyTapEventsWithHold(key, hold)...)
}

// ComboStep creates a keyboard combo step.
func ComboStep(keys ...Key) InputStep {
	return KeyboardStep(ComboEvents(keys...)...)
}

// TextStep creates a text input step using a typing profile.
func TextStep(text string, profile TypingProfile) InputStep {
	return KeyboardStep(profile.Events(text)...)
}

// Run executes a sequence with this client.
func (c *Client) Run(ctx context.Context, sequence InputSequence) error {
	if err := c.ensureReady(ctx); err != nil {
		return err
	}
	for _, step := range sequence.Steps {
		switch step.Kind {
		case InputStepNone:
			continue
		case InputStepMouse:
			if len(step.Mouse) == 0 {
				continue
			}
			if err := c.Mouse.Inject(ctx, step.Mouse...); err != nil {
				return err
			}
		case InputStepKeyboard:
			if len(step.Keyboard) == 0 {
				continue
			}
			if err := c.Keyboard.Inject(ctx, step.Keyboard...); err != nil {
				return err
			}
		case InputStepPause:
			if err := sleepContext(ctx, step.Duration); err != nil {
				return err
			}
		default:
			return unsupported("unknown input sequence step")
		}
	}
	return nil
}

// RunSteps executes steps with this client.
func (c *Client) RunSteps(ctx context.Context, steps ...InputStep) error {
	return c.Run(ctx, NewInputSequence(steps...))
}

func cloneInputSteps(steps []InputStep) []InputStep {
	if len(steps) == 0 {
		return nil
	}
	out := make([]InputStep, len(steps))
	for i, step := range steps {
		out[i] = step
		out[i].Mouse = append([]MouseEvent(nil), step.Mouse...)
		out[i].Keyboard = append([]KeyboardEvent(nil), step.Keyboard...)
	}
	return out
}
