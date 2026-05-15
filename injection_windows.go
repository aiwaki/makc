//go:build windows

package makc

// WithMouseSendInput selects the documented Win32 SendInput API for mouse
// injection. Available on every Windows build.
func WithMouseSendInput() Option { return WithMouseInjection(MouseInjectionSendInput) }

// WithMouseInjectMouseInput selects user32!InjectMouseInput for mouse
// injection. The symbol is not available on every Windows build; Open
// returns ErrUnsupported when the export is missing. Use WithMouseSendInput
// for a guaranteed-available alternative.
func WithMouseInjectMouseInput() Option { return WithMouseInjection(MouseInjectionInjectMouseInput) }

// WithKeyboardSendInput selects the documented Win32 SendInput API for
// keyboard injection.
func WithKeyboardSendInput() Option { return WithKeyboardInjection(KeyboardInjectionSendInput) }

// WithKeyboardInjectKeyboardInput selects user32!InjectKeyboardInput for
// keyboard injection. The symbol is not available on every Windows build;
// Open returns ErrUnsupported when the export is missing.
func WithKeyboardInjectKeyboardInput() Option {
	return WithKeyboardInjection(KeyboardInjectionInjectKeyboardInput)
}
