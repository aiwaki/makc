//go:build !windows

package makc

import "context"

func newSystemBackend(config) (systemBackend, error) {
	return nil, unsupported("only Windows is supported")
}

type unsupportedBackend struct{}

func (unsupportedBackend) Close() error                          { return nil }
func (unsupportedBackend) MouseInjection() MouseInjectionBackend { return MouseInjectionAuto }
func (unsupportedBackend) KeyboardInjection() KeyboardInjectionBackend {
	return KeyboardInjectionAuto
}
func (unsupportedBackend) InputTag() uintptr { return 0 }
func (unsupportedBackend) ScreenSize(context.Context) (Point, error) {
	return Point{}, unsupported("listen")
}
func (unsupportedBackend) CursorPos(context.Context) (Point, error) {
	return Point{}, unsupported("cursor position")
}
func (unsupportedBackend) MouseButtonState(context.Context, MouseButton) (State, error) {
	return Up, unsupported("mouse state")
}
func (unsupportedBackend) InjectMouse(context.Context, []MouseEvent) error {
	return unsupported("mouse injection")
}
func (unsupportedBackend) MoveMouse(context.Context, MouseMove) error {
	return unsupported("mouse injection")
}
func (unsupportedBackend) SetMouseButton(context.Context, MouseButton, State) error {
	return unsupported("mouse injection")
}
func (unsupportedBackend) KeyState(context.Context, Key) (State, error) {
	return Up, unsupported("keyboard state")
}
func (unsupportedBackend) InjectKeyboard(context.Context, []KeyboardEvent) error {
	return unsupported("keyboard injection")
}
func (unsupportedBackend) SetKey(context.Context, Key, State) error {
	return unsupported("keyboard injection")
}
func (unsupportedBackend) ListenInput(context.Context, ListenOptions) (*Listener, error) {
	return nil, unsupported("listen")
}
