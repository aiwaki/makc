//go:build darwin

package makc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/bits"
	"sync"
	"unicode/utf16"

	"github.com/ebitengine/purego"
)

const (
	cgEventLeftMouseDown     = 1
	cgEventLeftMouseUp       = 2
	cgEventRightMouseDown    = 3
	cgEventRightMouseUp      = 4
	cgEventMouseMoved        = 5
	cgEventLeftMouseDragged  = 6
	cgEventRightMouseDragged = 7
	cgEventOtherMouseDown    = 25
	cgEventOtherMouseUp      = 26
	cgEventOtherMouseDragged = 27

	cgEventTapHID = 0

	cgEventSourceStateHIDSystem = 1

	cgMouseEventDeltaX = 4
	cgMouseEventDeltaY = 5

	cgScrollEventUnitLine = 1

	cgMouseButtonLeft   = 0
	cgMouseButtonRight  = 1
	cgMouseButtonCenter = 2
)

type darwinBackend struct {
	api               *darwinAPI
	mouseInjection    MouseInjectionBackend
	keyboardInjection KeyboardInjectionBackend

	mu                 sync.Mutex
	pressedMouseButton uint32
}

func newSystemBackend(cfg config) (systemBackend, error) {
	api, err := newDarwinAPI()
	if err != nil {
		return nil, err
	}

	mouseInjection := cfg.mouseInjection
	switch mouseInjection {
	case MouseInjectionAuto:
		mouseInjection = MouseInjectionCGEvent
	case MouseInjectionCGEvent:
	case MouseInjectionSendInput, MouseInjectionInjectMouseInput:
		return nil, unsupported("Win32 mouse injection backends are only available on Windows")
	case MouseInjectionUInput:
		return nil, unsupported("uinput mouse injection is only available on Linux")
	default:
		return nil, fmt.Errorf("makc: unknown mouse injection backend %d", cfg.mouseInjection)
	}

	keyboardInjection := cfg.keyboardInjection
	switch keyboardInjection {
	case KeyboardInjectionAuto:
		keyboardInjection = KeyboardInjectionCGEvent
	case KeyboardInjectionCGEvent:
	case KeyboardInjectionSendInput, KeyboardInjectionInjectKeyboardInput:
		return nil, unsupported("Win32 keyboard injection backends are only available on Windows")
	case KeyboardInjectionUInput:
		return nil, unsupported("uinput keyboard injection is only available on Linux")
	default:
		return nil, fmt.Errorf("makc: unknown keyboard injection backend %d", cfg.keyboardInjection)
	}

	return &darwinBackend{
		api:               api,
		mouseInjection:    mouseInjection,
		keyboardInjection: keyboardInjection,
	}, nil
}

func (b *darwinBackend) Close() error {
	return nil
}

func (b *darwinBackend) MouseInjection() MouseInjectionBackend {
	return b.mouseInjection
}

func (b *darwinBackend) KeyboardInjection() KeyboardInjectionBackend {
	return b.keyboardInjection
}

func (b *darwinBackend) InputTag() uintptr {
	return 0
}

func (b *darwinBackend) ScreenSize(ctx context.Context) (Point, error) {
	if err := checkContext(ctx); err != nil {
		return Point{}, err
	}
	display := b.api.cgMainDisplayID()
	return Point{
		X: int(b.api.cgDisplayPixelsWide(display)),
		Y: int(b.api.cgDisplayPixelsHigh(display)),
	}, nil
}

func (b *darwinBackend) CursorPos(ctx context.Context) (Point, error) {
	if err := checkContext(ctx); err != nil {
		return Point{}, err
	}
	location, err := b.cursorLocation()
	if err != nil {
		return Point{}, err
	}
	return Point{X: int(math.Round(location.X)), Y: int(math.Round(location.Y))}, nil
}

func (b *darwinBackend) MouseButtonState(ctx context.Context, button MouseButton) (State, error) {
	if err := checkContext(ctx); err != nil {
		return Up, err
	}
	cgButton, err := darwinMouseButton(button)
	if err != nil {
		return Up, err
	}
	if b.api.cgEventSourceButtonState(cgEventSourceStateHIDSystem, cgButton) {
		return Down, nil
	}
	return Up, nil
}

