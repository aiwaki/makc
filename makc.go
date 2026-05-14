package makc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
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
	mouseMotionFlags  MouseMotionFlag
}

// MouseMotionFlag is a bitset of platform-specific mouse motion tuning hints.
// Hints unsupported by the current backend are ignored — passing them on a
// platform that does not implement them is not an error.
type MouseMotionFlag uint8

const (
	// MouseMotionNoCoalesce disables operating-system coalescing of rapid
	// mouse move events. Maps to Win32 MOUSEEVENTF_MOVE_NOCOALESCE; ignored
	// on macOS and Linux. Use when each move event matters at high
	// frequencies, e.g. gesture capture or sub-pixel automation.
	MouseMotionNoCoalesce MouseMotionFlag = 1 << iota

	// MouseMotionVirtualDesk treats absolute mouse coordinates as positions
	// on the entire multi-monitor virtual desktop instead of the primary
	// screen. Maps to Win32 MOUSEEVENTF_VIRTUALDESK; ignored on macOS and
	// Linux. The Windows backend uses SM_CXVIRTUALSCREEN /
	// SM_CYVIRTUALSCREEN with SM_XVIRTUALSCREEN / SM_YVIRTUALSCREEN as the
	// origin when this flag is set.
	MouseMotionVirtualDesk
)

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

// WithMouseMotion enables additional mouse motion tuning hints. Pass an OR
// of MouseMotionFlag values. Hints not supported by the current backend are
// silently ignored.
func WithMouseMotion(flags MouseMotionFlag) Option {
	return func(cfg *config) {
		cfg.mouseMotionFlags |= flags
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

	backend   systemBackend
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
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

// Close releases backend resources. It is safe to call Close more than once
// and from multiple goroutines; the underlying backend Close runs exactly once.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		if c.backend != nil {
			c.closeErr = c.backend.Close()
		}
	})
	return c.closeErr
}

func (c *Client) ensureReady(ctx context.Context) error {
	if c == nil || c.backend == nil || c.closed.Load() {
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

func contextOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
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
