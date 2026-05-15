//go:build linux

package makc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

// XDG desktop portal RemoteDesktop bindings. Documented at
// https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.RemoteDesktop.html
//
// Lifecycle:
//   1. CreateSession returns a Request handle; we wait on its Response signal
//      to learn the session_handle path.
//   2. SelectDevices declares which input device classes we'll touch
//      (keyboard | pointer mask).
//   3. Start displays a permission dialog (the only blocking-on-user step)
//      and resolves once the user approves.
//   4. NotifyPointer* / NotifyKeyboard* deliver events through the portal,
//      which routes them via libei into the compositor's input pipeline.
//
// Compared to libei: the portal path is pure D-Bus, no native bindings, and
// works on every modern compositor that ships xdg-desktop-portal with the
// RemoteDesktop interface enabled (GNOME ≥ 41, KDE Plasma ≥ 5.27, wlroots
// ≥ 0.16 with portal-wlr).

const (
	portalDestination     = "org.freedesktop.portal.Desktop"
	portalObjectPath      = "/org/freedesktop/portal/desktop"
	portalRemoteDesktop   = "org.freedesktop.portal.RemoteDesktop"
	portalRequestIface    = "org.freedesktop.portal.Request"
	portalSessionIface    = "org.freedesktop.portal.Session"
	portalRequestPathPrefix = "/org/freedesktop/portal/desktop/request"

	portalDeviceKeyboard uint32 = 1
	portalDevicePointer  uint32 = 2

	// Pointer button codes carried in NotifyPointerButton match Linux
	// evdev BTN_* constants. Keep these in sync with linuxBtn* in
	// backend_linux.go but defined separately so the portal file is
	// self-contained.
	portalBtnLeft   uint32 = 0x110
	portalBtnRight  uint32 = 0x111
	portalBtnMiddle uint32 = 0x112
	portalBtnSide   uint32 = 0x113
	portalBtnExtra  uint32 = 0x114

	portalKeyStateUp   uint32 = 0
	portalKeyStateDown uint32 = 1

	// Default request timeout for D-Bus calls. Start may legitimately
	// take longer (waiting on user approval); it is given its own
	// context derived from the caller.
	portalCallTimeout = 5 * time.Second
)

// portalInjector is the long-lived holder for an authenticated
// RemoteDesktop session. Created on linuxBackend init when the user opts
// into the portal backend. Subsequent injection calls reuse the session.
type portalInjector struct {
	mu          sync.Mutex
	conn        *dbus.Conn
	signals     chan *dbus.Signal
	sessionPath dbus.ObjectPath
	hasKeyboard bool
	hasPointer  bool
	closed      bool
}

func openPortalInjector(ctx context.Context, mask uint32) (*portalInjector, error) {
	if mask == 0 {
		return nil, errors.New("makc: portal session needs at least one device class")
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("makc: connect session bus for portal: %w", err)
	}

	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(portalRequestIface),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchPathNamespace(dbus.ObjectPath(portalRequestPathPrefix)),
	); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("makc: subscribe portal Response signal: %w", err)
	}

	pi := &portalInjector{
		conn:    conn,
		signals: signals,
	}

	if err := pi.bringUpSession(ctx, mask); err != nil {
		pi.Close()
		return nil, err
	}
	return pi, nil
}

