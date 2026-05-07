//go:build windows

package makc

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf16"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/windows"
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002
	keyeventfUnicode     = 0x0004
	keyeventfScanCode    = 0x0008

	mouseeventfMove       = 0x0001
	mouseeventfLeftDown   = 0x0002
	mouseeventfLeftUp     = 0x0004
	mouseeventfRightDown  = 0x0008
	mouseeventfRightUp    = 0x0010
	mouseeventfMiddleDown = 0x0020
	mouseeventfMiddleUp   = 0x0040
	mouseeventfXDown      = 0x0080
	mouseeventfXUp        = 0x0100
	mouseeventfWheel      = 0x0800
	mouseeventfHWheel     = 0x01000
	mouseeventfAbsolute   = 0x8000

	smCXScreen = 0
	smCYScreen = 1

	vkLeftButton   = 0x01
	vkRightButton  = 0x02
	vkMiddleButton = 0x04
	vkXButton1     = 0x05
	vkXButton2     = 0x06

	xbutton1 = 0x0001
	xbutton2 = 0x0002
)

type winBackend struct {
	api               *winAPI
	mouseInjection    MouseInjectionBackend
	keyboardInjection KeyboardInjectionBackend
	inputTag          uintptr
}

func newSystemBackend(cfg config) (systemBackend, error) {
	api, err := newWinAPI()
	if err != nil {
		return nil, err
	}

	mouseInjection := cfg.mouseInjection
	switch mouseInjection {
	case MouseInjectionAuto:
		if api.injectMouseInput != nil {
			mouseInjection = MouseInjectionInjectMouseInput
		} else {
			mouseInjection = MouseInjectionSendInput
		}
	case MouseInjectionInjectMouseInput:
		if api.injectMouseInput == nil {
			return nil, unsupported("user32!InjectMouseInput is not available")
		}
	case MouseInjectionSendInput:
	case MouseInjectionCGEvent:
		return nil, unsupported("CGEvent mouse injection is only available on macOS")
	case MouseInjectionUInput:
		return nil, unsupported("uinput mouse injection is only available on Linux")
	default:
		return nil, fmt.Errorf("makc: unknown mouse injection backend %d", cfg.mouseInjection)
	}

	keyboardInjection := cfg.keyboardInjection
	switch keyboardInjection {
	case KeyboardInjectionAuto:
		if api.injectKeyboardInput != nil {
			keyboardInjection = KeyboardInjectionInjectKeyboardInput
		} else {
			keyboardInjection = KeyboardInjectionSendInput
		}
	case KeyboardInjectionInjectKeyboardInput:
		if api.injectKeyboardInput == nil {
			return nil, unsupported("user32!InjectKeyboardInput is not available")
		}
	case KeyboardInjectionSendInput:
	case KeyboardInjectionCGEvent:
		return nil, unsupported("CGEvent keyboard injection is only available on macOS")
	case KeyboardInjectionUInput:
		return nil, unsupported("uinput keyboard injection is only available on Linux")
	default:
		return nil, fmt.Errorf("makc: unknown keyboard injection backend %d", cfg.keyboardInjection)
	}

	return &winBackend{
		api:               api,
		mouseInjection:    mouseInjection,
		keyboardInjection: keyboardInjection,
		inputTag:          cfg.inputTag,
	}, nil
}

func (b *winBackend) Close() error {
	return nil
}

func (b *winBackend) MouseInjection() MouseInjectionBackend {
	return b.mouseInjection
}

func (b *winBackend) KeyboardInjection() KeyboardInjectionBackend {
	return b.keyboardInjection
}

func (b *winBackend) InputTag() uintptr {
	return b.inputTag
}

func (b *winBackend) ScreenSize(ctx context.Context) (Point, error) {
	if err := checkContext(ctx); err != nil {
		return Point{}, err
	}
	return Point{
		X: int(b.api.getSystemMetrics(smCXScreen)),
		Y: int(b.api.getSystemMetrics(smCYScreen)),
	}, nil
}

func (b *winBackend) CursorPos(ctx context.Context) (Point, error) {
	if err := checkContext(ctx); err != nil {
		return Point{}, err
	}

	var p winPoint
	if ok := b.api.getCursorPos(&p); ok == 0 {
		return Point{}, fmt.Errorf("makc: GetCursorPos failed: %w", lastWindowsError())
	}
	return Point{X: int(p.X), Y: int(p.Y)}, nil
}

func (b *winBackend) MouseButtonState(ctx context.Context, button MouseButton) (State, error) {
	if err := checkContext(ctx); err != nil {
		return Up, err
	}

	vk, err := mouseButtonVK(button)
	if err != nil {
		return Up, err
	}
	if uint16(b.api.getAsyncKeyState(int32(vk)))&0x8000 != 0 {
		return Down, nil
	}
	return Up, nil
}