func (b *darwinBackend) MoveMouse(ctx context.Context, move MouseMove) error {
	return b.InjectMouse(ctx, []MouseEvent{MouseMoveEvent(move)})
}

func (b *darwinBackend) SetMouseButton(ctx context.Context, button MouseButton, state State) error {
	return b.InjectMouse(ctx, []MouseEvent{MouseButtonEvent(button, state)})
}

func (b *darwinBackend) InjectMouse(ctx context.Context, events []MouseEvent) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	if err := b.requireAccessibility("mouse injection"); err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, event := range events {
		if err := checkContext(ctx); err != nil {
			return err
		}
		switch event.Kind {
		case MouseEventMove:
			if err := b.postMouseMove(event.Move); err != nil {
				return err
			}
		case MouseEventButton:
			if err := b.postMouseButton(event.Button, event.State); err != nil {
				return err
			}
		case MouseEventWheel:
			if err := b.postScroll(darwinWheelClicks(event.Delta), 0); err != nil {
				return err
			}
		case MouseEventHWheel:
			if err := b.postScroll(0, darwinWheelClicks(event.Delta)); err != nil {
				return err
			}
		default:
			return unsupported("unknown mouse event")
		}
	}
	return nil
}

func (b *darwinBackend) KeyState(ctx context.Context, key Key) (State, error) {
	if err := checkContext(ctx); err != nil {
		return Up, err
	}
	keyCode, err := darwinKeyCode(key)
	if err != nil {
		return Up, err
	}
	if b.api.cgEventSourceKeyState(cgEventSourceStateHIDSystem, keyCode) {
		return Down, nil
	}
	return Up, nil
}

func (b *darwinBackend) SetKey(ctx context.Context, key Key, state State) error {
	return b.InjectKeyboard(ctx, []KeyboardEvent{KeyEvent(key, state)})
}

func (b *darwinBackend) InjectKeyboard(ctx context.Context, events []KeyboardEvent) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	if err := b.requireAccessibility("keyboard injection"); err != nil {
		return err
	}

	for _, event := range events {
		if err := checkContext(ctx); err != nil {
			return err
		}
		switch event.Kind {
		case KeyboardEventKey:
			keyCode, err := darwinKeyCode(event.Key)
			if err != nil {
				return err
			}
			if err := b.postKeyboard(keyCode, event.State); err != nil {
				return err
			}
		case KeyboardEventScanCode:
			if err := b.postKeyboard(event.ScanCode, event.State); err != nil {
				return err
			}
		case KeyboardEventText:
			if err := b.postText(event.Text); err != nil {
				return err
			}
		default:
			return unsupported("unknown keyboard event")
		}
	}
	return nil
}

func (b *darwinBackend) ListenInput(context.Context, ListenOptions) (*Listener, error) {
	return nil, unsupported("macOS input listening is not implemented")
}

func (b *darwinBackend) requireAccessibility(operation string) error {
	if b.api.axIsProcessTrusted() {
		return nil
	}
	return fmt.Errorf("makc: macOS Accessibility permission is required for %s", operation)
}

func (b *darwinBackend) cursorLocation() (cgPoint, error) {
	event := b.api.cgEventCreate(0)
	if event == 0 {
		return cgPoint{}, errors.New("makc: CGEventCreate failed")
	}
	defer b.api.cfRelease(event)
	return b.api.cgEventGetLocation(event), nil
}

func (b *darwinBackend) postMouseMove(move MouseMove) error {
	current, err := b.cursorLocation()
	if err != nil {
		return err
	}
	location := cgPoint{X: float64(move.X), Y: float64(move.Y)}
	if move.Relative {
		location.X += current.X
		location.Y += current.Y
	}
	deltaX := int64(math.Round(location.X - current.X))
	deltaY := int64(math.Round(location.Y - current.Y))

	eventType := uint32(cgEventMouseMoved)
	button := uint32(cgMouseButtonLeft)
	if dragButton, ok := b.dragMouseButton(); ok {
		eventType = darwinMouseDraggedEventType(dragButton)
		button = dragButton
	}
	return b.postMouseEvent(eventType, location, button, deltaX, deltaY)
}

