//go:build windows

package makc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whMouseLL    = 14
	whKeyboardLL = 13

	wmQuit  = 0x0012
	wmInput = 0x00FF

	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmMButtonDown = 0x0207
	wmMButtonUp   = 0x0208
	wmXButtonDown = 0x020B
	wmXButtonUp   = 0x020C
	wmMouseWheel  = 0x020A
	wmMouseHWheel = 0x020E
	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105

	llmhfInjected               = 0x00000001
	llmhfLowerIntegrityInjected = 0x00000002
	llkhfExtended               = 0x00000001
	llkhfLowerIntegrityInjected = 0x00000002
	llkhfInjected               = 0x00000010
	llkhfAltDown                = 0x00000020

	ridInput = 0x10000003

	rimInput        = 0
	rimTypeMouse    = 0
	rimTypeKeyboard = 1

	ridevRemove    = 0x00000001
	ridevInputSink = 0x00000100

	hidUsagePageGeneric = 0x01
	hidUsageMouse       = 0x02
	hidUsageKeyboard    = 0x06

	rawMouseMoveAbsolute = 0x0001

	rawMouseLeftButtonDown   = 0x0001
	rawMouseLeftButtonUp     = 0x0002
	rawMouseRightButtonDown  = 0x0004
	rawMouseRightButtonUp    = 0x0008
	rawMouseMiddleButtonDown = 0x0010
	rawMouseMiddleButtonUp   = 0x0020
	rawMouseButton4Down      = 0x0040
	rawMouseButton4Up        = 0x0080
	rawMouseButton5Down      = 0x0100
	rawMouseButton5Up        = 0x0200
	rawMouseWheel            = 0x0400
	rawMouseHWheel           = 0x0800

	rawKeyBreak = 0x0001
	rawKeyE0    = 0x0002
	rawKeyE1    = 0x0004
)

func (b *winBackend) ListenInput(ctx context.Context, opts ListenOptions) (*Listener, error) {
	opts = normalizeListenOptions(opts)
	switch opts.Backend {
	case ListenBackendAuto, ListenBackendLowLevelHook:
		return b.listenWithRunner(ctx, opts, b.runHookInputListener)
	case ListenBackendRawInput:
		return b.listenWithRunner(ctx, opts, b.runRawInputListener)
	case ListenBackendEvdev:
		return nil, unsupported("evdev listening is only available on Linux")
	default:
		return nil, unsupported("unknown listen backend")
	}
}

// ensureHookCallbacks lazily registers the singleton thunks for the LL mouse
// hook, the LL keyboard hook, and the raw-input window procedure. Call from
// any goroutine; the sync.Once guarantees a single registration even under
// race. windows.NewCallback slots are never freed, so registering them once
// per backend is the only way to use Listen repeatedly without exhausting
// the global thunk table.
func (b *winBackend) ensureHookCallbacks() {
	b.hookCallbacksOnce.Do(func() {
		b.mouseHookCallback = windows.NewCallback(func(nCode int, wParam uintptr, lParam *msllHookStruct) uintptr {
			if nCode >= 0 && lParam != nil {
				// Cursor cache update runs unconditionally so CursorPos
				// can serve from cache while a listener is active. Pt
				// is documented to be valid for every WH_MOUSE_LL
				// callback regardless of message type.
				p := Point{X: int(lParam.Pt.X), Y: int(lParam.Pt.Y)}
				b.cachedCursor.Store(&p)
				if e := b.activeMouseEmitter.Load(); e != nil {
					if event, ok := mouseHookEvent(uint32(wParam), lParam); ok {
						e.emit(event)
					}
				}
			}
			return b.api.callNextHookEx(0, int32(nCode), wParam, uintptr(unsafe.Pointer(lParam)))
		})
		b.kbdHookCallback = windows.NewCallback(func(nCode int, wParam uintptr, lParam *kbdllHookStruct) uintptr {
			if nCode >= 0 {
				if e := b.activeKbdEmitter.Load(); e != nil && lParam != nil {
					if event, ok := keyboardHookEvent(uint32(wParam), lParam); ok {
						e.emit(event)
					}
				}
			}
			return b.api.callNextHookEx(0, int32(nCode), wParam, uintptr(unsafe.Pointer(lParam)))
		})
		b.wndProcCallback = windows.NewCallback(func(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
			return b.api.defWindowProc(hwnd, message, wParam, lParam)
		})
	})
}

