//go:build darwin

package makc

// WithMouseCGEvent selects CoreGraphics CGEvent APIs for mouse injection.
// This is the only mouse injection backend available on macOS; passing the
// Option is equivalent to leaving it at the default (Auto).
func WithMouseCGEvent() Option { return WithMouseInjection(MouseInjectionCGEvent) }

// WithKeyboardCGEvent selects CoreGraphics CGEvent APIs for keyboard
// injection. This is the only keyboard injection backend available on
// macOS; passing the Option is equivalent to leaving it at the default.
func WithKeyboardCGEvent() Option { return WithKeyboardInjection(KeyboardInjectionCGEvent) }