func (b *darwinBackend) postMouseButton(button MouseButton, state State) error {
	if !state.valid() {
		return errors.New("makc: mouse button state is unknown")
	}
	cgButton, err := darwinMouseButton(button)
	if err != nil {
		return err
	}
	location, err := b.cursorLocation()
	if err != nil {
		return err
	}
	eventType := darwinMouseButtonEventType(cgButton, state)
	if err := b.postMouseEvent(eventType, location, cgButton, 0, 0); err != nil {
		return err
	}
	b.setPressedMouseButton(cgButton, state)
	return nil
}

func (b *darwinBackend) postMouseEvent(eventType uint32, location cgPoint, button uint32, deltaX int64, deltaY int64) error {
	source, err := b.eventSource()
	if err != nil {
		return err
	}
	defer b.api.cfRelease(source)

	event := b.api.cgEventCreateMouseEvent(source, eventType, location, button)
	if event == 0 {
		return errors.New("makc: CGEventCreateMouseEvent failed")
	}
	defer b.api.cfRelease(event)
	if deltaX != 0 || deltaY != 0 {
		b.api.cgEventSetIntegerValueField(event, cgMouseEventDeltaX, deltaX)
		b.api.cgEventSetIntegerValueField(event, cgMouseEventDeltaY, deltaY)
	}
	b.api.cgEventPost(cgEventTapHID, event)
	return nil
}

func (b *darwinBackend) postScroll(vertical, horizontal int32) error {
	if vertical == 0 && horizontal == 0 {
		return nil
	}
	source, err := b.eventSource()
	if err != nil {
		return err
	}
	defer b.api.cfRelease(source)

	event := b.api.cgEventCreateScrollWheelEvent(source, cgScrollEventUnitLine, 2, vertical, horizontal)
	if event == 0 {
		return errors.New("makc: CGEventCreateScrollWheelEvent failed")
	}
	defer b.api.cfRelease(event)
	b.api.cgEventPost(cgEventTapHID, event)
	return nil
}

func (b *darwinBackend) postKeyboard(keyCode uint16, state State) error {
	if !state.valid() {
		return errors.New("makc: key state is unknown")
	}
	source, err := b.eventSource()
	if err != nil {
		return err
	}
	defer b.api.cfRelease(source)

	event := b.api.cgEventCreateKeyboardEvent(source, keyCode, state == Down)
	if event == 0 {
		return errors.New("makc: CGEventCreateKeyboardEvent failed")
	}
	defer b.api.cfRelease(event)
	b.api.cgEventPost(cgEventTapHID, event)
	return nil
}

func (b *darwinBackend) postText(text string) error {
	for _, r := range text {
		units := utf16.Encode([]rune{r})
		if len(units) == 0 {
			continue
		}
		if err := b.postUnicode(units, Down); err != nil {
			return err
		}
		if err := b.postUnicode(units, Up); err != nil {
			return err
		}
	}
	return nil
}

func (b *darwinBackend) postUnicode(units []uint16, state State) error {
	source, err := b.eventSource()
	if err != nil {
		return err
	}
	defer b.api.cfRelease(source)

	event := b.api.cgEventCreateKeyboardEvent(source, 0, state == Down)
	if event == 0 {
		return errors.New("makc: CGEventCreateKeyboardEvent(unicode) failed")
	}
	defer b.api.cfRelease(event)
	b.api.cgEventKeyboardSetUnicodeString(event, uintptr(len(units)), &units[0])
	b.api.cgEventPost(cgEventTapHID, event)
	return nil
}

func (b *darwinBackend) eventSource() (uintptr, error) {
	source := b.api.cgEventSourceCreate(cgEventSourceStateHIDSystem)
	if source == 0 {
		return 0, errors.New("makc: CGEventSourceCreate failed")
	}
	return source, nil
}

func (b *darwinBackend) dragMouseButton() (uint32, bool) {
	if b.pressedMouseButton != 0 {
		return uint32(bits.TrailingZeros32(b.pressedMouseButton)), true
	}
	for _, button := range []uint32{cgMouseButtonLeft, cgMouseButtonRight, cgMouseButtonCenter} {
		if b.api.cgEventSourceButtonState(cgEventSourceStateHIDSystem, button) {
			return button, true
		}
	}
	return 0, false
}

