package makc

import "context"

type systemBackend interface {
	Close() error

	MouseInjection() MouseInjectionBackend
	KeyboardInjection() KeyboardInjectionBackend
	InputTag() uintptr
	ScreenSize(context.Context) (Point, error)
	CursorPos(context.Context) (Point, error)
	MouseButtonState(context.Context, MouseButton) (State, error)
	InjectMouse(context.Context, []MouseEvent) error
	MoveMouse(context.Context, MouseMove) error
	SetMouseButton(context.Context, MouseButton, State) error

	KeyState(context.Context, Key) (State, error)
	InjectKeyboard(context.Context, []KeyboardEvent) error
	SetKey(context.Context, Key, State) error

	ListenInput(context.Context, ListenOptions) (*Listener, error)
}