func (b *winBackend) MoveMouse(ctx context.Context, move MouseMove) error {
	return b.InjectMouse(ctx, []MouseEvent{MouseMoveEvent(move)})
}

func (b *winBackend) SetMouseButton(ctx context.Context, button MouseButton, state State) error {
	return b.InjectMouse(ctx, []MouseEvent{MouseButtonEvent(button, state)})
}

func (b *winBackend) InjectMouse(ctx context.Context, events []MouseEvent) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	switch b.mouseInjection {
	case MouseInjectionInjectMouseInput:
		return b.injectMouseEvents(events)
	case MouseInjectionSendInput:
		return b.sendInputMouseEvents(events)
	default:
		return fmt.Errorf("makc: unsupported mouse injection backend %s", b.mouseInjection)
	}
}

func (b *winBackend) KeyState(ctx context.Context, key Key) (State, error) {
	if err := checkContext(ctx); err != nil {
		return Up, err
	}
	if key == KeyUnknown {
		return Up, errors.New("makc: key is unknown")
	}
	if uint16(b.api.getAsyncKeyState(int32(key)))&0x8000 != 0 {
		return Down, nil
	}
	return Up, nil
}

func (b *winBackend) SetKey(ctx context.Context, key Key, state State) error {
	return b.InjectKeyboard(ctx, []KeyboardEvent{KeyEvent(key, state)})
}

func (b *winBackend) InjectKeyboard(ctx context.Context, events []KeyboardEvent) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	switch b.keyboardInjection {
	case KeyboardInjectionInjectKeyboardInput:
		return b.injectKeyboardEvents(events)
	case KeyboardInjectionSendInput:
		return b.sendInputKeyboardEvents(events)
	default:
		return fmt.Errorf("makc: unsupported keyboard injection backend %s", b.keyboardInjection)
	}
}

func keyboardEventInputs(events []KeyboardEvent, extraInfo uintptr) ([]keyboardInput, error) {
	inputs := make([]keyboardInput, 0, len(events))
	for _, event := range events {
		switch event.Kind {
		case KeyboardEventKey:
			in, err := keyboardKeyInput(event.Key, event.State)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, in)
		case KeyboardEventScanCode:
			in, err := keyboardScanCodeInput(event.ScanCode, event.State, event.Extended)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, in)
		case KeyboardEventText:
			inputs = append(inputs, keyboardTextInputs(event.Text)...)
		default:
			return nil, unsupported("unknown keyboard event")
		}
	}
	for i := range inputs {
		inputs[i].DwExtraInfo = extraInfo
	}
	return inputs, nil
}

func keyboardKeyInput(key Key, state State) (keyboardInput, error) {
	if key == KeyUnknown {
		return keyboardInput{}, errors.New("makc: key is unknown")
	}
	if !state.valid() {
		return keyboardInput{}, errors.New("makc: key state is unknown")
	}
	ki := keyboardInput{WVk: uint16(key)}
	if state == Up {
		ki.DwFlags = keyeventfKeyUp
	}
	return ki, nil
}

func keyboardScanCodeInput(scanCode uint16, state State, extended bool) (keyboardInput, error) {
	if scanCode == 0 {
		return keyboardInput{}, errors.New("makc: scan code is unknown")
	}
	if !state.valid() {
		return keyboardInput{}, errors.New("makc: key state is unknown")
	}
	ki := keyboardInput{
		WScan:   scanCode,
		DwFlags: keyeventfScanCode,
	}
	if extended {
		ki.DwFlags |= keyeventfExtendedKey
	}
	if state == Up {
		ki.DwFlags |= keyeventfKeyUp
	}
	return ki, nil
}

func keyboardTextInputs(text string) []keyboardInput {
	units := utf16.Encode([]rune(text))
	inputs := make([]keyboardInput, 0, len(units)*2)
	for _, unit := range units {
		inputs = append(inputs,
			keyboardUnicodeInput(unit, Down),
			keyboardUnicodeInput(unit, Up),
		)
	}
	return inputs
}

func keyboardUnicodeInput(unit uint16, state State) keyboardInput {
	ki := keyboardInput{
		WScan:   unit,
		DwFlags: keyeventfUnicode,
	}
	if state == Up {
		ki.DwFlags |= keyeventfKeyUp
	}
	return ki
}

func (b *winBackend) injectKeyboardEvents(events []KeyboardEvent) error {
	inputs, err := keyboardEventInputs(events, 0)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return nil
	}
	if ok := b.api.injectKeyboardInput(&inputs[0], int32(len(inputs))); ok == 0 {
		return fmt.Errorf("makc: InjectKeyboardInput failed: %w", lastWindowsError())
	}
	return nil
}

