package makc

import (
	"context"
	"sync"
	"sync/atomic"
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
	// Backend selects the listening primitive. Zero uses the platform default.
	Backend ListenBackend

	// Mask selects which input families should be emitted. Zero means ListenAll.
	Mask ListenMask

	// Buffer sets the Events channel capacity. Values less than one use the
	// package default.
	Buffer int

	// IncludeInjected includes events that the backend reports as injected.
	IncludeInjected bool

	// NormalizeOwnInjected clears injected markers from events tagged as this
	// client's own input before filtering.
	NormalizeOwnInjected bool
}

// ListenBackend selects the input listening primitive.
type ListenBackend uint8

const (
	// ListenBackendAuto uses the platform default. On Windows this uses
	// low-level hooks. On Linux this uses evdev.
	ListenBackendAuto ListenBackend = iota

	// ListenBackendLowLevelHook uses WH_MOUSE_LL and WH_KEYBOARD_LL.
	ListenBackendLowLevelHook

	// ListenBackendRawInput uses RegisterRawInputDevices and WM_INPUT.
	ListenBackendRawInput

	// ListenBackendEvdev reads Linux /dev/input/event* devices.
	ListenBackendEvdev
)

func (b ListenBackend) String() string {
	switch b {
	case ListenBackendAuto:
		return "auto"
	case ListenBackendLowLevelHook:
		return "hook"
	case ListenBackendRawInput:
		return "rawinput"
	case ListenBackendEvdev:
		return "evdev"
	default:
		return "unknown"
	}
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
	// Kind identifies the event payload and operation.
	Kind InputEventKind

	// Time is when makc observed the event.
	Time time.Time

	// Injected reports whether the platform marked the event as injected.
	Injected bool

	// LowerIntegrityInjected reports the Windows lower-integrity injected
	// marker when available.
	LowerIntegrityInjected bool

	// Own reports whether the event matches this client's input tag where the
	// backend supports tagging.
	Own bool

	// Raw reports whether the event came from a raw input stream.
	Raw bool

	// Device is a backend-specific device handle or identifier.
	Device uintptr

	// ExtraInfo is backend-specific event metadata, such as Win32 dwExtraInfo.
	ExtraInfo uintptr

	// Mouse contains mouse event details when Kind is a mouse event.
	Mouse MouseInputEvent

	// Keyboard contains keyboard event details when Kind is InputEventKey.
	Keyboard KeyboardInputEvent
}

// MouseInputEvent contains listened mouse event data.
type MouseInputEvent struct {
	// Position is the absolute cursor position reported with the event.
	Position Point

	// Move is the movement payload for mouse move events.
	Move MouseMove

	// Button is the button payload for button events.
	Button MouseButton

	// State is the button state for button events.
	State State

	// Delta is the raw wheel delta for wheel events.
	Delta int
}

// KeyboardInputEvent contains listened keyboard event data.
type KeyboardInputEvent struct {
	// Key is the virtual key for keyboard events when known.
	Key Key

	// ScanCode is the hardware scan code reported by the backend when known.
	ScanCode uint16

	// State is the key state for key events.
	State State

	// Extended reports whether the backend marked the key as extended.
	Extended bool

	// AltDown reports whether Alt was down during the key event.
	AltDown bool
}

// Listener owns an active input listener.
type Listener struct {
	// Events receives listened input events until the listener stops.
	Events <-chan InputEvent

	done   <-chan error
	cancel context.CancelFunc
	stats  *listenerStats

	// Wait coordination. waitOnce runs the single drain of done into
	// waitErr; waitDone broadcasts completion to repeat callers so
	// Listener.Wait can be called more than once without hanging on the
	// already-drained one-shot done channel.
	waitOnce sync.Once
	waitErr  error
	waitDone chan struct{}
}

func newListener(events <-chan InputEvent, done <-chan error, cancel context.CancelFunc, stats *listenerStats) *Listener {
	return &Listener{
		Events:   events,
		done:     done,
		cancel:   cancel,
		stats:    stats,
		waitDone: make(chan struct{}),
	}
}

// ListenerStats reports counters maintained by an active or finished listener.
type ListenerStats struct {
	// Delivered is the number of events successfully sent on Events since
	// the listener started.
	Delivered uint64

	// Dropped is the number of events the listener observed but dropped
	// because the Events channel buffer was full. Tune ListenOptions.Buffer
	// or drain Events faster if Dropped grows.
	Dropped uint64
}

// listenerStats is the internal counter pair shared between the goroutine
// producing events and Listener.Stats.
type listenerStats struct {
	delivered atomic.Uint64
	dropped   atomic.Uint64
}

func newListenerStats() *listenerStats {
	return &listenerStats{}
}

// Stats returns a snapshot of the listener's delivery counters. It is safe
// to call from any goroutine, including after the listener stops.
func (l *Listener) Stats() ListenerStats {
	if l == nil || l.stats == nil {
		return ListenerStats{}
	}
	return ListenerStats{
		Delivered: l.stats.delivered.Load(),
		Dropped:   l.stats.dropped.Load(),
	}
}

// Close requests listener shutdown.
func (l *Listener) Close() {
	if l == nil || l.cancel == nil {
		return
	}
	l.cancel()
}

// Wait blocks until the listener stops and returns the listener's exit
// error. Safe to call from multiple goroutines and to call repeatedly:
// the first call drains the underlying done channel, subsequent calls
// observe the broadcast and return the same cached error.
func (l *Listener) Wait() error {
	if l == nil {
		return nil
	}
	l.waitOnce.Do(func() {
		if l.done != nil {
			l.waitErr = <-l.done
		}
		if l.waitDone != nil {
			close(l.waitDone)
		}
	})
	if l.waitDone != nil {
		<-l.waitDone
	}
	return l.waitErr
}

// Listen starts an input listener.
func (c *Client) Listen(ctx context.Context, opts ListenOptions) (*Listener, error) {
	if err := c.ensureReady(ctx); err != nil {
		return nil, err
	}
	ctx = contextOrBackground(ctx)
	opts = normalizeListenOptions(opts)
	if err := validateListenOptions(opts); err != nil {
		return nil, err
	}
	return c.backend.ListenInput(ctx, opts)
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

func validateListenOptions(opts ListenOptions) error {
	if opts.Mask&^ListenAll != 0 {
		return unsupported("unknown listen mask")
	}
	return nil
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
