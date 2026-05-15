//go:build linux

package makc

// WithMouseUInput selects the Linux uinput kernel interface for mouse
// injection. This is the only mouse injection backend available on Linux;
// passing the Option is equivalent to leaving it at the default (Auto).
func WithMouseUInput() Option { return WithMouseInjection(MouseInjectionUInput) }

// WithKeyboardUInput selects the Linux uinput kernel interface for keyboard
// injection. This is the only keyboard injection backend available on
// Linux; passing the Option is equivalent to leaving it at the default.
func WithKeyboardUInput() Option { return WithKeyboardInjection(KeyboardInjectionUInput) }
