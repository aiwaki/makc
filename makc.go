package makc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

var (
	// ErrUnsupported is returned when the selected backend cannot run on the
	// current operating system or platform build.
	ErrUnsupported = errors.New("makc: unsupported backend")

	// ErrClosed is returned after Client.Close has been called.
	ErrClosed = errors.New("makc: client is closed")
)

// MouseInjectionBackend selects the mouse input injection primitive.
type MouseInjectionBackend uint8

const (
	// MouseInjectionAuto selects the preferred backend for the current platform.
	// On Windows this prefers InjectMouseInput when user32 exports it and falls
	// back to SendInput otherwise. On macOS this selects CGEvent. On Linux this
	// selects uinput.
	MouseInjectionAuto MouseInjectionBackend = iota

	// MouseInjectionSendInput uses the documented Win32 SendInput API.
	MouseInjectionSendInput

	// MouseInjectionInjectMouseInput uses user32!InjectMouseInput when the
	// symbol is present. This backend is intentionally isolated because the
	// symbol is not available on every Windows build.
	MouseInjectionInjectMouseInput

	// MouseInjectionCGEvent uses CoreGraphics CGEvent APIs on macOS.
	MouseInjectionCGEvent

	// MouseInjectionUInput uses the Linux uinput kernel interface.
	MouseInjectionUInput
)

func (b MouseInjectionBackend) String() string {
	switch b {
	case MouseInjectionAuto:
		return "auto"
	case MouseInjectionSendInput:
		return "sendinput"
	case MouseInjectionInjectMouseInput:
		return "injectmouseinput"
	case MouseInjectionCGEvent:
		return "cgevent"
	case MouseInjectionUInput:
		return "uinput"
	default:
		return "unknown"
	}
}

// KeyboardInjectionBackend selects the keyboard input injection primitive.
type KeyboardInjectionBackend uint8

const (
	// KeyboardInjectionAuto selects the preferred backend for the current
	// platform. On Windows this prefers InjectKeyboardInput when user32 exports
	// it and falls back to SendInput otherwise. On macOS this selects CGEvent.
	// On Linux this selects uinput.
	KeyboardInjectionAuto KeyboardInjectionBackend = iota

	// KeyboardInjectionSendInput uses the documented Win32 SendInput API.
	KeyboardInjectionSendInput

	// KeyboardInjectionInjectKeyboardInput uses user32!InjectKeyboardInput when
	// the symbol is present. This backend is intentionally isolated because the
	// symbol is not available on every Windows build.
	KeyboardInjectionInjectKeyboardInput

	// KeyboardInjectionCGEvent uses CoreGraphics CGEvent APIs on macOS.
	KeyboardInjectionCGEvent

	// KeyboardInjectionUInput uses the Linux uinput kernel interface.
	KeyboardInjectionUInput
)

func (b KeyboardInjectionBackend) String() string {
	switch b {
	case KeyboardInjectionAuto:
		return "auto"
	case KeyboardInjectionSendInput:
		return "sendinput"
	case KeyboardInjectionInjectKeyboardInput:
		return "injectkeyboardinput"
	case KeyboardInjectionCGEvent:
		return "cgevent"
	case KeyboardInjectionUInput:
		return "uinput"
	default:
		return "unknown"
	}
}

type config struct {
	mouseInjection    MouseInjectionBackend
	keyboardInjection KeyboardInjectionBackend
	inputTag          uintptr
}

// Option configures a Client.
type Option func(*config)

// WithMouseInjection selects the backend used by Mouse movement and button
// injection.
func WithMouseInjection(backend MouseInjectionBackend) Option {
	return func(cfg *config) {
		cfg.mouseInjection = backend
	}
}

// WithKeyboardInjection selects the backend used by Keyboard injection.
func WithKeyboardInjection(backend KeyboardInjectionBackend) Option {
	return func(cfg *config) {
		cfg.keyboardInjection = backend
	}
}

// WithInputTag sets the backend tag used on injected inputs where supported.
// On Windows SendInput this maps to dwExtraInfo.
//
// The default is a non-zero per-client tag. Passing 0 disables tagging and
// own-event detection.
func WithInputTag(tag uintptr) Option {
	return func(cfg *config) {
		cfg.inputTag = tag
	}
}

// Client owns the lifecycle of a makc backend.
type Client struct {
	Mouse    *Mouse
	Keyboard *Keyboard

	backend systemBackend
	closed  bool
}

// Open initializes a makc client.
func Open(opts ...Option) (*Client, error) {
	cfg := config{
		mouseInjection:    MouseInjectionAuto,
		keyboardInjection: KeyboardInjectionAuto,
		inputTag:          nextInputTag(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	backend, err := newSystemBackend(cfg)
	if err != nil {
		return nil, err
	}

	c := &Client{
		backend: backend,
	}
	c.Mouse = &Mouse{client: c}
	c.Keyboard = &Keyboard{client: c}
	return c, nil
}

// InputTag returns the backend tag used by this client for injected inputs when
// the current platform supports tagging. A zero tag means input tagging is
// disabled or unavailable.
func (c *Client) InputTag() uintptr {
	if c == nil || c.backend == nil {
		return 0
	}
	return c.backend.InputTag()
}

// Close releases backend resources. It is safe to call Close more than once.
func (c *Client) Close() error {
	if c == nil || c.closed {
		return nil
	}
	c.closed = true
	return c.backend.Close()
}

func (c *Client) ensureReady(ctx context.Context) error {
	if c == nil || c.backend == nil || c.closed {
		return ErrClosed
	}
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func unsupported(name string) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, name)
}

var inputTagCounter uint64

func nextInputTag() uintptr {
	seed := uint64(0x4d414b43) << 32
	tag := seed ^
		(uint64(os.Getpid()) << 16) ^
		uint64(time.Now().UnixNano()) ^
		atomic.AddUint64(&inputTagCounter, 1)
	if tag == 0 {
		tag = 1
	}
	return uintptr(tag)
}
