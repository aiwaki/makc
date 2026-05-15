//go:build darwin

package makc

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CGEventType values used by the macOS listener. Values match
// <CoreGraphics/CGEventTypes.h>.
const (
	cgEventTapDisabledByTimeout   = 0xFFFFFFFE // -2 cast to uint32
	cgEventTapDisabledByUserInput = 0xFFFFFFFF // -1
	cgEventNull                   = 0
	cgEventKeyDown                = 10
	cgEventKeyUp                  = 11
	cgEventFlagsChanged           = 12
	cgEventScrollWheel            = 22
)

// Tap location, placement, and option constants.
const (
	// Tap location. HID-level taps see real hardware events but not the
	// synthetic events that other userspace processes post via
	// CGEventPost — including our own injection path. Session-level
	// taps see both real input AND synthetics that reach the login
	// session, which is what makc users want when they install a
	// listener and inject in the same process.
	cgHIDEventTap              = 0
	cgSessionEventTap          = 1
	cgHeadInsertEventTap       = 0
	cgEventTapOptionListenOnly = 1
)

// CGEventGetIntegerValueField indices.
const (
	cgKeyboardEventKeycode       = 9
	cgMouseEventButtonNumber     = 3
	cgScrollWheelEventDeltaAxis1 = 11
	cgScrollWheelEventDeltaAxis2 = 12
	cgEventSourceUnixProcessID   = 41
	cgEventSourceUserData        = 42
)

// CGEventFlags bits for modifier keys. Used by the FlagsChanged handler
// to derive up/down state from the post-change flag word.
const (
	cgFlagMaskAlphaShift  = 0x00010000 // Caps Lock
	cgFlagMaskShift       = 0x00020000
	cgFlagMaskControl     = 0x00040000
	cgFlagMaskAlternate   = 0x00080000 // Option
	cgFlagMaskCommand     = 0x00100000
	cgFlagMaskSecondaryFn = 0x00800000
)

// CFRunLoopRunInMode return codes.
const (
	cfRunLoopRunFinished      = 1
	cfRunLoopRunStopped       = 2
	cfRunLoopRunTimedOut      = 3
	cfRunLoopRunHandledSource = 4
)

// listenerAPI bundles the Quartz / CoreFoundation symbols specific to event
// taps. Bound lazily on first ListenInput call to avoid paying the Dlsym
// cost when the backend is only used for injection.
type darwinListenAPI struct {
	cgEventTapCreate              func(uint32, uint32, uint32, uint64, uintptr, unsafe.Pointer) uintptr
	cgEventTapEnable              func(uintptr, bool)
	cgEventTapIsEnabled           func(uintptr) bool
	cgEventGetType                func(uintptr) uint32
	cgEventGetFlags               func(uintptr) uint64
	cgEventGetLocation            func(uintptr) cgPoint
	cgEventGetIntegerValueField   func(uintptr, uint32) int64
	cfStringCreateWithCString     func(uintptr, *byte, uint32) uintptr
	cfMachPortCreateRunLoopSource func(uintptr, uintptr, uintptr) uintptr
	cfRunLoopGetCurrent           func() uintptr
	cfRunLoopAddSource            func(uintptr, uintptr, uintptr)
	cfRunLoopRemoveSource         func(uintptr, uintptr, uintptr)
	cfRunLoopRunInMode            func(uintptr, float64, bool) int32
	cfRunLoopStop                 func(uintptr)

	// commonModesRef is the CFString "kCFRunLoopCommonModes". Used with
	// CFRunLoopAddSource/CFRunLoopRemoveSource: a source registered in
	// the common-modes set is automatically added to every mode that's
	// currently a member of the set (default mode, modal panel mode,
	// event tracking mode, etc).
	//
	// defaultModeRef is the CFString "kCFRunLoopDefaultMode". Used with
	// CFRunLoopRunInMode: that call requires a real mode name, NOT the
	// common-modes marker — passing kCFRunLoopCommonModes triggers the
	// "invalid mode 'kCFRunLoopCommonModes' provided to
	// CFRunLoopRunSpecific" warning at startup. The source is added via
	// commonModesRef so events still flow into the default-mode loop.
	//
	// Both CFStrings are built locally via CFStringCreateWithCString.
	// CFRunLoop matches modes by CFEqual, so a self-built CFString with
	// the documented content acts as an alias for the global — without
	// dereferencing the const symbols (purego/Dlsym on data symbols is
	// fragile and vet-noisy).
	commonModesRef uintptr
	defaultModeRef uintptr
}