func (p *portalInjector) bringUpSession(ctx context.Context, mask uint32) error {
	portal := p.conn.Object(portalDestination, dbus.ObjectPath(portalObjectPath))

	// CreateSession.
	createReq, err := p.callCreateSession(ctx, portal)
	if err != nil {
		return err
	}
	createResp, err := p.waitResponse(ctx, createReq)
	if err != nil {
		return fmt.Errorf("makc: portal CreateSession: %w", err)
	}
	if createResp.code != 0 {
		return fmt.Errorf("makc: portal CreateSession returned code %d", createResp.code)
	}
	sessionPath, err := createResp.objectPath("session_handle")
	if err != nil {
		return fmt.Errorf("makc: portal CreateSession: %w", err)
	}
	p.sessionPath = sessionPath

	// SelectDevices.
	selReq, err := p.callSelectDevices(ctx, portal, mask)
	if err != nil {
		return err
	}
	selResp, err := p.waitResponse(ctx, selReq)
	if err != nil {
		return fmt.Errorf("makc: portal SelectDevices: %w", err)
	}
	if selResp.code != 0 {
		return fmt.Errorf("makc: portal SelectDevices returned code %d", selResp.code)
	}

	// Start. The user-facing approval dialog blocks here; respect
	// caller context for cancellation.
	startReq, err := p.callStart(ctx, portal)
	if err != nil {
		return err
	}
	startResp, err := p.waitResponse(ctx, startReq)
	if err != nil {
		return fmt.Errorf("makc: portal Start: %w", err)
	}
	if startResp.code != 0 {
		return fmt.Errorf("makc: portal Start returned code %d (likely user cancelled)", startResp.code)
	}

	// Inspect the device mask the portal granted; the user may have
	// approved a subset of what we requested.
	if granted, ok := startResp.results["devices"]; ok {
		if value, ok := granted.Value().(uint32); ok {
			p.hasKeyboard = value&portalDeviceKeyboard != 0
			p.hasPointer = value&portalDevicePointer != 0
		}
	} else {
		// Older portals don't return the mask; assume what we asked.
		p.hasKeyboard = mask&portalDeviceKeyboard != 0
		p.hasPointer = mask&portalDevicePointer != 0
	}
	return nil
}

func (p *portalInjector) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	sessionPath := p.sessionPath
	conn := p.conn
	p.sessionPath = ""
	p.mu.Unlock()

	if conn != nil && sessionPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = conn.Object(portalDestination, sessionPath).CallWithContext(ctx, portalSessionIface+".Close", 0).Store()
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
	return nil
}

// pointerMotion / pointerMotionAbsolute / pointerButton / pointerAxis /
// keyboardKeycode are thin wrappers over the corresponding portal D-Bus
// methods. Each grabs p.mu so concurrent callers don't interleave on a
// single session — the portal is happy with parallel calls but our
// signal demuxer assumes one outstanding request at a time.

func (p *portalInjector) pointerMotion(ctx context.Context, dx, dy float64) error {
	if !p.hasPointer {
		return errors.New("makc: portal session has no pointer device")
	}
	return p.notify(ctx, "NotifyPointerMotion", emptyOptions(), dx, dy)
}

func (p *portalInjector) pointerMotionAbsolute(ctx context.Context, x, y float64) error {
	if !p.hasPointer {
		return errors.New("makc: portal session has no pointer device")
	}
	// stream argument selects which logical screen; 0 is the first.
	return p.notify(ctx, "NotifyPointerMotionAbsolute", emptyOptions(), uint32(0), x, y)
}

func (p *portalInjector) pointerButton(ctx context.Context, button MouseButton, state State) error {
	if !p.hasPointer {
		return errors.New("makc: portal session has no pointer device")
	}
	code, err := portalMouseButton(button)
	if err != nil {
		return err
	}
	st := portalKeyStateUp
	if state == Down {
		st = portalKeyStateDown
	}
	return p.notify(ctx, "NotifyPointerButton", emptyOptions(), int32(code), st)
}

func (p *portalInjector) pointerAxis(ctx context.Context, dx, dy float64) error {
	if !p.hasPointer {
		return errors.New("makc: portal session has no pointer device")
	}
	return p.notify(ctx, "NotifyPointerAxis", emptyOptions(), dx, dy)
}

func (p *portalInjector) keyboardKeycode(ctx context.Context, keycode int32, state State) error {
	if !p.hasKeyboard {
		return errors.New("makc: portal session has no keyboard device")
	}
	st := portalKeyStateUp
	if state == Down {
		st = portalKeyStateDown
	}
	return p.notify(ctx, "NotifyKeyboardKeycode", emptyOptions(), keycode, st)
}

