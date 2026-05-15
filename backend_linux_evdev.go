//go:build linux

package makc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	linuxEvdevNameMax = 256
	linuxKeyMax       = 0x2ff
	linuxKeyBitsLen   = linuxKeyMax/8 + 1

	linuxEvMax = 0x1f

	evIOCGNameNR = 0x06
	evIOCGKeyNR  = 0x18
	evIOCGBitNR  = 0x20
)

type linuxEvdevDevice struct {
	fd     int
	path   string
	name   string
	relX   int32
	relY   int32
	wheel  int32
	hwheel int32
}

// evdevKeyState answers a key/button state query using a cached set of
// evdev fds. The cache is opened lazily on first call and reused; on a
// per-device disconnect (ENODEV / ENXIO) the entry is dropped from the
// cache and not retried until backend re-init. Listener paths still open
// their own short-lived fd set — the cache here is exclusively for the
// state-poll path.
func (b *linuxBackend) evdevKeyState(ctx context.Context, code uint16) (State, error) {
	if err := checkContext(ctx); err != nil {
		return Up, err
	}

	b.stateMu.Lock()
	defer b.stateMu.Unlock()

	if !b.stateLoaded {
		devices, err := openLinuxEvdevDevices(ListenAll)
		if err != nil {
			return Up, err
		}
		b.stateDevices = devices
		b.stateLoaded = true
	}

	for i := 0; i < len(b.stateDevices); {
		if err := checkContext(ctx); err != nil {
			return Up, err
		}
		device := b.stateDevices[i]
		down, err := linuxEvdevKeyDown(device.fd, code)
		if err != nil {
			if errors.Is(err, unix.ENODEV) || errors.Is(err, unix.ENXIO) {
				_ = unix.Close(device.fd)
				device.fd = -1
				b.stateDevices = append(b.stateDevices[:i], b.stateDevices[i+1:]...)
				continue
			}
			return Up, fmt.Errorf("makc: EVIOCGKEY(%s): %w", device.path, err)
		}
		if down {
			return Down, nil
		}
		i++
	}
	return Up, nil
}

func linuxListenEvdev(ctx context.Context, opts ListenOptions) (*Listener, error) {
	ctx, cancel := context.WithCancel(ctx)
	devices, err := openLinuxEvdevDevices(opts.Mask)
	if err != nil {
		cancel()
		return nil, err
	}

	events := make(chan InputEvent, opts.Buffer)
	done := make(chan error, 1)
	stats := newListenerStats()
	go runLinuxEvdevListener(ctx, opts, stats, devices, events, done)

	return &Listener{
		Events: events,
		done:   done,
		cancel: cancel,
		stats:  stats,
	}, nil
}

func openLinuxEvdevDevices(mask ListenMask) ([]*linuxEvdevDevice, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, unsupported("linux evdev devices are not available")
	}

	var devices []*linuxEvdevDevice
	var permissionErr error
	var openErr error
	for _, path := range paths {
		fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) {
				permissionErr = fmt.Errorf("%s: %w", path, err)
				continue
			}
			openErr = fmt.Errorf("%s: %w", path, err)
			continue
		}

		name, _ := linuxEvdevName(fd)
		if !linuxEvdevMatchesMask(fd, mask) {
			_ = unix.Close(fd)
			continue
		}

		devices = append(devices, &linuxEvdevDevice{
			fd:   fd,
			path: path,
			name: name,
		})
	}

	if len(devices) != 0 {
		return devices, nil
	}
	if permissionErr != nil {
		return nil, fmt.Errorf("makc: open Linux evdev devices: %w", permissionErr)
	}
	if openErr != nil {
		return nil, fmt.Errorf("makc: open Linux evdev devices: %w", openErr)
	}
	return nil, unsupported("linux evdev devices matching listen mask are not available")
}

func closeLinuxEvdevDevices(devices []*linuxEvdevDevice) {
	for _, device := range devices {
		if device != nil && device.fd >= 0 {
			_ = unix.Close(device.fd)
			device.fd = -1
		}
	}
}