// kCFStringEncodingASCII = 0x0600. ASCII is sufficient for the mode name.
const kCFStringEncodingASCII = 0x0600

func (b *darwinBackend) ensureListenAPI() error {
	b.listenAPIOnce.Do(func() {
		appServices, err := purego.Dlopen("/System/Library/Frameworks/ApplicationServices.framework/ApplicationServices", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			b.listenAPIErr = fmt.Errorf("makc: load ApplicationServices.framework: %w", err)
			return
		}
		coreFoundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			b.listenAPIErr = fmt.Errorf("makc: load CoreFoundation.framework: %w", err)
			return
		}

		la := &darwinListenAPI{}
		bindings := []struct {
			handle uintptr
			fptr   any
			name   string
		}{
			{appServices, &la.cgEventTapCreate, "CGEventTapCreate"},
			{appServices, &la.cgEventTapEnable, "CGEventTapEnable"},
			{appServices, &la.cgEventTapIsEnabled, "CGEventTapIsEnabled"},
			{appServices, &la.cgEventGetType, "CGEventGetType"},
			{appServices, &la.cgEventGetFlags, "CGEventGetFlags"},
			{appServices, &la.cgEventGetLocation, "CGEventGetLocation"},
			{appServices, &la.cgEventGetIntegerValueField, "CGEventGetIntegerValueField"},
			{coreFoundation, &la.cfStringCreateWithCString, "CFStringCreateWithCString"},
			{coreFoundation, &la.cfMachPortCreateRunLoopSource, "CFMachPortCreateRunLoopSource"},
			{coreFoundation, &la.cfRunLoopGetCurrent, "CFRunLoopGetCurrent"},
			{coreFoundation, &la.cfRunLoopAddSource, "CFRunLoopAddSource"},
			{coreFoundation, &la.cfRunLoopRemoveSource, "CFRunLoopRemoveSource"},
			{coreFoundation, &la.cfRunLoopRunInMode, "CFRunLoopRunInMode"},
			{coreFoundation, &la.cfRunLoopStop, "CFRunLoopStop"},
		}
		for _, bind := range bindings {
			proc, err := purego.Dlsym(bind.handle, bind.name)
			if err != nil {
				b.listenAPIErr = fmt.Errorf("makc: load %s: %w", bind.name, err)
				return
			}
			purego.RegisterFunc(bind.fptr, proc)
		}

		// Build CFStrings for the run-loop modes we use. See doc on
		// commonModesRef / defaultModeRef for why these are built
		// locally instead of dereferencing the global symbols.
		commonName := []byte("kCFRunLoopCommonModes\x00")
		la.commonModesRef = la.cfStringCreateWithCString(0, &commonName[0], kCFStringEncodingASCII)
		if la.commonModesRef == 0 {
			b.listenAPIErr = errors.New("makc: CFStringCreateWithCString(kCFRunLoopCommonModes) failed")
			return
		}
		defaultName := []byte("kCFRunLoopDefaultMode\x00")
		la.defaultModeRef = la.cfStringCreateWithCString(0, &defaultName[0], kCFStringEncodingASCII)
		if la.defaultModeRef == 0 {
			b.listenAPIErr = errors.New("makc: CFStringCreateWithCString(kCFRunLoopDefaultMode) failed")
			return
		}

		b.listenAPI = la
	})
	return b.listenAPIErr
}

// ListenInput installs a CGEventTap at the HID stream and dispatches events
// onto the listener channel. Requires Input Monitoring permission. Replaces
// the previous "not implemented" stub.
func (b *darwinBackend) ListenInput(ctx context.Context, opts ListenOptions) (*Listener, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := b.ensureListenAPI(); err != nil {
		return nil, err
	}
	switch opts.Backend {
	case ListenBackendAuto, ListenBackendLowLevelHook:
		// Treat hook as the macOS-equivalent name for the event tap.
	case ListenBackendRawInput:
		return nil, unsupported("raw input listening is only available on Windows")
	case ListenBackendEvdev:
		return nil, unsupported("evdev listening is only available on Linux")
	default:
		return nil, unsupported("unknown listen backend")
	}

	if !b.activeListener.CompareAndSwap(false, true) {
		return nil, errors.New("makc: another macOS listener is already active on this client")
	}

	ctx, cancel := context.WithCancel(ctx)
	events := make(chan InputEvent, opts.Buffer)
	done := make(chan error, 1)
	ready := make(chan error, 1)
	stats := newListenerStats()

	go b.runEventTapListener(ctx, opts, stats, events, ready, done)

	select {
	case err := <-ready:
		if err != nil {
			cancel()
			b.activeListener.Store(false)
			return nil, err
		}
		return newListener(events, done, cancel, stats), nil
	case <-ctx.Done():
		cancel()
		b.activeListener.Store(false)
		return nil, ctx.Err()
	}
}

