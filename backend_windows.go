//go:build windows

package makc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	inputMouse    = 0
	inputKeyboard = 1

	keyeventfExtendedKey = 0x0001
	keyeventfKeyUp       = 0x0002
	keyeventfUnicode     = 0x0004
	keyeventfScanCode    = 0x0008

	mouseeventfMove            = 0x0001
	mouseeventfLeftDown        = 0x0002
	mouseeventfLeftUp          = 0x0004
	mouseeventfRightDown       = 0x0008
	mouseeventfRightUp         = 0x0010
	mouseeventfMiddleDown      = 0x0020
	mouseeventfMiddleUp        = 0x0040
	mouseeventfXDown           = 0x0080
	mouseeventfXUp             = 0x0100
	mouseeventfWheel           = 0x0800
	mouseeventfHWheel          = 0x01000
	mouseeventfMoveNoCoalesce  = 0x2000
	mouseeventfVirtualDesk     = 0x4000
	mouseeventfAbsolute        = 0x8000

	smCXScreen        = 0
	smCYScreen        = 1
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79

	spiGetMouseSpeed = 0x0070

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
	mouseMotionFlags  MouseMotionFlag

	// Singleton callbacks for low-level hooks and the raw-input window
	// procedure. windows.NewCallback allocates a thunk slot from a small
	// global table (~2000 entries) that is never released. Allocating new
	// slots per Listen call guarantees an eventual exhaustion panic on a
	// long-running process. We register each callback at most once for
	// the lifetime of the backend and route the actual event delivery
	// through atomic-loaded emitter pointers, which Listen swaps in and
	// out on start/stop.
	hookCallbacksOnce sync.Once
	mouseHookCallback uintptr
	kbdHookCallback   uintptr
	wndProcCallback   uintptr

	activeMouseEmitter atomic.Pointer[hookEmitter]
	activeKbdEmitter   atomic.Pointer[hookEmitter]
	activeRawEmitter   atomic.Pointer[hookEmitter]

	// cachedCursor is the most recent cursor position observed by an
	// active mouse hook listener. Populated by the LL mouse hook on
	// every event (Pt is always present per WM_MOUSE_LL docs); read by
	// CursorPos as a fast path that skips the GetCursorPos syscall when
	// a listener is keeping the cache fresh. Cleared on listener stop
	// to prevent stale reads.
	cachedCursor atomic.Pointer[Point]

	// Raw-input window class is registered lazily once per backend and
	// torn down in Close. Each Listen call creates its own ephemeral
	// window of this class.
	rawClassOnce  sync.Once
	rawClassErr   error
	rawClassName  *uint16
	rawClassAtom  uint16
}

// hookEmitter is the per-Listen receiver wired into the singleton hook
// callbacks. Listen installs an emitter in the matching atomic.Pointer slot
// before installing its OS hook and clears the slot on shutdown. The hot path
// in the callback does one atomic load, one nil-check, and dispatches.
type hookEmitter struct {
	emit func(InputEvent)
}