type inputListenerRunner func(context.Context, ListenOptions, *listenerStats, chan<- InputEvent, chan<- error, chan<- error)

func (b *winBackend) listenWithRunner(ctx context.Context, opts ListenOptions, runner inputListenerRunner) (*Listener, error) {
	ctx, cancel := context.WithCancel(ctx)

	events := make(chan InputEvent, opts.Buffer)
	done := make(chan error, 1)
	ready := make(chan error, 1)
	stats := newListenerStats()

	go runner(ctx, opts, stats, events, ready, done)

	select {
	case err := <-ready:
		if err != nil {
			cancel()
			return nil, err
		}
		return &Listener{
			Events: events,
			done:   done,
			cancel: cancel,
			stats:  stats,
		}, nil
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

func (b *winBackend) runHookInputListener(ctx context.Context, opts ListenOptions, stats *listenerStats, events chan<- InputEvent, ready chan<- error, done chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(events)

	b.ensureHookCallbacks()
	threadID := windows.GetCurrentThreadId()

	emitter := &hookEmitter{
		emit: func(event InputEvent) {
			markOwnInputEvent(&event, b.inputTag)
			if !prepareInputEvent(&event, opts) {
				return
			}
			select {
			case events <- event:
				stats.delivered.Add(1)
			default:
				stats.dropped.Add(1)
			}
		},
	}

	type installed struct {
		hook    uintptr
		release func()
	}
	var hooks []installed
	cleanup := func() {
		// Detach emitters before unhooking so a tail-end callback in
		// flight finds an empty slot and short-circuits to CallNextHookEx.
		for _, h := range hooks {
			h.release()
		}
		for i := len(hooks) - 1; i >= 0; i-- {
			if hooks[i].hook != 0 {
				b.api.unhookWindowsHookEx(hooks[i].hook)
			}
		}
	}

	if opts.Mask&ListenMouse != 0 {
		if !b.activeMouseEmitter.CompareAndSwap(nil, emitter) {
			ready <- errors.New("makc: another mouse hook listener is already active on this client")
			done <- nil
			return
		}
		hook, err := b.api.setWindowsHookEx(whMouseLL, b.mouseHookCallback, 0, 0)
		if err != nil {
			b.activeMouseEmitter.Store(nil)
			ready <- err
			done <- nil
			return
		}
		hooks = append(hooks, installed{hook: hook, release: func() {
			b.activeMouseEmitter.Store(nil)
			b.cachedCursor.Store(nil)
		}})
	}

	if opts.Mask&ListenKeyboard != 0 {
		if !b.activeKbdEmitter.CompareAndSwap(nil, emitter) {
			cleanup()
			ready <- errors.New("makc: another keyboard hook listener is already active on this client")
			done <- nil
			return
		}
		hook, err := b.api.setWindowsHookEx(whKeyboardLL, b.kbdHookCallback, 0, 0)
		if err != nil {
			b.activeKbdEmitter.Store(nil)
			cleanup()
			ready <- err
			done <- nil
			return
		}
		hooks = append(hooks, installed{hook: hook, release: func() { b.activeKbdEmitter.Store(nil) }})
	}

	if len(hooks) == 0 {
		ready <- unsupported("empty listen mask")
		done <- nil
		return
	}

	go func() {
		<-ctx.Done()
		b.api.postThreadMessage(threadID, wmQuit, 0, 0)
	}()

	ready <- nil

	var stopped bool
	var err error
	var msg winMsg
	for {
		result, getErr := b.api.getMessage(&msg, 0, 0, 0)
		if getErr != nil {
			err = getErr
			break
		}
		if result == 0 {
			stopped = true
			break
		}
	}

	cleanup()

	if stopped {
		done <- nil
		return
	}
	done <- err
}

func (b *winBackend) runRawInputListener(ctx context.Context, opts ListenOptions, stats *listenerStats, events chan<- InputEvent, ready chan<- error, done chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(events)

	b.ensureHookCallbacks()
	if err := b.ensureRawInputClass(); err != nil {
		ready <- err
		done <- nil
		return
	}

	threadID := windows.GetCurrentThreadId()
	windowName, err := windows.UTF16PtrFromString("makc raw input")
	if err != nil {
		ready <- err
		done <- nil
		return
	}

	hwnd, err := b.api.createWindowEx(0, b.rawClassName, windowName, 0, 0, 0, 0, 0, hwndMessage(), 0, 0, 0)
	if err != nil {
		ready <- err
		done <- nil
		return
	}
	defer b.api.destroyWindow(hwnd)

	devices := rawInputDevices(opts.Mask, hwnd, ridevInputSink)
	if len(devices) == 0 {
		ready <- unsupported("empty listen mask")
		done <- nil
		return
	}
	if err := b.api.registerRawInputDevices(&devices[0], uint32(len(devices)), uint32(unsafe.Sizeof(rawInputDevice{}))); err != nil {
		ready <- err
		done <- nil
		return
	}
	defer b.unregisterRawInputDevices(opts.Mask)

	emit := func(event InputEvent) {
		markOwnInputEvent(&event, b.inputTag)
		if !prepareInputEvent(&event, opts) {
			return
		}
		select {
		case events <- event:
			stats.delivered.Add(1)
		default:
			stats.dropped.Add(1)
		}
	}

	go func() {
		<-ctx.Done()
		b.api.postThreadMessage(threadID, wmQuit, 0, 0)
	}()

	ready <- nil

	var stopped bool
	var loopErr error
	var msg winMsg
	for {
		result, getErr := b.api.getMessage(&msg, 0, 0, 0)
		if getErr != nil {
			loopErr = getErr
			break
		}
		if result == 0 {
			stopped = true
			break
		}
		if msg.Message != wmInput {
			continue
		}
		inputEvents, err := b.rawInputEvents(msg.LParam)
		// Per MSDN WM_INPUT remarks, the application must always call
		// DefWindowProc for cleanup of the raw input data, regardless of
		// whether wParam is RIM_INPUT (foreground) or RIM_INPUTSINK
		// (background). Skipping it for RIM_INPUTSINK leaks kernel
		// buffers — and ridevInputSink is exactly the flag we register
		// with, so most events arrive as RIM_INPUTSINK.
		b.api.defWindowProc(msg.Hwnd, msg.Message, msg.WParam, msg.LParam)
		if err != nil {
			loopErr = err
			break
		}
		for _, event := range inputEvents {
			emit(event)
		}
	}

	runtime.KeepAlive(windowName)

	if stopped {
		done <- nil
		return
	}
	done <- loopErr
}

// ensureRawInputClass registers the message-only window class used by raw
// input listeners. Class name is stable per process+backend so repeated
// Listen calls reuse the same atom; the class is unregistered in
// winBackend.Close. Bound wndProc is the singleton b.wndProcCallback.
func (b *winBackend) ensureRawInputClass() error {
	b.rawClassOnce.Do(func() {
		name, err := windows.UTF16PtrFromString(fmt.Sprintf("makcRawInput%d", windows.GetCurrentProcessId()))
		if err != nil {
			b.rawClassErr = err
			return
		}
		wc := wndClassEx{
			CbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
			LpfnWndProc:   b.wndProcCallback,
			LpszClassName: name,
		}
		atom, err := b.api.registerClassEx(&wc)
		if err != nil {
			b.rawClassErr = err
			return
		}
		b.rawClassName = name
		b.rawClassAtom = atom
	})
	return b.rawClassErr
}

func rawInputDevices(mask ListenMask, hwnd uintptr, flags uint32) []rawInputDevice {
	devices := make([]rawInputDevice, 0, 2)
	if mask&ListenMouse != 0 {
		devices = append(devices, rawInputDevice{
			UsagePage:  hidUsagePageGeneric,
			Usage:      hidUsageMouse,
			Flags:      flags,
			HwndTarget: hwnd,
		})
	}
	if mask&ListenKeyboard != 0 {
		devices = append(devices, rawInputDevice{
			UsagePage:  hidUsagePageGeneric,
			Usage:      hidUsageKeyboard,
			Flags:      flags,
			HwndTarget: hwnd,
		})
	}
	return devices
}

func (b *winBackend) unregisterRawInputDevices(mask ListenMask) {
	devices := rawInputDevices(mask, 0, ridevRemove)
	if len(devices) == 0 {
		return
	}
	_ = b.api.registerRawInputDevices(&devices[0], uint32(len(devices)), uint32(unsafe.Sizeof(rawInputDevice{})))
}

func (b *winBackend) rawInputEvents(handle uintptr) ([]InputEvent, error) {
	var size uint32
	headerSize := uint32(unsafe.Sizeof(rawInputHeader{}))
	if _, err := b.api.getRawInputData(handle, ridInput, nil, &size, headerSize); err != nil {
		return nil, fmt.Errorf("makc: GetRawInputData(size) failed: %w", err)
	}
	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	if _, err := b.api.getRawInputData(handle, ridInput, unsafe.Pointer(&buf[0]), &size, headerSize); err != nil {
		return nil, fmt.Errorf("makc: GetRawInputData(input) failed: %w", err)
	}
	return parseRawInputEvents(buf), nil
}

func parseRawInputEvents(buf []byte) []InputEvent {
	if len(buf) < int(unsafe.Sizeof(rawInputHeader{})) {
		return nil
	}

	header := (*rawInputHeader)(unsafe.Pointer(&buf[0]))
	data := unsafe.Pointer(uintptr(unsafe.Pointer(&buf[0])) + unsafe.Sizeof(rawInputHeader{}))
	base := InputEvent{
		Time:   time.Now(),
		Raw:    true,
		Device: header.HDevice,
	}

	switch header.Type {
	case rimTypeMouse:
		if len(buf) < int(unsafe.Sizeof(rawInputHeader{})+unsafe.Sizeof(rawMouse{})) {
			return nil
		}
		return rawMouseEvents(base, *(*rawMouse)(data))
	case rimTypeKeyboard:
		if len(buf) < int(unsafe.Sizeof(rawInputHeader{})+unsafe.Sizeof(rawKeyboard{})) {
			return nil
		}
		if event, ok := rawKeyboardEvent(base, *(*rawKeyboard)(data)); ok {
			return []InputEvent{event}
		}
	}
	return nil
}

func rawMouseEvents(base InputEvent, mouse rawMouse) []InputEvent {
	base.ExtraInfo = uintptr(mouse.ExtraInformation)
	events := make([]InputEvent, 0, 4)

	if mouse.LastX != 0 || mouse.LastY != 0 {
		event := base
		event.Kind = InputEventMouseMove
		if mouse.Flags&rawMouseMoveAbsolute != 0 {
			event.Mouse.Move = Abs(int(mouse.LastX), int(mouse.LastY))
			event.Mouse.Position = Point{X: int(mouse.LastX), Y: int(mouse.LastY)}
		} else {
			event.Mouse.Move = Rel(int(mouse.LastX), int(mouse.LastY))
		}
		events = append(events, event)
	}

	addButton := func(flag uint16, button MouseButton, state State) {
		if mouse.ButtonFlags&flag == 0 {
			return
		}
		event := base
		event.Kind = InputEventMouseButton
		event.Mouse.Button = button
		event.Mouse.State = state
		events = append(events, event)
	}
	addButton(rawMouseLeftButtonDown, ButtonLeft, Down)
	addButton(rawMouseLeftButtonUp, ButtonLeft, Up)
	addButton(rawMouseRightButtonDown, ButtonRight, Down)
	addButton(rawMouseRightButtonUp, ButtonRight, Up)
	addButton(rawMouseMiddleButtonDown, ButtonMiddle, Down)
	addButton(rawMouseMiddleButtonUp, ButtonMiddle, Up)
	addButton(rawMouseButton4Down, ButtonX1, Down)
	addButton(rawMouseButton4Up, ButtonX1, Up)
	addButton(rawMouseButton5Down, ButtonX2, Down)
	addButton(rawMouseButton5Up, ButtonX2, Up)

	if mouse.ButtonFlags&rawMouseWheel != 0 {
		event := base
		event.Kind = InputEventMouseWheel
		event.Mouse.Delta = int(int16(mouse.ButtonData))
		events = append(events, event)
	}
	if mouse.ButtonFlags&rawMouseHWheel != 0 {
		event := base
		event.Kind = InputEventMouseHWheel
		event.Mouse.Delta = int(int16(mouse.ButtonData))
		events = append(events, event)
	}

	return events
}

func rawKeyboardEvent(base InputEvent, keyboard rawKeyboard) (InputEvent, bool) {
	event := base
	event.Kind = InputEventKey
	event.ExtraInfo = uintptr(keyboard.ExtraInformation)
	event.Keyboard = KeyboardInputEvent{
		Key:      Key(keyboard.VKey),
		ScanCode: keyboard.MakeCode,
		Extended: keyboard.Flags&(rawKeyE0|rawKeyE1) != 0,
	}

	switch keyboard.Message {
	case wmKeyDown, wmSysKeyDown:
		event.Keyboard.State = Down
	case wmKeyUp, wmSysKeyUp:
		event.Keyboard.State = Up
	default:
		if keyboard.Flags&rawKeyBreak != 0 {
			event.Keyboard.State = Up
		} else {
			event.Keyboard.State = Down
		}
	}

	return event, true
}

func hwndMessage() uintptr {
	return ^uintptr(2)
}

func mouseHookEvent(message uint32, hook *msllHookStruct) (InputEvent, bool) {
	if hook == nil {
		return InputEvent{}, false
	}

	event := InputEvent{
		Time:                   time.Now(),
		Injected:               hook.Flags&llmhfInjected != 0,
		LowerIntegrityInjected: hook.Flags&llmhfLowerIntegrityInjected != 0,
		ExtraInfo:              hook.DwExtraInfo,
	}
	event.Mouse.Position = Point{X: int(hook.Pt.X), Y: int(hook.Pt.Y)}

	switch message {
	case wmMouseMove:
		event.Kind = InputEventMouseMove
	case wmLButtonDown:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = ButtonLeft
		event.Mouse.State = Down
	case wmLButtonUp:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = ButtonLeft
		event.Mouse.State = Up
	case wmRButtonDown:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = ButtonRight
		event.Mouse.State = Down
	case wmRButtonUp:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = ButtonRight
		event.Mouse.State = Up
	case wmMButtonDown:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = ButtonMiddle
		event.Mouse.State = Down
	case wmMButtonUp:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = ButtonMiddle
		event.Mouse.State = Up
	case wmXButtonDown:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = xMouseButton(hook.MouseData)
		event.Mouse.State = Down
	case wmXButtonUp:
		event.Kind = InputEventMouseButton
		event.Mouse.Button = xMouseButton(hook.MouseData)
		event.Mouse.State = Up
	case wmMouseWheel:
		event.Kind = InputEventMouseWheel
		event.Mouse.Delta = mouseDataHighWord(hook.MouseData)
	case wmMouseHWheel:
		event.Kind = InputEventMouseHWheel
		event.Mouse.Delta = mouseDataHighWord(hook.MouseData)
	default:
		return InputEvent{}, false
	}

	return event, true
}

func keyboardHookEvent(message uint32, hook *kbdllHookStruct) (InputEvent, bool) {
	if hook == nil {
		return InputEvent{}, false
	}

	event := InputEvent{
		Kind:                   InputEventKey,
		Time:                   time.Now(),
		Injected:               hook.Flags&llkhfInjected != 0,
		LowerIntegrityInjected: hook.Flags&llkhfLowerIntegrityInjected != 0,
		ExtraInfo:              hook.DwExtraInfo,
	}
	event.Keyboard = KeyboardInputEvent{
		Key:      Key(hook.VKCode),
		ScanCode: uint16(hook.ScanCode),
		Extended: hook.Flags&llkhfExtended != 0,
		AltDown:  hook.Flags&llkhfAltDown != 0,
	}

	switch message {
	case wmKeyDown, wmSysKeyDown:
		event.Keyboard.State = Down
	case wmKeyUp, wmSysKeyUp:
		event.Keyboard.State = Up
	default:
		return InputEvent{}, false
	}

	return event, true
}

func xMouseButton(mouseData uint32) MouseButton {
	switch uint16(mouseData >> 16) {
	case xbutton2:
		return ButtonX2
	default:
		return ButtonX1
	}
}

func mouseDataHighWord(mouseData uint32) int {
	return int(int16(mouseData >> 16))
}

type msllHookStruct struct {
	Pt          winPoint
	MouseData   uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type kbdllHookStruct struct {
	VKCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

type rawInputDevice struct {
	UsagePage  uint16
	Usage      uint16
	Flags      uint32
	HwndTarget uintptr
}

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type rawInputHeader struct {
	Type    uint32
	Size    uint32
	HDevice uintptr
	WParam  uintptr
}

type rawMouse struct {
	Flags            uint16
	_                uint16
	ButtonFlags      uint16
	ButtonData       uint16
	RawButtons       uint32
	LastX            int32
	LastY            int32
	ExtraInformation uint32
}

type rawKeyboard struct {
	MakeCode         uint16
	Flags            uint16
	Reserved         uint16
	VKey             uint16
	Message          uint32
	ExtraInformation uint32
}