func (b *darwinBackend) setPressedMouseButton(button uint32, state State) {
	mask := uint32(1) << button
	if state == Down {
		b.pressedMouseButton |= mask
		return
	}
	b.pressedMouseButton &^= mask
}

func darwinMouseButton(button MouseButton) (uint32, error) {
	switch button {
	case ButtonLeft:
		return cgMouseButtonLeft, nil
	case ButtonRight:
		return cgMouseButtonRight, nil
	case ButtonMiddle:
		return cgMouseButtonCenter, nil
	case ButtonX1:
		return 3, nil
	case ButtonX2:
		return 4, nil
	default:
		return 0, fmt.Errorf("makc: unknown mouse button %d", button)
	}
}

func darwinMouseButtonEventType(button uint32, state State) uint32 {
	switch button {
	case cgMouseButtonLeft:
		if state == Down {
			return cgEventLeftMouseDown
		}
		return cgEventLeftMouseUp
	case cgMouseButtonRight:
		if state == Down {
			return cgEventRightMouseDown
		}
		return cgEventRightMouseUp
	default:
		if state == Down {
			return cgEventOtherMouseDown
		}
		return cgEventOtherMouseUp
	}
}

func darwinMouseDraggedEventType(button uint32) uint32 {
	switch button {
	case cgMouseButtonLeft:
		return cgEventLeftMouseDragged
	case cgMouseButtonRight:
		return cgEventRightMouseDragged
	default:
		return cgEventOtherMouseDragged
	}
}

func darwinWheelClicks(delta int) int32 {
	if delta == 0 {
		return 0
	}
	clicks := int32(math.Round(float64(delta) / float64(WheelDelta)))
	if clicks == 0 {
		if delta > 0 {
			return 1
		}
		return -1
	}
	return clicks
}

func darwinKeyCode(key Key) (uint16, error) {
	code, ok := darwinKeyCodes[key]
	if !ok {
		return 0, fmt.Errorf("makc: key %s is not supported on macOS", key)
	}
	return code, nil
}

