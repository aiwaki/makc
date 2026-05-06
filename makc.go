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
	// current operating system or Windows build.
	ErrUnsupported = errors.New("makc: unsupported backend")

	// ErrClosed is returned after Client.Close has been called.
	ErrClosed = errors.New("makc: client is closed")
)

// MouseInjectionBackend selects the Windows mouse input injection primitive.
type MouseInjectionBackend uint8

const (
	// MouseInjectionAuto prefers InjectMouseInput when user32 exports it and
	// falls back to SendInput otherwise.
	MouseInjectionAuto MouseInjectionBackend = iota

	// MouseInjectionSendInput uses the documented Win32 SendInput API.
	MouseInjectionSendInput

	// MouseInjectionInjectMouseInput uses user32!InjectMouseInput when the
	// symbol is present. This backend is intentionally isolated because the
	// symbol is not available on every Windows build.
	MouseInjectionInjectMouseInput
)

func (b MouseInjectionBackend) String() string {
	switch b {
	case MouseInjectionAuto:
		return "auto"
	case MouseInjectionSendInput:
		return "sendinput"
	case MouseInjectionInjectMouseInput:
		return "injectmouseinput"
	default:
		return "unknown"
	}
}

// KeyboardInjectionBackend selects the Windows keyboard input injection
// primitive.
type KeyboardInjectionBackend uint8

const (
	// KeyboardInjectionAuto prefers InjectKeyboardInput when user32 exports it
	// and falls back to SendInput otherwise.
	KeyboardInjectionAuto KeyboardInjectionBackend = iota

	// KeyboardInjectionSendInput uses the documented Win32 SendInput API.
	KeyboardInjectionSendInput

	// KeyboardInjectionInjectKeyboardInput uses user32!InjectKeyboardInput when
	// the symbol is present. This backend is intentionally isolated because the
	// symbol is not available on every Windows build.
	KeyboardInjectionInjectKeyboardInput
)

func (b KeyboardInjectionBackend) String() string {
	switch b {
	case KeyboardInjectionAuto:
		return "auto"
	case KeyboardInjectionSendInput:
		return "sendinput"
	case KeyboardInjectionInjectKeyboardInput:
		return "injectkeyboardinput"
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
// injection on Windows.
func WithMouseInjection(backend MouseInjectionBackend) Option {
	return func(cfg *config) {
		cfg.mouseInjection = backend
	}
}

// WithKeyboardInjection selects the backend used by Keyboard injection on
// Windows.
func WithKeyboardInjection(backend KeyboardInjectionBackend) Option {
	return func(cfg *config) {
		cfg.keyboardInjection = backend
	}
}

// WithInputTag sets the Win32 dwExtraInfo value used on injected inputs.
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

// InputTag returns the Win32 dwExtraInfo tag used by this client for injected
// inputs. A zero tag means input tagging is disabled.
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