func newSystemBackend(cfg config) (systemBackend, error) {
	api, err := newWinAPI()
	if err != nil {
		return nil, err
	}

	mouseInjection := cfg.mouseInjection
	switch mouseInjection {
	case MouseInjectionAuto:
		if api.hasInjectMouseInput {
			mouseInjection = MouseInjectionInjectMouseInput
		} else {
			mouseInjection = MouseInjectionSendInput
		}
	case MouseInjectionInjectMouseInput:
		if !api.hasInjectMouseInput {
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
		if api.hasInjectKeyboardInput {
			keyboardInjection = KeyboardInjectionInjectKeyboardInput
		} else {
			keyboardInjection = KeyboardInjectionSendInput
		}
	case KeyboardInjectionInjectKeyboardInput:
		if !api.hasInjectKeyboardInput {
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
		mouseMotionFlags:  cfg.mouseMotionFlags,
	}, nil
}

func (b *winBackend) Close() error {
	if b == nil {
		return nil
	}
	if b.rawClassName != nil && b.rawClassAtom != 0 {
		b.api.unregisterClass(b.rawClassName, 0)
		b.rawClassName = nil
		b.rawClassAtom = 0
	}
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

	if b.activeMouseEmitter.Load() != nil {
		if cached := b.cachedCursor.Load(); cached != nil {
			return *cached, nil
		}
	}

	var p winPoint
	if err := b.api.getCursorPos(&p); err != nil {
		return Point{}, err
	}
	return Point{X: int(p.X), Y: int(p.Y)}, nil
}

func (b *winBackend) MouseSystemSpeed(ctx context.Context) (int, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	var speed uint32
	if err := b.api.systemParametersInfo(spiGetMouseSpeed, 0, unsafe.Pointer(&speed), 0); err != nil {
		return 0, err
	}
	return int(speed), nil
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
	return b.api.injectKeyboardInput(&inputs[0], int32(len(inputs)))
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
	return b.api.injectMouseInput(&inputs[0], int32(len(inputs)))
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
	_, err := b.api.sendInput(uint32(len(inputs)), unsafe.Pointer(&inputs[0]), int32(unsafe.Sizeof(input{})))
	return err
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

	if b.mouseMotionFlags&MouseMotionNoCoalesce != 0 {
		flags |= mouseeventfMoveNoCoalesce
	}

	if !move.Relative {
		flags |= mouseeventfAbsolute
		// Per MSDN, MOUSEEVENTF_VIRTUALDESK requires MOUSEEVENTF_ABSOLUTE
		// and reinterprets the coordinate space as the union of all
		// monitors (origin = SM_XVIRTUALSCREEN, size =
		// SM_CXVIRTUALSCREEN). For relative moves, VIRTUALDESK is a no-op
		// because relative deltas don't carry coordinate-space semantics.
		if b.mouseMotionFlags&MouseMotionVirtualDesk != 0 {
			flags |= mouseeventfVirtualDesk
			originX := b.api.getSystemMetrics(smXVirtualScreen)
			originY := b.api.getSystemMetrics(smYVirtualScreen)
			width := b.api.getSystemMetrics(smCXVirtualScreen)
			height := b.api.getSystemMetrics(smCYVirtualScreen)
			dx = absoluteMouseCoordinate(dx-originX, width)
			dy = absoluteMouseCoordinate(dy-originY, height)
		} else {
			dx = absoluteMouseCoordinate(dx, b.api.getSystemMetrics(smCXScreen))
			dy = absoluteMouseCoordinate(dy, b.api.getSystemMetrics(smCYScreen))
		}
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

// winAPI binds the Win32 calls used by makc through *windows.LazyProc. We
// deliberately do NOT use purego.RegisterFunc here even though it would work:
// purego routes through syscall.Syscall6 but does not preserve the connection
// between the syscall and a follow-up GetLastError read on the same OS thread.
// In practice that meant lastWindowsError() returned stale errno bytes from
// other goroutines. *windows.LazyProc.Call returns the syscall.Errno directly
// from the call site, sidestepping the TLS race entirely.
type winAPI struct {
	user32 *windows.LazyDLL

	procGetAsyncKeyState        *windows.LazyProc
	procGetCursorPos            *windows.LazyProc
	procGetSystemMetrics        *windows.LazyProc
	procSendInput               *windows.LazyProc
	procInjectMouseInput        *windows.LazyProc // optional
	procInjectKeyboardInput     *windows.LazyProc // optional
	procRegisterRawInputDevices *windows.LazyProc
	procGetRawInputData         *windows.LazyProc
	procRegisterClassEx         *windows.LazyProc
	procCreateWindowEx          *windows.LazyProc
	procDefWindowProc           *windows.LazyProc
	procDestroyWindow           *windows.LazyProc
	procUnregisterClass         *windows.LazyProc
	procSetWindowsHookEx        *windows.LazyProc
	procCallNextHookEx          *windows.LazyProc
	procUnhookWindowsHookEx     *windows.LazyProc
	procGetMessage              *windows.LazyProc
	procPostThreadMessage       *windows.LazyProc
	procSystemParametersInfo    *windows.LazyProc

	hasInjectMouseInput    bool
	hasInjectKeyboardInput bool
}

func newWinAPI() (*winAPI, error) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	if err := user32.Load(); err != nil {
		return nil, fmt.Errorf("makc: load user32.dll: %w", err)
	}

	api := &winAPI{user32: user32}

	required := []struct {
		dst  **windows.LazyProc
		name string
	}{
		{&api.procGetAsyncKeyState, "GetAsyncKeyState"},
		{&api.procGetCursorPos, "GetCursorPos"},
		{&api.procGetSystemMetrics, "GetSystemMetrics"},
		{&api.procSendInput, "SendInput"},
		{&api.procRegisterRawInputDevices, "RegisterRawInputDevices"},
		{&api.procGetRawInputData, "GetRawInputData"},
		{&api.procRegisterClassEx, "RegisterClassExW"},
		{&api.procCreateWindowEx, "CreateWindowExW"},
		{&api.procDefWindowProc, "DefWindowProcW"},
		{&api.procDestroyWindow, "DestroyWindow"},
		{&api.procUnregisterClass, "UnregisterClassW"},
		{&api.procSetWindowsHookEx, "SetWindowsHookExW"},
		{&api.procCallNextHookEx, "CallNextHookEx"},
		{&api.procUnhookWindowsHookEx, "UnhookWindowsHookEx"},
		{&api.procGetMessage, "GetMessageW"},
		{&api.procPostThreadMessage, "PostThreadMessageW"},
		{&api.procSystemParametersInfo, "SystemParametersInfoW"},
	}
	for _, p := range required {
		proc := user32.NewProc(p.name)
		if err := proc.Find(); err != nil {
			return nil, fmt.Errorf("makc: resolve user32!%s: %w", p.name, err)
		}
		*p.dst = proc
	}

	// Optional symbols: present on some Windows builds. Find() failure
	// is recorded as "not available" rather than an init error.
	api.procInjectMouseInput = user32.NewProc("InjectMouseInput")
	if err := api.procInjectMouseInput.Find(); err == nil {
		api.hasInjectMouseInput = true
	} else {
		api.procInjectMouseInput = nil
	}
	api.procInjectKeyboardInput = user32.NewProc("InjectKeyboardInput")
	if err := api.procInjectKeyboardInput.Find(); err == nil {
		api.hasInjectKeyboardInput = true
	} else {
		api.procInjectKeyboardInput = nil
	}
	return api, nil
}

// errnoOrDefault returns err when it is a non-zero windows.Errno; otherwise
// returns ERROR_GEN_FAILURE so callers always have something to wrap. proc.Call
// always returns a non-nil error (windows.Errno), but Errno(0) means "no
// error reported" which we treat as a generic failure when the call already
// signalled a failure via its return value.
func errnoOrDefault(err error) error {
	if errno, ok := err.(windows.Errno); ok && errno != 0 {
		return errno
	}
	return windows.ERROR_GEN_FAILURE
}

func (a *winAPI) getAsyncKeyState(vk int32) int16 {
	r, _, _ := a.procGetAsyncKeyState.Call(uintptr(vk))
	return int16(r)
}

func (a *winAPI) getCursorPos(p *winPoint) error {
	r, _, e := a.procGetCursorPos.Call(uintptr(unsafe.Pointer(p)))
	if r == 0 {
		return fmt.Errorf("makc: GetCursorPos failed: %w", errnoOrDefault(e))
	}
	return nil
}

func (a *winAPI) getSystemMetrics(idx int32) int32 {
	r, _, _ := a.procGetSystemMetrics.Call(uintptr(idx))
	return int32(r)
}

func (a *winAPI) sendInput(n uint32, p unsafe.Pointer, size int32) (uint32, error) {
	r, _, e := a.procSendInput.Call(uintptr(n), uintptr(p), uintptr(size))
	sent := uint32(r)
	if sent != n {
		return sent, fmt.Errorf("makc: SendInput sent %d of %d inputs: %w", sent, n, errnoOrDefault(e))
	}
	return sent, nil
}

func (a *winAPI) injectMouseInput(p *injectedMouseInput, n int32) error {
	if a.procInjectMouseInput == nil {
		return errors.New("makc: InjectMouseInput is not available on this Windows build")
	}
	r, _, e := a.procInjectMouseInput.Call(uintptr(unsafe.Pointer(p)), uintptr(n))
	if r == 0 {
		return fmt.Errorf("makc: InjectMouseInput failed: %w", errnoOrDefault(e))
	}
	return nil
}

func (a *winAPI) injectKeyboardInput(p *keyboardInput, n int32) error {
	if a.procInjectKeyboardInput == nil {
		return errors.New("makc: InjectKeyboardInput is not available on this Windows build")
	}
	r, _, e := a.procInjectKeyboardInput.Call(uintptr(unsafe.Pointer(p)), uintptr(n))
	if r == 0 {
		return fmt.Errorf("makc: InjectKeyboardInput failed: %w", errnoOrDefault(e))
	}
	return nil
}

func (a *winAPI) registerRawInputDevices(p *rawInputDevice, n uint32, size uint32) error {
	r, _, e := a.procRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(p)), uintptr(n), uintptr(size))
	if r == 0 {
		return fmt.Errorf("makc: RegisterRawInputDevices failed: %w", errnoOrDefault(e))
	}
	return nil
}

// getRawInputData returns the bytes copied (or written for a sizing query)
// and a non-nil error only on failure. ^uint32(0) is the documented Win32
// failure sentinel.
func (a *winAPI) getRawInputData(handle uintptr, command uint32, data unsafe.Pointer, size *uint32, headerSize uint32) (uint32, error) {
	r, _, e := a.procGetRawInputData.Call(handle, uintptr(command), uintptr(data), uintptr(unsafe.Pointer(size)), uintptr(headerSize))
	got := uint32(r)
	if got == ^uint32(0) {
		return got, errnoOrDefault(e)
	}
	return got, nil
}

func (a *winAPI) registerClassEx(wc *wndClassEx) (uint16, error) {
	r, _, e := a.procRegisterClassEx.Call(uintptr(unsafe.Pointer(wc)))
	if r == 0 {
		return 0, fmt.Errorf("makc: RegisterClassExW failed: %w", errnoOrDefault(e))
	}
	return uint16(r), nil
}

func (a *winAPI) createWindowEx(exStyle uint32, className, windowName *uint16, style uint32, x, y, w, h int32, parent, menu, instance, lpParam uintptr) (uintptr, error) {
	r, _, e := a.procCreateWindowEx.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, menu, instance, lpParam,
	)
	if r == 0 {
		return 0, fmt.Errorf("makc: CreateWindowExW failed: %w", errnoOrDefault(e))
	}
	return r, nil
}