func (p *portalInjector) notify(ctx context.Context, method string, args ...any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("makc: portal session closed")
	}

	callCtx, cancel := contextWithFallback(ctx, portalCallTimeout)
	defer cancel()

	prepended := make([]any, 0, len(args)+1)
	prepended = append(prepended, p.sessionPath)
	prepended = append(prepended, args...)

	obj := p.conn.Object(portalDestination, dbus.ObjectPath(portalObjectPath))
	if err := obj.CallWithContext(callCtx, portalRemoteDesktop+"."+method, 0, prepended...).Store(); err != nil {
		return fmt.Errorf("makc: portal %s: %w", method, err)
	}
	return nil
}

func contextWithFallback(parent context.Context, fallback time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), fallback)
	}
	if _, ok := parent.Deadline(); ok {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, fallback)
}

func emptyOptions() map[string]dbus.Variant {
	return map[string]dbus.Variant{}
}

func portalMouseButton(button MouseButton) (uint32, error) {
	switch button {
	case ButtonLeft:
		return portalBtnLeft, nil
	case ButtonRight:
		return portalBtnRight, nil
	case ButtonMiddle:
		return portalBtnMiddle, nil
	case ButtonX1:
		return portalBtnSide, nil
	case ButtonX2:
		return portalBtnExtra, nil
	default:
		return 0, fmt.Errorf("makc: unknown mouse button %d", button)
	}
}

// Portal session-bring-up plumbing.

type portalResponse struct {
	code    uint32
	results map[string]dbus.Variant
}

func (r portalResponse) objectPath(name string) (dbus.ObjectPath, error) {
	value, ok := r.results[name]
	if !ok {
		return "", fmt.Errorf("missing %q in portal response", name)
	}
	if path, ok := value.Value().(dbus.ObjectPath); ok {
		return path, nil
	}
	if raw, ok := value.Value().(string); ok {
		return dbus.ObjectPath(raw), nil
	}
	return "", fmt.Errorf("unexpected type %T for %q", value.Value(), name)
}

func (p *portalInjector) callCreateSession(ctx context.Context, portal dbus.BusObject) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(makePortalToken("makccreate")),
		"session_handle_token": dbus.MakeVariant(makePortalToken("makcsession")),
	}
	var request dbus.ObjectPath
	if err := portal.CallWithContext(ctx, portalRemoteDesktop+".CreateSession", 0, options).Store(&request); err != nil {
		return "", fmt.Errorf("makc: portal CreateSession: %w", err)
	}
	return request, nil
}

func (p *portalInjector) callSelectDevices(ctx context.Context, portal dbus.BusObject, mask uint32) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(makePortalToken("makcselect")),
		"types":        dbus.MakeVariant(mask),
	}
	var request dbus.ObjectPath
	if err := portal.CallWithContext(ctx, portalRemoteDesktop+".SelectDevices", 0, p.sessionPath, options).Store(&request); err != nil {
		return "", fmt.Errorf("makc: portal SelectDevices: %w", err)
	}
	return request, nil
}

func (p *portalInjector) callStart(ctx context.Context, portal dbus.BusObject) (dbus.ObjectPath, error) {
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(makePortalToken("makcstart")),
	}
	var request dbus.ObjectPath
	if err := portal.CallWithContext(ctx, portalRemoteDesktop+".Start", 0, p.sessionPath, "", options).Store(&request); err != nil {
		return "", fmt.Errorf("makc: portal Start: %w", err)
	}
	return request, nil
}

func (p *portalInjector) waitResponse(ctx context.Context, request dbus.ObjectPath) (portalResponse, error) {
	for {
		select {
		case <-ctx.Done():
			return portalResponse{}, ctx.Err()
		case sig := <-p.signals:
			if sig == nil || sig.Path != request || sig.Name != portalRequestIface+".Response" {
				continue
			}
			if len(sig.Body) != 2 {
				return portalResponse{}, fmt.Errorf("portal response body has %d fields", len(sig.Body))
			}
			code, ok := sig.Body[0].(uint32)
			if !ok {
				return portalResponse{}, fmt.Errorf("portal response code has type %T", sig.Body[0])
			}
			results, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				return portalResponse{}, fmt.Errorf("portal response results has type %T", sig.Body[1])
			}
			return portalResponse{code: code, results: results}, nil
		}
	}
}

func makePortalToken(prefix string) string {
	return prefix + strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(os.Getpid())
}
