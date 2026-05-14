//go:build linux

package makc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	linuxEvSyn = 0x00
	linuxEvKey = 0x01
	linuxEvRel = 0x02

	linuxSynReport = 0x00

	linuxRelX      = 0x00
	linuxRelY      = 0x01
	linuxRelHWheel = 0x06
	linuxRelWheel  = 0x08

	linuxBtnLeft   = 0x110
	linuxBtnRight  = 0x111
	linuxBtnMiddle = 0x112
	linuxBtnSide   = 0x113
	linuxBtnExtra  = 0x114

	linuxBusUSB = 0x03
)

type linuxBackend struct {
	device            *uinputDevice
	x11               *linuxX11Display
	mouseInjection    MouseInjectionBackend
	keyboardInjection KeyboardInjectionBackend

	// stateMu guards the lazily-opened evdev fd cache used by KeyState
	// and MouseButtonState. Without the cache, every state query was
	// glob+open+ioctl+close across every /dev/input/event* — easily
	// 100+ syscalls per call, fatal for any polling workload.
	stateMu      sync.Mutex
	stateDevices []*linuxEvdevDevice
	stateLoaded  bool
}

func newSystemBackend(cfg config) (systemBackend, error) {
	mouseInjection := cfg.mouseInjection
	switch mouseInjection {
	case MouseInjectionAuto:
		mouseInjection = MouseInjectionUInput
	case MouseInjectionUInput:
	case MouseInjectionSendInput, MouseInjectionInjectMouseInput:
		return nil, unsupported("Win32 mouse injection backends are only available on Windows")
	case MouseInjectionCGEvent:
		return nil, unsupported("CGEvent mouse injection is only available on macOS")
	default:
		return nil, fmt.Errorf("makc: unknown mouse injection backend %d", cfg.mouseInjection)
	}

	keyboardInjection := cfg.keyboardInjection
	switch keyboardInjection {
	case KeyboardInjectionAuto:
		keyboardInjection = KeyboardInjectionUInput
	case KeyboardInjectionUInput:
	case KeyboardInjectionSendInput, KeyboardInjectionInjectKeyboardInput:
		return nil, unsupported("Win32 keyboard injection backends are only available on Windows")
	case KeyboardInjectionCGEvent:
		return nil, unsupported("CGEvent keyboard injection is only available on macOS")
	default:
		return nil, fmt.Errorf("makc: unknown keyboard injection backend %d", cfg.keyboardInjection)
	}

	device, err := newUInputDevice()
	if err != nil {
		return nil, err
	}

	x11, _ := newLinuxX11Display()

	return &linuxBackend{
		device:            device,
		x11:               x11,
		mouseInjection:    mouseInjection,
		keyboardInjection: keyboardInjection,
	}, nil
}

func (b *linuxBackend) Close() error {
	if b == nil {
		return nil
	}
	b.stateMu.Lock()
	closeLinuxEvdevDevices(b.stateDevices)
	b.stateDevices = nil
	b.stateLoaded = false
	b.stateMu.Unlock()
	return errors.Join(
		b.device.Close(),
		b.x11.Close(),
	)
}

func (b *linuxBackend) MouseInjection() MouseInjectionBackend {
	return b.mouseInjection
}

func (b *linuxBackend) KeyboardInjection() KeyboardInjectionBackend {
	return b.keyboardInjection
}

func (b *linuxBackend) InputTag() uintptr {
	return 0
}

func (b *linuxBackend) ScreenSize(ctx context.Context) (Point, error) {
	if b.x11 == nil {
		return Point{}, unsupported("linux screen size requires an X11 DISPLAY")
	}
	return b.x11.screenSize(ctx)
}

func (b *linuxBackend) CursorPos(ctx context.Context) (Point, error) {
	if b.x11 == nil {
		return Point{}, unsupported("linux cursor position requires an X11 DISPLAY")
	}
	return b.x11.cursorPos(ctx)
}

func (b *linuxBackend) MouseButtonState(ctx context.Context, button MouseButton) (State, error) {
	code, err := linuxMouseButton(button)
	if err != nil {
		return Up, err
	}
	return b.evdevKeyState(ctx, code)
}