func (b *winBackend) sendInputKeyboardEvents(events []KeyboardEvent) error {
	keyboardInputs, err := keyboardEventInputs(events, b.inputTag)
	if err != nil {
		return err
	}
	inputs := make([]input, 0, len(keyboardInputs))
	for _, ki := range keyboardInputs {
		inputs = append(inputs, keyboardSendInput(ki))
	}
	return b.sendInputs(inputs)
}

func keyboardSendInput(ki keyboardInput) input {
	in := input{Type: inputKeyboard}
	*(*keyboardInput)(unsafe.Pointer(&in.Mi)) = ki
	return in
}

func (b *winBackend) injectMouseEvents(events []MouseEvent) error {
	inputs := make([]injectedMouseInput, 0, len(events))
	for _, event := range events {
		in, err := b.mouseEventInput(event, 0)
		if err != nil {
			return err
		}
		inputs = append(inputs, in)
	}
	if len(inputs) == 0 {
		return nil
	}
	if ok := b.api.injectMouseInput(&inputs[0], int32(len(inputs))); ok == 0 {
		return fmt.Errorf("makc: InjectMouseInput failed: %w", lastWindowsError())
	}
	return nil
}

func (b *winBackend) sendInputMouseEvents(events []MouseEvent) error {
	inputs := make([]input, 0, len(events))
	for _, event := range events {
		mi, err := b.mouseEventInput(event, b.inputTag)
		if err != nil {
			return err
		}
		inputs = append(inputs, input{
			Type: inputMouse,
			Mi: mouseInput{
				Dx:          mi.DeltaX,
				Dy:          mi.DeltaY,
				MouseData:   mi.MouseData,
				DwFlags:     mi.DwFlags,
				Time:        mi.Time,
				DwExtraInfo: mi.DwExtraInfo,
			},
		})
	}
	return b.sendInputs(inputs)
}

func (b *winBackend) sendInputs(inputs []input) error {
	if len(inputs) == 0 {
		return nil
	}
	sent := b.api.sendInput(uint32(len(inputs)), unsafe.Pointer(&inputs[0]), int32(unsafe.Sizeof(input{})))
	if sent != uint32(len(inputs)) {
		return fmt.Errorf("makc: SendInput sent %d of %d inputs: %w", sent, len(inputs), lastWindowsError())
	}
	return nil
}

func (b *winBackend) mouseEventInput(event MouseEvent, extraInfo uintptr) (injectedMouseInput, error) {
	switch event.Kind {
	case MouseEventMove:
		return b.mouseMoveInput(event.Move, extraInfo), nil
	case MouseEventButton:
		flags, data, err := mouseButtonFlags(event.Button, event.State)
		if err != nil {
			return injectedMouseInput{}, err
		}
		return injectedMouseInput{
			MouseData:   data,
			DwFlags:     flags,
			DwExtraInfo: extraInfo,
		}, nil
	case MouseEventWheel:
		return injectedMouseInput{
			MouseData:   uint32(int32(event.Delta)),
			DwFlags:     mouseeventfWheel,
			DwExtraInfo: extraInfo,
		}, nil
	case MouseEventHWheel:
		return injectedMouseInput{
			MouseData:   uint32(int32(event.Delta)),
			DwFlags:     mouseeventfHWheel,
			DwExtraInfo: extraInfo,
		}, nil
	default:
		return injectedMouseInput{}, unsupported("unknown mouse event")
	}
}

func (b *winBackend) mouseMoveInput(move MouseMove, extraInfo uintptr) injectedMouseInput {
	dx := int32(move.X)
	dy := int32(move.Y)
	flags := uint32(mouseeventfMove)

	if !move.Relative {
		flags |= mouseeventfAbsolute
		dx = absoluteMouseCoordinate(dx, b.api.getSystemMetrics(smCXScreen))
		dy = absoluteMouseCoordinate(dy, b.api.getSystemMetrics(smCYScreen))
	}

	return injectedMouseInput{
		DeltaX:      dx,
		DeltaY:      dy,
		DwFlags:     flags,
		DwExtraInfo: extraInfo,
	}
}

func absoluteMouseCoordinate(pos, size int32) int32 {
	if pos < 0 {
		pos = 0
	}
	if size <= 0 {
		return pos
	}
	return int32((int64(pos)*65536)/int64(size) + 1)
}

func mouseButtonVK(button MouseButton) (uint16, error) {
	switch button {
	case ButtonLeft:
		return vkLeftButton, nil
	case ButtonRight:
		return vkRightButton, nil
	case ButtonMiddle:
		return vkMiddleButton, nil
	case ButtonX1:
		return vkXButton1, nil
	case ButtonX2:
		return vkXButton2, nil
	default:
		return 0, fmt.Errorf("makc: unknown mouse button %d", button)
	}
}