func (a *winAPI) defWindowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	r, _, _ := a.procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *winAPI) destroyWindow(hwnd uintptr) {
	_, _, _ = a.procDestroyWindow.Call(hwnd)
}

func (a *winAPI) unregisterClass(className *uint16, instance uintptr) {
	_, _, _ = a.procUnregisterClass.Call(uintptr(unsafe.Pointer(className)), instance)
}

func (a *winAPI) setWindowsHookEx(idHook int32, lpfn uintptr, hMod uintptr, threadID uint32) (uintptr, error) {
	r, _, e := a.procSetWindowsHookEx.Call(uintptr(idHook), lpfn, hMod, uintptr(threadID))
	if r == 0 {
		return 0, fmt.Errorf("makc: SetWindowsHookExW failed: %w", errnoOrDefault(e))
	}
	return r, nil
}

func (a *winAPI) callNextHookEx(hhk uintptr, nCode int32, wParam, lParam uintptr) uintptr {
	r, _, _ := a.procCallNextHookEx.Call(hhk, uintptr(nCode), wParam, lParam)
	return r
}

func (a *winAPI) unhookWindowsHookEx(hhk uintptr) {
	_, _, _ = a.procUnhookWindowsHookEx.Call(hhk)
}

// getMessage returns >0 on a normal message, 0 on WM_QUIT, and a non-nil
// error on the documented -1 failure code.
func (a *winAPI) getMessage(msg *winMsg, hwnd uintptr, filterMin, filterMax uint32) (int32, error) {
	r, _, e := a.procGetMessage.Call(uintptr(unsafe.Pointer(msg)), hwnd, uintptr(filterMin), uintptr(filterMax))
	res := int32(r)
	if res == -1 {
		return res, fmt.Errorf("makc: GetMessageW failed: %w", errnoOrDefault(e))
	}
	return res, nil
}

func (a *winAPI) postThreadMessage(threadID uint32, msg uint32, wParam, lParam uintptr) {
	_, _, _ = a.procPostThreadMessage.Call(uintptr(threadID), uintptr(msg), wParam, lParam)
}

func (a *winAPI) systemParametersInfo(action uint32, param uint32, pv unsafe.Pointer, winIni uint32) error {
	r, _, e := a.procSystemParametersInfo.Call(uintptr(action), uintptr(param), uintptr(pv), uintptr(winIni))
	if r == 0 {
		return fmt.Errorf("makc: SystemParametersInfoW(0x%X) failed: %w", action, errnoOrDefault(e))
	}
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