func (b *linuxBackend) MoveMouse(ctx context.Context, move MouseMove) error {
	return b.InjectMouse(ctx, []MouseEvent{MouseMoveEvent(move)})
}

func (b *linuxBackend) SetMouseButton(ctx context.Context, button MouseButton, state State) error {
	return b.InjectMouse(ctx, []MouseEvent{MouseButtonEvent(button, state)})
}

func (b *linuxBackend) InjectMouse(ctx context.Context, events []MouseEvent) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	for _, event := range events {
		if err := checkContext(ctx); err != nil {
			return err
		}
		switch event.Kind {
		case MouseEventMove:
			if !event.Move.Relative {
				if b.x11 == nil {
					return unsupported("linux absolute mouse movement requires an X11 DISPLAY")
				}
				if err := b.x11.movePointer(ctx, Point{X: event.Move.X, Y: event.Move.Y}); err != nil {
					return err
				}
				continue
			}
			if err := b.device.emitRelMove(event.Move.X, event.Move.Y); err != nil {
				return err
			}
		case MouseEventButton:
			code, err := linuxMouseButton(event.Button)
			if err != nil {
				return err
			}
			if !event.State.valid() {
				return errors.New("makc: mouse button state is unknown")
			}
			if err := b.device.emitKey(code, event.State); err != nil {
				return err
			}
		case MouseEventWheel:
			if err := b.device.emitRel(linuxRelWheel, linuxWheelClicks(event.Delta)); err != nil {
				return err
			}
		case MouseEventHWheel:
			if err := b.device.emitRel(linuxRelHWheel, linuxWheelClicks(event.Delta)); err != nil {
				return err
			}
		default:
			return unsupported("unknown mouse event")
		}
	}
	return nil
}

func (b *linuxBackend) KeyState(ctx context.Context, key Key) (State, error) {
	code, err := linuxKeyCode(key)
	if err != nil {
		return Up, err
	}
	return b.evdevKeyState(ctx, code)
}

func (b *linuxBackend) SetKey(ctx context.Context, key Key, state State) error {
	return b.InjectKeyboard(ctx, []KeyboardEvent{KeyEvent(key, state)})
}

func (b *linuxBackend) InjectKeyboard(ctx context.Context, events []KeyboardEvent) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	for _, event := range events {
		if err := checkContext(ctx); err != nil {
			return err
		}
		switch event.Kind {
		case KeyboardEventKey:
			code, err := linuxKeyCode(event.Key)
			if err != nil {
				return err
			}
			if err := b.device.emitKey(code, event.State); err != nil {
				return err
			}
		case KeyboardEventScanCode:
			if event.ScanCode == 0 {
				return errors.New("makc: scan code is unknown")
			}
			if err := b.device.emitKey(uint16(event.ScanCode), event.State); err != nil {
				return err
			}
		case KeyboardEventText:
			return unsupported("linux uinput backend does not support Unicode text injection")
		default:
			return unsupported("unknown keyboard event")
		}
	}
	return nil
}

