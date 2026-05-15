//go:build linux

package makc

// WithMouseUInput selects the Linux uinput kernel interface for mouse
// injection. Default backend on Linux. Requires write access to
// /dev/uinput; absolute movement also requires an X11 DISPLAY.
func WithMouseUInput() Option { return WithMouseInjection(MouseInjectionUInput) }

// WithKeyboardUInput selects the Linux uinput kernel interface for keyboard
// injection. Default backend on Linux.
func WithKeyboardUInput() Option { return WithKeyboardInjection(KeyboardInjectionUInput) }

// WithMouseXDGPortal selects the XDG desktop portal RemoteDesktop interface
// for mouse injection. Required on Wayland sessions where uinput is
// unavailable or insufficient (compositors typically ignore uinput devices
// without an active session). Triggers an interactive permission dialog on
// the first Open call; the granted session persists for the lifetime of
// the Client. Pair with WithKeyboardXDGPortal when a single session should
// drive both devices.
func WithMouseXDGPortal() Option { return WithMouseInjection(MouseInjectionXDGPortal) }

// WithKeyboardXDGPortal selects the XDG desktop portal RemoteDesktop
// interface for keyboard injection. See WithMouseXDGPortal for the
// permission flow. Unicode text events (KeyboardEventText) are not
// supported on this backend — the portal protocol exposes only keycode
// and keysym methods.
func WithKeyboardXDGPortal() Option { return WithKeyboardInjection(KeyboardInjectionXDGPortal) }
