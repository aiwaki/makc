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
	cgHIDEventTap            = 0
	cgHeadInsertEventTap     = 0
	cgEventTapOptionListenOnly = 1
)

// CGEventGetIntegerValueField indices.
const (
	cgKeyboardEventKeycode      = 9
	cgMouseEventButtonNumber    = 3
	cgScrollWheelEventDeltaAxis1 = 11
	cgScrollWheelEventDeltaAxis2 = 12
	cgEventSourceUserData       = 42
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

	// commonModesRef is a CFString built with CFStringCreateWithCString
	// containing "kCFRunLoopCommonModes". CFRunLoop matches modes by
	// string equality (CFEqual), so a self-built CFString with the same
	// content works as an alias for the kCFRunLoopCommonModes global —
	// and avoids the unsafe.Pointer(uintptr) Dlsym dereference that vet
	// flags as a possible misuse.
	commonModesRef uintptr
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

		// Build the CFString for kCFRunLoopCommonModes ourselves rather
		// than dereferencing the global symbol — see commonModesRef
		// docs for rationale.
		modeName := []byte("kCFRunLoopCommonModes\x00")
		la.commonModesRef = la.cfStringCreateWithCString(0, &modeName[0], kCFStringEncodingASCII)
		if la.commonModesRef == 0 {
			b.listenAPIErr = errors.New("makc: CFStringCreateWithCString(kCFRunLoopCommonModes) failed")
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
		return &Listener{
			Events: events,
			done:   done,
			cancel: cancel,
			stats:  stats,
		}, nil
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
		cgHIDEventTap,
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
	api.cfRunLoopAddSource(runLoop, source, api.commonModesRef)
	api.cgEventTapEnable(tapPort, true)

	cleanup := func() {
		api.cgEventTapEnable(tapPort, false)
		api.cfRunLoopRemoveSource(runLoop, source, api.commonModesRef)
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
		result := api.cfRunLoopRunInMode(api.commonModesRef, slice, false)
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
	base := InputEvent{
		Time:      time.Now(),
		ExtraInfo: uintptr(api.cgEventGetIntegerValueField(event, cgEventSourceUserData)),
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
		// Modifier change. Not represented by a single up/down; emit as
		// a key event with state derived from the new flags vs prior
		// would require state tracking. For now skip — listeners that
		// need modifier tracking can read CGEventGetFlags via raw taps.
		return InputEvent{}, false
	}
	return InputEvent{}, false
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