func (b *linuxBackend) ListenInput(ctx context.Context, opts ListenOptions) (*Listener, error) {
	opts = normalizeListenOptions(opts)
	switch opts.Backend {
	case ListenBackendAuto, ListenBackendEvdev:
		return linuxListenEvdev(ctx, opts)
	case ListenBackendLowLevelHook:
		return nil, unsupported("low-level hook listening is only available on Windows")
	case ListenBackendRawInput:
		return nil, unsupported("Raw Input listening is only available on Windows")
	default:
		return nil, unsupported("unknown listen backend")
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

func linuxMouseButton(button MouseButton) (uint16, error) {
	switch button {
	case ButtonLeft:
		return linuxBtnLeft, nil
	case ButtonRight:
		return linuxBtnRight, nil
	case ButtonMiddle:
		return linuxBtnMiddle, nil
	case ButtonX1:
		return linuxBtnSide, nil
	case ButtonX2:
		return linuxBtnExtra, nil
	default:
		return 0, fmt.Errorf("makc: unknown mouse button %d", button)
	}
}

func linuxWheelClicks(delta int) int32 {
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

func linuxKeyCode(key Key) (uint16, error) {
	code, ok := linuxKeyCodes[key]
	if !ok {
		return 0, fmt.Errorf("makc: key %s is not supported on Linux uinput", key)
	}
	return code, nil
}

var linuxKeyCodes = map[Key]uint16{
	KeyEscape: 1,
	Key1:      2, Key2: 3, Key3: 4, Key4: 5, Key5: 6, Key6: 7, Key7: 8, Key8: 9, Key9: 10, Key0: 11,
	KeyMinus: 12, KeyEquals: 13, KeyBackspace: 14, KeyTab: 15,
	KeyQ: 16, KeyW: 17, KeyE: 18, KeyR: 19, KeyT: 20, KeyY: 21, KeyU: 22, KeyI: 23, KeyO: 24, KeyP: 25,
	KeyLeftSquareBracket: 26, KeyRightSquareBracket: 27, KeyEnter: 28,
	KeyControl: 29, KeyLeftControl: 29,
	KeyA: 30, KeyS: 31, KeyD: 32, KeyF: 33, KeyG: 34, KeyH: 35, KeyJ: 36, KeyK: 37, KeyL: 38,
	KeySemicolon: 39, KeySingleQuote: 40, KeyBackQuote: 41, KeyShift: 42, KeyLeftShift: 42,
	KeyBackslash: 43, KeyZ: 44, KeyX: 45, KeyC: 46, KeyV: 47, KeyB: 48, KeyN: 49, KeyM: 50,
	KeyComma: 51, KeyDot: 52, KeySlash: 53, KeyRightShift: 54,
	KeyMultiply: 55, KeyAlt: 56, KeyLeftAlt: 56, KeySpace: 57, KeyCapsLock: 58,
	KeyF1: 59, KeyF2: 60, KeyF3: 61, KeyF4: 62, KeyF5: 63, KeyF6: 64, KeyF7: 65, KeyF8: 66, KeyF9: 67, KeyF10: 68,
	KeyNumLock: 69, KeyScrollLock: 70,
	KeyNumpad7: 71, KeyNumpad8: 72, KeyNumpad9: 73, KeySubtract: 74,
	KeyNumpad4: 75, KeyNumpad5: 76, KeyNumpad6: 77, KeyAdd: 78,
	KeyNumpad1: 79, KeyNumpad2: 80, KeyNumpad3: 81, KeyNumpad0: 82, KeyDecimal: 83,
	KeyF11: 87, KeyF12: 88, KeyRightControl: 97, KeyDivide: 98,
	KeyRightAlt: 100, KeyHome: 102, KeyUp: 103, KeyPageUp: 104, KeyLeft: 105, KeyRight: 106,
	KeyEnd: 107, KeyDown: 108, KeyPageDown: 109, KeyInsert: 110, KeyDelete: 111,
	KeyPause: 119, KeyLeftWindows: 125, KeyRightWindows: 126, KeyApps: 127,
}

type uinputDevice struct {
	fd      int
	created bool
}

func newUInputDevice() (*uinputDevice, error) {
	fd, err := openUInput()
	if err != nil {
		return nil, err
	}
	device := &uinputDevice{fd: fd}
	if err := device.setup(); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	return device, nil
}

func openUInput() (int, error) {
	paths := []string{"/dev/uinput", "/dev/input/uinput", "/dev/misc/uinput"}
	var last error
	for _, path := range paths {
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err == nil {
			return fd, nil
		}
		last = fmt.Errorf("%s: %w", path, err)
		if !errors.Is(err, unix.ENOENT) {
			return -1, fmt.Errorf("makc: open Linux uinput device: %w", last)
		}
	}
	if last == nil {
		last = os.ErrNotExist
	}
	return -1, fmt.Errorf("%w: linux uinput device is not available: %v", ErrUnsupported, last)
}

func (d *uinputDevice) setup() error {
	for _, eventType := range []uint16{linuxEvKey, linuxEvRel} {
		if err := linuxIoctl(d.fd, uiSetEvBit, uintptr(eventType)); err != nil {
			return fmt.Errorf("makc: UI_SET_EVBIT(%d): %w", eventType, err)
		}
	}
	for _, rel := range []uint16{linuxRelX, linuxRelY, linuxRelWheel, linuxRelHWheel} {
		if err := linuxIoctl(d.fd, uiSetRelBit, uintptr(rel)); err != nil {
			return fmt.Errorf("makc: UI_SET_RELBIT(%d): %w", rel, err)
		}
	}
	for _, key := range linuxSupportedKeys() {
		if err := linuxIoctl(d.fd, uiSetKeyBit, uintptr(key)); err != nil {
			return fmt.Errorf("makc: UI_SET_KEYBIT(%d): %w", key, err)
		}
	}

	userDev := uinputUserDev{
		ID: inputID{
			BusType: linuxBusUSB,
			Vendor:  0x4d41,
			Product: 0x4b43,
			Version: 1,
		},
	}
	copy(userDev.Name[:], "makc uinput")
	if err := writeStruct(d.fd, &userDev); err != nil {
		return fmt.Errorf("makc: write uinput_user_dev: %w", err)
	}
	if err := linuxIoctl(d.fd, uiDevCreate, 0); err != nil {
		return fmt.Errorf("makc: UI_DEV_CREATE: %w", err)
	}
	d.created = true
	// After UI_DEV_CREATE the kernel asynchronously hands the device to
	// udev, which creates /dev/input/eventN. The previous code slept a
	// fixed 20ms — wrong both ways: too short under load (first events
	// would race ahead of device readiness and silently disappear) and
	// too long on a quiet system. Use UI_GET_SYSNAME (kernel ≥ 3.15) to
	// learn the sysfs name, then poll for the event node to appear with
	// a bounded deadline. Older kernels fall back to the legacy sleep.
	if err := waitUInputReady(d.fd, 250*time.Millisecond); err != nil {
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// waitUInputReady blocks until the event device backing fd is visible in
// sysfs, or until deadline elapses. Returns the underlying ioctl error
// when UI_GET_SYSNAME is not available (e.g. pre-3.15 kernels) so the
// caller can fall back gracefully.
func waitUInputReady(fd int, deadline time.Duration) error {
	const sysnameMaxLen = 64
	buf := make([]byte, sysnameMaxLen)
	if err := linuxIoctl(fd, uiGetSysname(sysnameMaxLen), uintptr(unsafe.Pointer(&buf[0]))); err != nil {
		return err
	}
	n := bytes.IndexByte(buf, 0)
	if n < 0 {
		n = len(buf)
	}
	sysname := string(buf[:n])
	if sysname == "" {
		return errors.New("makc: empty UI_GET_SYSNAME result")
	}
	pattern := filepath.Join("/sys/devices/virtual/input", sysname, "event*")
	end := time.Now().Add(deadline)
	for {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return nil
		}
		if time.Now().After(end) {
			// Best-effort: device created but sysfs entry not yet visible.
			// Returning nil avoids failing device init for this — the
			// fallback sleep already happens at the call site.
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func linuxSupportedKeys() []uint16 {
	seen := map[uint16]bool{
		linuxBtnLeft:   true,
		linuxBtnRight:  true,
		linuxBtnMiddle: true,
		linuxBtnSide:   true,
		linuxBtnExtra:  true,
	}
	keys := []uint16{linuxBtnLeft, linuxBtnRight, linuxBtnMiddle, linuxBtnSide, linuxBtnExtra}
	for _, code := range linuxKeyCodes {
		if seen[code] {
			continue
		}
		seen[code] = true
		keys = append(keys, code)
	}
	return keys
}

func (d *uinputDevice) Close() error {
	if d == nil || d.fd < 0 {
		return nil
	}
	var err error
	if d.created {
		err = linuxIoctl(d.fd, uiDevDestroy, 0)
	}
	if closeErr := unix.Close(d.fd); err == nil {
		err = closeErr
	}
	d.fd = -1
	d.created = false
	return err
}

func (d *uinputDevice) emitRelMove(dx, dy int) error {
	if dx != 0 {
		if err := d.writeEvent(linuxEvRel, linuxRelX, int32(dx)); err != nil {
			return err
		}
	}
	if dy != 0 {
		if err := d.writeEvent(linuxEvRel, linuxRelY, int32(dy)); err != nil {
			return err
		}
	}
	return d.syn()
}

func (d *uinputDevice) emitRel(code uint16, value int32) error {
	if value == 0 {
		return nil
	}
	if err := d.writeEvent(linuxEvRel, code, value); err != nil {
		return err
	}
	return d.syn()
}

func (d *uinputDevice) emitKey(code uint16, state State) error {
	if !state.valid() {
		return errors.New("makc: key state is unknown")
	}
	value := int32(0)
	if state == Down {
		value = 1
	}
	if err := d.writeEvent(linuxEvKey, code, value); err != nil {
		return err
	}
	return d.syn()
}

func (d *uinputDevice) syn() error {
	return d.writeEvent(linuxEvSyn, linuxSynReport, 0)
}

func (d *uinputDevice) writeEvent(eventType uint16, code uint16, value int32) error {
	event := linuxInputEvent{
		Type:  eventType,
		Code:  code,
		Value: value,
	}
	if err := writeStruct(d.fd, &event); err != nil {
		return fmt.Errorf("makc: write uinput event type=%d code=%d value=%d: %w", eventType, code, value, err)
	}
	return nil
}

func writeStruct[T any](fd int, value *T) error {
	size := unsafe.Sizeof(*value)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(value)), size)
	for len(buf) > 0 {
		n, err := unix.Write(fd, buf)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		buf = buf[n:]
	}
	return nil
}

func linuxIoctl(fd int, req uintptr, arg uintptr) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

func uiIO(nr uintptr) uintptr {
	return ('U' << linuxIOCTypeShift) | (nr << linuxIOCNRShift)
}

func uiIOW(nr uintptr, size uintptr) uintptr {
	return (linuxIOCWrite << linuxIOCDirShift) |
		(size << linuxIOCSizeShift) |
		('U' << linuxIOCTypeShift) |
		(nr << linuxIOCNRShift)
}

const (
	linuxIOCNRShift   = 0
	linuxIOCTypeShift = 8
	linuxIOCSizeShift = 16
	linuxIOCDirShift  = 30
	linuxIOCWrite     = 1
	linuxIOCRead      = 2

	uiDevCreate  = ('U' << linuxIOCTypeShift) | (1 << linuxIOCNRShift)
	uiDevDestroy = ('U' << linuxIOCTypeShift) | (2 << linuxIOCNRShift)
	uiSetEvBit   = (linuxIOCWrite << linuxIOCDirShift) | (unsafe.Sizeof(int32(0)) << linuxIOCSizeShift) | ('U' << linuxIOCTypeShift) | (100 << linuxIOCNRShift)
	uiSetKeyBit  = (linuxIOCWrite << linuxIOCDirShift) | (unsafe.Sizeof(int32(0)) << linuxIOCSizeShift) | ('U' << linuxIOCTypeShift) | (101 << linuxIOCNRShift)
	uiSetRelBit  = (linuxIOCWrite << linuxIOCDirShift) | (unsafe.Sizeof(int32(0)) << linuxIOCSizeShift) | ('U' << linuxIOCTypeShift) | (102 << linuxIOCNRShift)
)

// uiGetSysname encodes UI_GET_SYSNAME(len) — kernel ≥ 3.15. The macro is
// _IOC(_IOC_READ, 'U', 44, len), matching linuxIOCR with NR=44.
func uiGetSysname(size uintptr) uintptr {
	return linuxIOCR(44, size)
}

type linuxInputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

type inputID struct {
	BusType uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

type uinputUserDev struct {
	Name         [80]byte
	ID           inputID
	FFEffectsMax uint32
	AbsMax       [64]int32
	AbsMin       [64]int32
	AbsFuzz      [64]int32
	AbsFlat      [64]int32
}