func linuxEvdevMatchesMask(fd int, mask ListenMask) bool {
	var bits [(linuxEvMax / 8) + 1]byte
	if err := linuxIoctl(fd, evIOCGBit(0, uintptr(len(bits))), uintptr(unsafe.Pointer(&bits[0]))); err != nil {
		return true
	}
	if mask&ListenMouse != 0 && (bitSet(bits[:], linuxEvRel) || bitSet(bits[:], linuxEvKey)) {
		return true
	}
	if mask&ListenKeyboard != 0 && bitSet(bits[:], linuxEvKey) {
		return true
	}
	return false
}

func linuxEvdevName(fd int) (string, error) {
	buf := make([]byte, linuxEvdevNameMax)
	if err := linuxIoctl(fd, evIOCGName(uintptr(len(buf))), uintptr(unsafe.Pointer(&buf[0]))); err != nil {
		return "", err
	}
	if n := bytes.IndexByte(buf, 0); n >= 0 {
		buf = buf[:n]
	}
	return string(buf), nil
}

func linuxEvdevKeyDown(fd int, code uint16) (bool, error) {
	var bits [linuxKeyBitsLen]byte
	if err := linuxIoctl(fd, evIOCGKey(uintptr(len(bits))), uintptr(unsafe.Pointer(&bits[0]))); err != nil {
		return false, err
	}
	return bitSet(bits[:], code), nil
}

func runLinuxEvdevListener(ctx context.Context, opts ListenOptions, stats *listenerStats, devices []*linuxEvdevDevice, events chan<- InputEvent, done chan<- error) {
	defer close(events)
	defer closeLinuxEvdevDevices(devices)

	pollFds := make([]unix.PollFd, len(devices))
	for i, device := range devices {
		pollFds[i] = unix.PollFd{
			Fd:     int32(device.fd),
			Events: unix.POLLIN | unix.POLLERR | unix.POLLHUP,
		}
	}

	for len(devices) != 0 {
		select {
		case <-ctx.Done():
			done <- nil
			return
		default:
		}

		n, err := unix.Poll(pollFds, 100)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			done <- fmt.Errorf("makc: poll Linux evdev devices: %w", err)
			return
		}
		if n == 0 {
			continue
		}

		for i := 0; i < len(devices); i++ {
			revents := pollFds[i].Revents
			if revents == 0 {
				continue
			}
			if revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
				_ = unix.Close(devices[i].fd)
				devices = append(devices[:i], devices[i+1:]...)
				pollFds = append(pollFds[:i], pollFds[i+1:]...)
				i--
				continue
			}
			if revents&unix.POLLIN == 0 {
				continue
			}
			if err := readLinuxEvdevEvents(devices[i], opts, stats, events); err != nil {
				done <- err
				return
			}
		}
	}

	// Loop fell out because every device was pruned. Distinguish a
	// graceful shutdown (ctx cancelled) from a silent disconnect — the
	// latter previously returned nil and the caller had no way to tell
	// the listener had died.
	if ctx.Err() != nil {
		done <- nil
		return
	}
	done <- errors.New("makc: every evdev device disconnected; reopen the client to recover")
}

func readLinuxEvdevEvents(device *linuxEvdevDevice, opts ListenOptions, stats *listenerStats, out chan<- InputEvent) error {
	for {
		event, err := readLinuxInputEvent(device.fd)
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				return nil
			}
			if errors.Is(err, io.EOF) || errors.Is(err, unix.ENODEV) || errors.Is(err, unix.ENXIO) {
				return nil
			}
			return fmt.Errorf("makc: read Linux evdev device %s: %w", device.path, err)
		}
		linuxEvdevEvent(device, event, opts, stats, out)
	}
}

