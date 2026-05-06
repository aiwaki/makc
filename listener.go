package makc

import (
	"context"
	"time"
)

// ListenMask selects which input families should be listened to.
type ListenMask uint8

const (
	ListenMouse ListenMask = 1 << iota
	ListenKeyboard
	ListenAll = ListenMouse | ListenKeyboard
)

// ListenOptions configures input listening.
type ListenOptions struct {
	Mask                 ListenMask
	Buffer               int
	IncludeInjected      bool
	NormalizeOwnInjected bool
}

// InputEventKind describes one listened input event.
type InputEventKind uint8

const (
	InputEventMouseMove InputEventKind = iota + 1
	InputEventMouseButton
	InputEventMouseWheel
	InputEventMouseHWheel
	InputEventKey
)

// InputEvent is emitted by Listener.Events.
type InputEvent struct {
	Kind                   InputEventKind
	Time                   time.Time
	Injected               bool
	LowerIntegrityInjected bool
	Own                    bool
	ExtraInfo              uintptr
	Mouse                  MouseInputEvent
	Keyboard               KeyboardInputEvent
}

// MouseInputEvent contains listened mouse event data.
type MouseInputEvent struct {
	Position Point
	Button   MouseButton
	State    State
	Delta    int
}

// KeyboardInputEvent contains listened keyboard event data.
type KeyboardInputEvent struct {
	Key      Key
	ScanCode uint16
	State    State
	Extended bool
	AltDown  bool
}

// Listener owns an active input listener.
type Listener struct {
	Events <-chan InputEvent

	done   <-chan error
	cancel context.CancelFunc
}

// Close requests listener shutdown.
func (l *Listener) Close() {
	if l == nil || l.cancel == nil {
		return
	}
	l.cancel()
}

// Wait blocks until the listener stops.
func (l *Listener) Wait() error {
	if l == nil || l.done == nil {
		return nil
	}
	return <-l.done
}

// Listen starts a low-level input listener.
func (c *Client) Listen(ctx context.Context, opts ListenOptions) (*Listener, error) {
	if err := c.ensureReady(ctx); err != nil {
		return nil, err
	}
	return c.backend.ListenInput(ctx, normalizeListenOptions(opts))
}

func normalizeListenOptions(opts ListenOptions) ListenOptions {
	if opts.Mask == 0 {
		opts.Mask = ListenAll
	}
	if opts.Buffer <= 0 {
		opts.Buffer = 64
	}
	return opts
}

func prepareInputEvent(event *InputEvent, opts ListenOptions) bool {
	if event == nil {
		return false
	}
	if event.Own && opts.NormalizeOwnInjected {
		event.Injected = false
		event.LowerIntegrityInjected = false
	}
	if event.Injected && !opts.IncludeInjected {
		return false
	}
	return true
}

func markOwnInputEvent(event *InputEvent, inputTag uintptr) {
	if event == nil || inputTag == 0 {
		return
	}
	event.Own = event.ExtraInfo == inputTag
}
