//go:build windows

package makc

import (
	"context"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	whMouseLL    = 14
	whKeyboardLL = 13

	wmQuit = 0x0012

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
)

func (b *winBackend) ListenInput(ctx context.Context, opts ListenOptions) (*Listener, error) {
	opts = normalizeListenOptions(opts)
	ctx, cancel := context.WithCancel(ctx)

	events := make(chan InputEvent, opts.Buffer)
	done := make(chan error, 1)
	ready := make(chan error, 1)

	go b.runInputListener(ctx, opts, events, ready, done)

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
		}, nil
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

func (b *winBackend) runInputListener(ctx context.Context, opts ListenOptions, events chan<- InputEvent, ready chan<- error, done chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(events)

	threadID := windows.GetCurrentThreadId()
	callbacks := make([]uintptr, 0, 2)
	hooks := make([]uintptr, 0, 2)
	var stopped bool
	var err error

	emit := func(event InputEvent) {
		markOwnInputEvent(&event, b.inputTag)
		if !prepareInputEvent(&event, opts) {
			return
		}
		select {
		case events <- event:
		default:
		}
	}

	if opts.Mask&ListenMouse != 0 {
		cb := windows.NewCallback(func(nCode int, wParam uintptr, lParam *msllHookStruct) uintptr {
			if nCode >= 0 {
				if event, ok := mouseHookEvent(uint32(wParam), lParam); ok {
					emit(event)
				}
			}
			return b.api.callNextHookEx(0, int32(nCode), wParam, uintptr(unsafe.Pointer(lParam)))
		})
		callbacks = append(callbacks, cb)
		hook := b.api.setWindowsHookEx(whMouseLL, cb, 0, 0)
		if hook == 0 {
			ready <- fmt.Errorf("makc: SetWindowsHookExW(WH_MOUSE_LL) failed: %w", lastWindowsError())
			done <- nil
			return
		}
		hooks = append(hooks, hook)
	}

	if opts.Mask&ListenKeyboard != 0 {
		cb := windows.NewCallback(func(nCode int, wParam uintptr, lParam *kbdllHookStruct) uintptr {
			if nCode >= 0 {
				if event, ok := keyboardHookEvent(uint32(wParam), lParam); ok {
					emit(event)
				}
			}
			return b.api.callNextHookEx(0, int32(nCode), wParam, uintptr(unsafe.Pointer(lParam)))
		})
		callbacks = append(callbacks, cb)
		hook := b.api.setWindowsHookEx(whKeyboardLL, cb, 0, 0)
		if hook == 0 {
			unhookAll(b.api, hooks)
			ready <- fmt.Errorf("makc: SetWindowsHookExW(WH_KEYBOARD_LL) failed: %w", lastWindowsError())
			done <- nil
			return
		}
		hooks = append(hooks, hook)
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

	var msg winMsg
	for {
		result := b.api.getMessage(&msg, 0, 0, 0)
		if result == -1 {
			err = fmt.Errorf("makc: GetMessageW failed: %w", lastWindowsError())
			break
		}
		if result == 0 {
			stopped = true
			break
		}
	}

	unhookAll(b.api, hooks)
	runtime.KeepAlive(callbacks)

	if stopped {
		done <- nil
		return
	}
	done <- err
}

func unhookAll(api *winAPI, hooks []uintptr) {
	for i := len(hooks) - 1; i >= 0; i-- {
		if hooks[i] != 0 {
			api.unhookWindowsHookEx(hooks[i])
		}
	}
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