func readLinuxInputEvent(fd int) (linuxInputEvent, error) {
	var event linuxInputEvent
	size := int(unsafe.Sizeof(event))
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&event)), size)
	n, err := unix.Read(fd, buf)
	if err != nil {
		return linuxInputEvent{}, err
	}
	if n == 0 {
		return linuxInputEvent{}, io.EOF
	}
	if n != size {
		return linuxInputEvent{}, io.ErrUnexpectedEOF
	}
	return event, nil
}

func linuxEvdevEvent(device *linuxEvdevDevice, ev linuxInputEvent, opts ListenOptions, stats *listenerStats, out chan<- InputEvent) {
	switch ev.Type {
	case linuxEvSyn:
		if ev.Code == linuxSynReport {
			linuxEvdevFlushRel(device, ev.Time, opts, stats, out)
		}
	case linuxEvRel:
		if opts.Mask&ListenMouse == 0 {
			return
		}
		switch ev.Code {
		case linuxRelX:
			device.relX += ev.Value
		case linuxRelY:
			device.relY += ev.Value
		case linuxRelWheel:
			device.wheel += ev.Value
		case linuxRelHWheel:
			device.hwheel += ev.Value
		}
	case linuxEvKey:
		state := Down
		if ev.Value == 0 {
			state = Up
		}
		if button, ok := linuxMouseButtonFromCode(ev.Code); ok {
			if opts.Mask&ListenMouse == 0 {
				return
			}
			linuxEvdevEmit(opts, stats, out, InputEvent{
				Kind:   InputEventMouseButton,
				Time:   linuxEventTime(ev.Time),
				Raw:    true,
				Device: uintptr(device.fd),
				Mouse: MouseInputEvent{
					Button: button,
					State:  state,
				},
			})
			return
		}
		key, ok := linuxKeyFromCode(ev.Code)
		if !ok || opts.Mask&ListenKeyboard == 0 {
			return
		}
		linuxEvdevEmit(opts, stats, out, InputEvent{
			Kind:   InputEventKey,
			Time:   linuxEventTime(ev.Time),
			Raw:    true,
			Device: uintptr(device.fd),
			Keyboard: KeyboardInputEvent{
				Key:      key,
				ScanCode: ev.Code,
				State:    state,
			},
		})
	}
}

func linuxEvdevFlushRel(device *linuxEvdevDevice, eventTime unix.Timeval, opts ListenOptions, stats *listenerStats, out chan<- InputEvent) {
	if opts.Mask&ListenMouse == 0 {
		device.relX = 0
		device.relY = 0
		device.wheel = 0
		device.hwheel = 0
		return
	}
	base := InputEvent{
		Time:   linuxEventTime(eventTime),
		Raw:    true,
		Device: uintptr(device.fd),
	}
	if device.relX != 0 || device.relY != 0 {
		event := base
		event.Kind = InputEventMouseMove
		event.Mouse.Move = Rel(int(device.relX), int(device.relY))
		linuxEvdevEmit(opts, stats, out, event)
	}
	if device.wheel != 0 {
		event := base
		event.Kind = InputEventMouseWheel
		event.Mouse.Delta = int(device.wheel) * WheelDelta
		linuxEvdevEmit(opts, stats, out, event)
	}
	if device.hwheel != 0 {
		event := base
		event.Kind = InputEventMouseHWheel
		event.Mouse.Delta = int(device.hwheel) * WheelDelta
		linuxEvdevEmit(opts, stats, out, event)
	}
	device.relX = 0
	device.relY = 0
	device.wheel = 0
	device.hwheel = 0
}

func linuxEvdevEmit(opts ListenOptions, stats *listenerStats, out chan<- InputEvent, event InputEvent) {
	if !prepareInputEvent(&event, opts) {
		return
	}
	select {
	case out <- event:
		stats.delivered.Add(1)
	default:
		stats.dropped.Add(1)
	}
}

func linuxEventTime(tv unix.Timeval) time.Time {
	if tv.Sec == 0 && tv.Usec == 0 {
		return time.Now()
	}
	return time.Unix(int64(tv.Sec), int64(tv.Usec)*1000)
}