func mouseButtonFlags(button MouseButton, state State) (flags uint32, data uint32, err error) {
	if !state.valid() {
		return 0, 0, errors.New("makc: mouse button state is unknown")
	}
	switch button {
	case ButtonLeft:
		if state == Down {
			return mouseeventfLeftDown, 0, nil
		}
		return mouseeventfLeftUp, 0, nil
	case ButtonRight:
		if state == Down {
			return mouseeventfRightDown, 0, nil
		}
		return mouseeventfRightUp, 0, nil
	case ButtonMiddle:
		if state == Down {
			return mouseeventfMiddleDown, 0, nil
		}
		return mouseeventfMiddleUp, 0, nil
	case ButtonX1:
		if state == Down {
			return mouseeventfXDown, xbutton1, nil
		}
		return mouseeventfXUp, xbutton1, nil
	case ButtonX2:
		if state == Down {
			return mouseeventfXDown, xbutton2, nil
		}
		return mouseeventfXUp, xbutton2, nil
	default:
		return 0, 0, fmt.Errorf("makc: unknown mouse button %d", button)
	}
}

func checkContext(ctx context.Context) error {
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

func lastWindowsError() error {
	if err := windows.GetLastError(); err != nil {
		return err
	}
	return windows.ERROR_GEN_FAILURE
}

type winAPI struct {
	getAsyncKeyState    func(int32) int16
	getCursorPos        func(*winPoint) int32
	getSystemMetrics    func(int32) int32
	sendInput           func(uint32, unsafe.Pointer, int32) uint32
	injectMouseInput    func(*injectedMouseInput, int32) int32
	injectKeyboardInput func(*keyboardInput, int32) int32

	registerRawInputDevices func(*rawInputDevice, uint32, uint32) int32
	getRawInputData         func(uintptr, uint32, unsafe.Pointer, *uint32, uint32) uint32
	registerClassEx         func(*wndClassEx) uint16
	createWindowEx          func(uint32, *uint16, *uint16, uint32, int32, int32, int32, int32, uintptr, uintptr, uintptr, uintptr) uintptr
	defWindowProc           func(uintptr, uint32, uintptr, uintptr) uintptr
	destroyWindow           func(uintptr) int32
	unregisterClass         func(*uint16, uintptr) int32

	setWindowsHookEx    func(int32, uintptr, uintptr, uint32) uintptr
	callNextHookEx      func(uintptr, int32, uintptr, uintptr) uintptr
	unhookWindowsHookEx func(uintptr) int32
	getMessage          func(*winMsg, uintptr, uint32, uint32) int32
	postThreadMessage   func(uint32, uint32, uintptr, uintptr) int32
}

func newWinAPI() (*winAPI, error) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	if err := user32.Load(); err != nil {
		return nil, fmt.Errorf("makc: load user32.dll: %w", err)
	}

	api := &winAPI{}
	handle := windows.Handle(user32.Handle())

	if err := registerProc(handle, &api.getAsyncKeyState, "GetAsyncKeyState"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.getCursorPos, "GetCursorPos"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.getSystemMetrics, "GetSystemMetrics"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.sendInput, "SendInput"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.registerRawInputDevices, "RegisterRawInputDevices"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.getRawInputData, "GetRawInputData"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.registerClassEx, "RegisterClassExW"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.createWindowEx, "CreateWindowExW"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.defWindowProc, "DefWindowProcW"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.destroyWindow, "DestroyWindow"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.unregisterClass, "UnregisterClassW"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.setWindowsHookEx, "SetWindowsHookExW"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.callNextHookEx, "CallNextHookEx"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.unhookWindowsHookEx, "UnhookWindowsHookEx"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.getMessage, "GetMessageW"); err != nil {
		return nil, err
	}
	if err := registerProc(handle, &api.postThreadMessage, "PostThreadMessageW"); err != nil {
		return nil, err
	}

	_ = registerOptionalProc(handle, &api.injectMouseInput, "InjectMouseInput")
	_ = registerOptionalProc(handle, &api.injectKeyboardInput, "InjectKeyboardInput")
	return api, nil
}

func registerProc(handle windows.Handle, fptr any, name string) error {
	proc, err := windows.GetProcAddress(handle, name)
	if err != nil {
		return fmt.Errorf("makc: load user32!%s: %w", name, err)
	}
	purego.RegisterFunc(fptr, proc)
	return nil
}

func registerOptionalProc(handle windows.Handle, fptr any, name string) error {
	proc, err := windows.GetProcAddress(handle, name)
	if err != nil {
		return err
	}
	purego.RegisterFunc(fptr, proc)
	return nil
}

type winPoint struct {
	X int32
	Y int32
}

type winMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      winPoint
}

type injectedMouseInput struct {
	DeltaX      int32
	DeltaY      int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type keyboardInput struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type input struct {
	Type uint32
	Mi   mouseInput
}