// tapDispatcher is the per-listener routing target captured by the tap
// callback closure. Stored in atomic.Pointer so the callback can detach
// cleanly when the listener stops.
type tapDispatcher struct {
	emit    func(InputEvent)
	tapPort uintptr // for re-enable on tap-disabled events
	api     *darwinListenAPI
}

func (b *darwinBackend) runEventTapListener(ctx context.Context, opts ListenOptions, stats *listenerStats, events chan<- InputEvent, ready chan<- error, done chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(events)
	defer b.activeListener.Store(false)

	api := b.listenAPI
	dispatcher := &tapDispatcher{api: api}
	b.activeTapDispatcher.Store(dispatcher)
	defer b.activeTapDispatcher.Store(nil)

	dispatcher.emit = func(event InputEvent) {
		markOwnInputEvent(&event, b.InputTag())
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

	mask := darwinEventMask(opts.Mask)
	if mask == 0 {
		ready <- unsupported("empty listen mask")
		done <- nil
		return
	}

	b.ensureTapCallback()
	tapPort := api.cgEventTapCreate(
		cgSessionEventTap,
		cgHeadInsertEventTap,
		cgEventTapOptionListenOnly,
		mask,
		b.tapCallback,
		nil,
	)
	if tapPort == 0 {
		ready <- errors.New("makc: CGEventTapCreate failed (Input Monitoring permission required)")
		done <- nil
		return
	}
	dispatcher.tapPort = tapPort

	source := api.cfMachPortCreateRunLoopSource(0, tapPort, 0)
	if source == 0 {
		b.api.cfRelease(tapPort)
		ready <- errors.New("makc: CFMachPortCreateRunLoopSource failed")
		done <- nil
		return
	}

	runLoop := api.cfRunLoopGetCurrent()
	// Attach the source to the concrete default mode rather than to
	// kCFRunLoopCommonModes. CommonModes is a placeholder that adds the
	// source to whatever modes have been *registered* as common — and a
	// freshly-created runloop on a goroutine-pinned OS thread has no
	// modes registered as common yet, so the source ends up attached to
	// nothing and the callback never fires. Default mode is always
	// available on any runloop.
	api.cfRunLoopAddSource(runLoop, source, api.defaultModeRef)
	api.cgEventTapEnable(tapPort, true)
	// CGEventTapCreate succeeds and returns a port even when the process
	// lacks Input Monitoring permission (Big Sur+); the tap is created
	// but events never reach the callback. CGEventTapIsEnabled returns
	// false in that state, giving us the only programmatic signal that
	// the missing permission is the cause of the silent listener.
	if !api.cgEventTapIsEnabled(tapPort) {
		api.cfRunLoopRemoveSource(runLoop, source, api.defaultModeRef)
		b.api.cfRelease(source)
		b.api.cfRelease(tapPort)
		ready <- errors.New("makc: CGEventTap is disabled — grant your binary Input Monitoring permission in System Settings → Privacy & Security → Input Monitoring")
		done <- nil
		return
	}

	cleanup := func() {
		api.cgEventTapEnable(tapPort, false)
		api.cfRunLoopRemoveSource(runLoop, source, api.defaultModeRef)
		b.api.cfRelease(source)
		b.api.cfRelease(tapPort)
	}

	go func() {
		<-ctx.Done()
		api.cfRunLoopStop(runLoop)
	}()

	ready <- nil

	// Run the loop in short slices so context cancellation propagates
	// even if no events arrive — CFRunLoopStop only takes effect at the
	// end of the current cycle, and a tap with no traffic blocks
	// indefinitely otherwise.
	const slice = 0.25 // seconds
	for {
		result := api.cfRunLoopRunInMode(api.defaultModeRef, slice, false)
		if result == cfRunLoopRunStopped || result == cfRunLoopRunFinished {
			break
		}
		if ctx.Err() != nil {
			break
		}
	}

	cleanup()
	done <- nil
}

// ensureTapCallback registers the singleton CGEventTap callback. Same
// motivation as ensureHookCallbacks on Windows: NewCallback slots are
// finite and not reclaimable, so we register one and route through
// activeTapDispatcher.
func (b *darwinBackend) ensureTapCallback() {
	b.tapCallbackOnce.Do(func() {
		b.tapCallback = purego.NewCallback(func(proxy uintptr, eventType uint32, event uintptr, refcon uintptr) uintptr {
			d := b.activeTapDispatcher.Load()
			if d == nil {
				return event
			}
			// Tap re-enable on watchdog timeouts. macOS disables the tap
			// if its callback exceeds the eventSuppressionStateRemoteSyntheticEvents
			// budget; we re-enable to keep listening.
			if eventType == cgEventTapDisabledByTimeout || eventType == cgEventTapDisabledByUserInput {
				d.api.cgEventTapEnable(d.tapPort, true)
				return event
			}
			ev, ok := darwinTapToEvent(d.api, eventType, event)
			if ok {
				d.emit(ev)
			}
			return event
		})
	})
}

// darwinEventMask builds the eventsOfInterest bitmask for CGEventTapCreate
// from the makc ListenMask. Each bit position is a CGEventType value.
func darwinEventMask(mask ListenMask) uint64 {
	var m uint64
	if mask&ListenMouse != 0 {
		for _, t := range []uint32{
			cgEventLeftMouseDown,
			cgEventLeftMouseUp,
			cgEventRightMouseDown,
			cgEventRightMouseUp,
			cgEventMouseMoved,
			cgEventLeftMouseDragged,
			cgEventRightMouseDragged,
			cgEventOtherMouseDown,
			cgEventOtherMouseUp,
			cgEventOtherMouseDragged,
			cgEventScrollWheel,
		} {
			m |= 1 << t
		}
	}
	if mask&ListenKeyboard != 0 {
		m |= 1 << cgEventKeyDown
		m |= 1 << cgEventKeyUp
		m |= 1 << cgEventFlagsChanged
	}
	return m
}

// darwinTapToEvent converts a CGEventRef into makc's InputEvent. Returns
// ok=false for event types not relevant to listeners (tap-disabled events
// are handled in the callback before reaching here).
func darwinTapToEvent(api *darwinListenAPI, eventType uint32, event uintptr) (InputEvent, bool) {
	loc := api.cgEventGetLocation(event)
	// macOS attaches the source process PID to every event. Real HID
	// input from the kernel reports PID 0; anything posted from
	// userspace via CGEventPost/CGEventCreate carries the posting
	// process's PID. Treat non-zero PID as the macOS analogue of
	// LLMHF_INJECTED on Windows.
	base := InputEvent{
		Time:      time.Now(),
		ExtraInfo: uintptr(api.cgEventGetIntegerValueField(event, cgEventSourceUserData)),
		Injected:  api.cgEventGetIntegerValueField(event, cgEventSourceUnixProcessID) != 0,
	}
	base.Mouse.Position = Point{X: int(loc.X), Y: int(loc.Y)}

	switch eventType {
	case cgEventMouseMoved, cgEventLeftMouseDragged, cgEventRightMouseDragged, cgEventOtherMouseDragged:
		base.Kind = InputEventMouseMove
		base.Mouse.Move = Abs(int(loc.X), int(loc.Y))
		return base, true
	case cgEventLeftMouseDown:
		base.Kind = InputEventMouseButton
		base.Mouse.Button = ButtonLeft
		base.Mouse.State = Down
		return base, true
	case cgEventLeftMouseUp:
		base.Kind = InputEventMouseButton
		base.Mouse.Button = ButtonLeft
		base.Mouse.State = Up
		return base, true
	case cgEventRightMouseDown:
		base.Kind = InputEventMouseButton
		base.Mouse.Button = ButtonRight
		base.Mouse.State = Down
		return base, true
	case cgEventRightMouseUp:
		base.Kind = InputEventMouseButton
		base.Mouse.Button = ButtonRight
		base.Mouse.State = Up
		return base, true
	case cgEventOtherMouseDown:
		base.Kind = InputEventMouseButton
		base.Mouse.Button = darwinOtherMouseButton(api.cgEventGetIntegerValueField(event, cgMouseEventButtonNumber))
		base.Mouse.State = Down
		return base, true
	case cgEventOtherMouseUp:
		base.Kind = InputEventMouseButton
		base.Mouse.Button = darwinOtherMouseButton(api.cgEventGetIntegerValueField(event, cgMouseEventButtonNumber))
		base.Mouse.State = Up
		return base, true
	case cgEventScrollWheel:
		base.Kind = InputEventMouseWheel
		base.Mouse.Delta = int(api.cgEventGetIntegerValueField(event, cgScrollWheelEventDeltaAxis1)) * WheelDelta
		return base, true
	case cgEventKeyDown:
		base.Kind = InputEventKey
		base.Keyboard = KeyboardInputEvent{
			ScanCode: uint16(api.cgEventGetIntegerValueField(event, cgKeyboardEventKeycode)),
			Key:      darwinKeyFromScanCode(uint16(api.cgEventGetIntegerValueField(event, cgKeyboardEventKeycode))),
			State:    Down,
		}
		return base, true
	case cgEventKeyUp:
		base.Kind = InputEventKey
		base.Keyboard = KeyboardInputEvent{
			ScanCode: uint16(api.cgEventGetIntegerValueField(event, cgKeyboardEventKeycode)),
			Key:      darwinKeyFromScanCode(uint16(api.cgEventGetIntegerValueField(event, cgKeyboardEventKeycode))),
			State:    Down,
		}
		base.Keyboard.State = Up
		return base, true
	case cgEventFlagsChanged:
		// Modifier press/release. The event carries the post-change
		// flag word and the keycode of the changed key. Derive state
		// from whether the keycode's mask bit is set in the new flags
		// — no separate state tracking required.
		scan := uint16(api.cgEventGetIntegerValueField(event, cgKeyboardEventKeycode))
		mask := darwinModifierMaskForScanCode(scan)
		if mask == 0 {
			return InputEvent{}, false
		}
		state := Up
		if api.cgEventGetFlags(event)&mask != 0 {
			state = Down
		}
		base.Kind = InputEventKey
		base.Keyboard = KeyboardInputEvent{
			ScanCode: scan,
			Key:      darwinKeyFromScanCode(scan),
			State:    state,
		}
		return base, true
	}
	return InputEvent{}, false
}

// darwinModifierMaskForScanCode maps a virtual key code for a modifier key
// to the CGEventFlags bit it sets when held. Returns 0 for non-modifier
// keys, signalling the caller to skip.
func darwinModifierMaskForScanCode(scan uint16) uint64 {
	switch scan {
	case 0x37, 0x36: // Left Cmd, Right Cmd
		return cgFlagMaskCommand
	case 0x38, 0x3C: // Left Shift, Right Shift
		return cgFlagMaskShift
	case 0x3A, 0x3D: // Left Alt/Option, Right Alt/Option
		return cgFlagMaskAlternate
	case 0x3B, 0x3E: // Left Control, Right Control
		return cgFlagMaskControl
	case 0x39: // Caps Lock
		return cgFlagMaskAlphaShift
	case 0x3F: // Fn
		return cgFlagMaskSecondaryFn
	default:
		return 0
	}
}

// darwinOtherMouseButton maps CGMouseEventButtonNumber into makc's
// MouseButton enum for buttons beyond left/right (button 2+).
func darwinOtherMouseButton(n int64) MouseButton {
	switch n {
	case 2:
		return ButtonMiddle
	case 3:
		return ButtonX1
	case 4:
		return ButtonX2
	default:
		return ButtonMiddle
	}
}

// darwinKeyFromScanCode reverses the darwinKeyCodes mapping. Built lazily
// once on first use.
var darwinKeysByScanCode atomic.Pointer[map[uint16]Key]

func darwinKeyFromScanCode(code uint16) Key {
	m := darwinKeysByScanCode.Load()
	if m == nil {
		built := make(map[uint16]Key, len(darwinKeyCodes))
		for k, v := range darwinKeyCodes {
			if _, exists := built[v]; !exists {
				built[v] = k
			}
		}
		darwinKeysByScanCode.Store(&built)
		m = &built
	}
	if key, ok := (*m)[code]; ok {
		return key
	}
	return KeyUnknown
}