func linuxMouseButtonFromCode(code uint16) (MouseButton, bool) {
	switch code {
	case linuxBtnLeft:
		return ButtonLeft, true
	case linuxBtnRight:
		return ButtonRight, true
	case linuxBtnMiddle:
		return ButtonMiddle, true
	case linuxBtnSide:
		return ButtonX1, true
	case linuxBtnExtra:
		return ButtonX2, true
	default:
		return ButtonLeft, false
	}
}

func linuxKeyFromCode(code uint16) (Key, bool) {
	key, ok := linuxKeysByCode[code]
	return key, ok
}

var linuxKeysByCode = map[uint16]Key{
	1: KeyEscape,
	2: Key1, 3: Key2, 4: Key3, 5: Key4, 6: Key5, 7: Key6, 8: Key7, 9: Key8, 10: Key9, 11: Key0,
	12: KeyMinus, 13: KeyEquals, 14: KeyBackspace, 15: KeyTab,
	16: KeyQ, 17: KeyW, 18: KeyE, 19: KeyR, 20: KeyT, 21: KeyY, 22: KeyU, 23: KeyI, 24: KeyO, 25: KeyP,
	26: KeyLeftSquareBracket, 27: KeyRightSquareBracket, 28: KeyEnter,
	29: KeyLeftControl,
	30: KeyA, 31: KeyS, 32: KeyD, 33: KeyF, 34: KeyG, 35: KeyH, 36: KeyJ, 37: KeyK, 38: KeyL,
	39: KeySemicolon, 40: KeySingleQuote, 41: KeyBackQuote, 42: KeyLeftShift,
	43: KeyBackslash, 44: KeyZ, 45: KeyX, 46: KeyC, 47: KeyV, 48: KeyB, 49: KeyN, 50: KeyM,
	51: KeyComma, 52: KeyDot, 53: KeySlash, 54: KeyRightShift,
	55: KeyMultiply, 56: KeyLeftAlt, 57: KeySpace, 58: KeyCapsLock,
	59: KeyF1, 60: KeyF2, 61: KeyF3, 62: KeyF4, 63: KeyF5, 64: KeyF6, 65: KeyF7, 66: KeyF8, 67: KeyF9, 68: KeyF10,
	69: KeyNumLock, 70: KeyScrollLock,
	71: KeyNumpad7, 72: KeyNumpad8, 73: KeyNumpad9, 74: KeySubtract,
	75: KeyNumpad4, 76: KeyNumpad5, 77: KeyNumpad6, 78: KeyAdd,
	79: KeyNumpad1, 80: KeyNumpad2, 81: KeyNumpad3, 82: KeyNumpad0, 83: KeyDecimal,
	87: KeyF11, 88: KeyF12, 97: KeyRightControl, 98: KeyDivide,
	100: KeyRightAlt, 102: KeyHome, 103: KeyUp, 104: KeyPageUp, 105: KeyLeft, 106: KeyRight,
	107: KeyEnd, 108: KeyDown, 109: KeyPageDown, 110: KeyInsert, 111: KeyDelete,
	119: KeyPause, 125: KeyLeftWindows, 126: KeyRightWindows, 127: KeyApps,
}

func bitSet(bits []byte, code uint16) bool {
	index := int(code) / 8
	if index >= len(bits) {
		return false
	}
	return bits[index]&(1<<(code%8)) != 0
}

func evIOCGName(size uintptr) uintptr {
	return linuxIOCR(evIOCGNameNR, size)
}

func evIOCGKey(size uintptr) uintptr {
	return linuxIOCR(evIOCGKeyNR, size)
}

func evIOCGBit(eventType uintptr, size uintptr) uintptr {
	return linuxIOCR(evIOCGBitNR+eventType, size)
}

func linuxIOCR(nr uintptr, size uintptr) uintptr {
	return (linuxIOCRead << linuxIOCDirShift) |
		(size << linuxIOCSizeShift) |
		('E' << linuxIOCTypeShift) |
		(nr << linuxIOCNRShift)
}