var darwinKeyCodes = map[Key]uint16{
	KeyA: 0x00, KeyS: 0x01, KeyD: 0x02, KeyF: 0x03, KeyH: 0x04, KeyG: 0x05,
	KeyZ: 0x06, KeyX: 0x07, KeyC: 0x08, KeyV: 0x09, KeyB: 0x0B, KeyQ: 0x0C,
	KeyW: 0x0D, KeyE: 0x0E, KeyR: 0x0F, KeyY: 0x10, KeyT: 0x11,
	Key1: 0x12, Key2: 0x13, Key3: 0x14, Key4: 0x15, Key6: 0x16, Key5: 0x17,
	KeyEquals: 0x18, Key9: 0x19, Key7: 0x1A, KeyMinus: 0x1B, Key8: 0x1C, Key0: 0x1D,
	KeyRightSquareBracket: 0x1E, KeyO: 0x1F, KeyU: 0x20, KeyLeftSquareBracket: 0x21,
	KeyI: 0x22, KeyP: 0x23, KeyEnter: 0x24, KeyL: 0x25, KeyJ: 0x26,
	KeySingleQuote: 0x27, KeyK: 0x28, KeySemicolon: 0x29, KeyBackslash: 0x2A,
	KeyComma: 0x2B, KeySlash: 0x2C, KeyN: 0x2D, KeyM: 0x2E, KeyDot: 0x2F,
	KeyTab: 0x30, KeySpace: 0x31, KeyBackQuote: 0x32, KeyBackspace: 0x33, KeyEscape: 0x35,
	KeyRightWindows: 0x36, KeyLeftWindows: 0x37, KeyShift: 0x38, KeyLeftShift: 0x38,
	KeyCapsLock: 0x39, KeyAlt: 0x3A, KeyLeftAlt: 0x3A, KeyControl: 0x3B, KeyLeftControl: 0x3B,
	KeyRightShift: 0x3C, KeyRightAlt: 0x3D, KeyRightControl: 0x3E,
	KeyDecimal: 0x41, KeyMultiply: 0x43, KeyAdd: 0x45, KeyClear: 0x47,
	KeyDivide: 0x4B, KeySubtract: 0x4E,
	KeyNumpad0: 0x52, KeyNumpad1: 0x53, KeyNumpad2: 0x54, KeyNumpad3: 0x55, KeyNumpad4: 0x56,
	KeyNumpad5: 0x57, KeyNumpad6: 0x58, KeyNumpad7: 0x59, KeyNumpad8: 0x5B, KeyNumpad9: 0x5C,
	KeyF5: 0x60, KeyF6: 0x61, KeyF7: 0x62, KeyF3: 0x63, KeyF8: 0x64, KeyF9: 0x65,
	KeyF11: 0x67, KeyF10: 0x6D, KeyF12: 0x6F, KeyHelp: 0x72, KeyHome: 0x73,
	KeyPageUp: 0x74, KeyDelete: 0x75, KeyF4: 0x76, KeyEnd: 0x77, KeyF2: 0x78,
	KeyPageDown: 0x79, KeyF1: 0x7A, KeyLeft: 0x7B, KeyRight: 0x7C, KeyDown: 0x7D, KeyUp: 0x7E,
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

func newDarwinAPI() (*darwinAPI, error) {
	appServices, err := purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("makc: load ApplicationServices.framework: %w", err)
	}
	coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("makc: load CoreFoundation.framework: %w", err)
	}

	api := &darwinAPI{}
	if err := registerDarwinProc(appServices, &api.axIsProcessTrusted, "AXIsProcessTrusted"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgMainDisplayID, "CGMainDisplayID"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgDisplayPixelsWide, "CGDisplayPixelsWide"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgDisplayPixelsHigh, "CGDisplayPixelsHigh"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventCreate, "CGEventCreate"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventSourceCreate, "CGEventSourceCreate"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventGetLocation, "CGEventGetLocation"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventSetIntegerValueField, "CGEventSetIntegerValueField"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventSourceButtonState, "CGEventSourceButtonState"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventSourceKeyState, "CGEventSourceKeyState"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventCreateMouseEvent, "CGEventCreateMouseEvent"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventCreateScrollWheelEvent, "CGEventCreateScrollWheelEvent"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventCreateKeyboardEvent, "CGEventCreateKeyboardEvent"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventKeyboardSetUnicodeString, "CGEventKeyboardSetUnicodeString"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(appServices, &api.cgEventPost, "CGEventPost"); err != nil {
		return nil, err
	}
	if err := registerDarwinProc(coreFoundation, &api.cfRelease, "CFRelease"); err != nil {
		return nil, err
	}
	return api, nil
}

func registerDarwinProc(handle uintptr, fptr any, name string) error {
	proc, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("makc: load %s: %w", name, err)
	}
	purego.RegisterFunc(fptr, proc)
	return nil
}

type darwinAPI struct {
	axIsProcessTrusted func() bool

	cgMainDisplayID             func() uint32
	cgDisplayPixelsWide         func(uint32) uintptr
	cgDisplayPixelsHigh         func(uint32) uintptr
	cgEventCreate               func(uintptr) uintptr
	cgEventSourceCreate         func(uint32) uintptr
	cgEventGetLocation          func(uintptr) cgPoint
	cgEventSetIntegerValueField func(uintptr, uint32, int64)
	cgEventSourceButtonState    func(uint32, uint32) bool
	cgEventSourceKeyState       func(uint32, uint16) bool

	cgEventCreateMouseEvent         func(uintptr, uint32, cgPoint, uint32) uintptr
	cgEventCreateScrollWheelEvent   func(uintptr, uint32, uint32, int32, ...any) uintptr
	cgEventCreateKeyboardEvent      func(uintptr, uint16, bool) uintptr
	cgEventKeyboardSetUnicodeString func(uintptr, uintptr, *uint16)
	cgEventPost                     func(uint32, uintptr)

	cfRelease func(uintptr)
}

type cgPoint struct {
	X float64
	Y float64
}
