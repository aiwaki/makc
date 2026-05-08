package makc

import (
	"context"
	"runtime"
	"strings"
)

// DisplayServer identifies the desktop display/input stack visible to makc.
type DisplayServer string

const (
	DisplayServerUnknown  DisplayServer = "unknown"
	DisplayServerHeadless DisplayServer = "headless"
	DisplayServerX11      DisplayServer = "x11"
	DisplayServerWayland  DisplayServer = "wayland"
	DisplayServerQuartz   DisplayServer = "quartz"
	DisplayServerWin32    DisplayServer = "win32"
)

// RuntimeInfo describes the input-related runtime environment.
type RuntimeInfo struct {
	// OS is the Go runtime operating system name.
	OS string

	// Arch is the Go runtime architecture name.
	Arch string

	// Display describes the active display/session environment.
	Display DisplayInfo

	// Linux contains Linux-specific diagnostics. It is zero-valued on other
	// platforms.
	Linux LinuxRuntimeInfo
}

// DisplayInfo describes the active display server environment.
type DisplayInfo struct {
	// Server is makc's normalized display server classification.
	Server DisplayServer

	// SessionType is the raw XDG_SESSION_TYPE value when available.
	SessionType string

	// CurrentDesktop is the raw XDG_CURRENT_DESKTOP value when available.
	CurrentDesktop string

	// DesktopSession is the raw DESKTOP_SESSION value when available.
	DesktopSession string

	// Display is the raw DISPLAY value when available.
	Display string

	// WaylandDisplay is the raw WAYLAND_DISPLAY value when available.
	WaylandDisplay string
}

// LinuxRuntimeInfo contains Linux-specific runtime diagnostics.
type LinuxRuntimeInfo struct {
	// UInput describes access to /dev/uinput.
	UInput RuntimeDeviceInfo

	// EvdevDevices is the number of readable /dev/input/event* devices found.
	EvdevDevices int

	// X11 describes the optional Xlib dependency.
	X11 RuntimeDependency

	// WaylandClient describes the optional Wayland client dependency.
	WaylandClient RuntimeDependency

	// LibEI describes the optional libei dependency.
	LibEI RuntimeDependency

	// LibOeffis describes the optional liboeffis dependency.
	LibOeffis RuntimeDependency

	// Portal describes session-bus signals used by desktop portal diagnostics.
	Portal RuntimePortalInfo
}

// RuntimeDeviceInfo describes a local device path used by an input backend.
type RuntimeDeviceInfo struct {
	// Path is the device path that was inspected.
	Path string

	// Exists reports whether the path exists.
	Exists bool

	// Readable reports whether the process can open the path for reading.
	Readable bool

	// Writable reports whether the process can open the path for writing.
	Writable bool

	// Error contains a probe error message, if any.
	Error string
}

// RuntimeDependency describes an optional runtime library or service.
type RuntimeDependency struct {
	// Name is the library or service name that was probed.
	Name string

	// Available reports whether the dependency is available to this process.
	Available bool

	// Error contains a probe error message, if any.
	Error string
}

// RuntimePortalInfo describes session-bus signals relevant to desktop portals.
type RuntimePortalInfo struct {
	// SessionBusAddress is the DBUS_SESSION_BUS_ADDRESS value when available.
	SessionBusAddress string

	// SessionBus reports whether a session bus address is present.
	SessionBus bool

	// RemoteDesktopHint reports whether the environment suggests a remote
	// desktop portal may be available.
	RemoteDesktopHint bool
}

// InspectRuntime returns input-related diagnostics for the current process.
func InspectRuntime() RuntimeInfo {
	info := RuntimeInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}
	fillRuntimeInfo(&info)
	return info
}

// RuntimeInfo returns input-related diagnostics for this client's process.
func (c *Client) RuntimeInfo(ctx context.Context) (RuntimeInfo, error) {
	if err := c.ensureReady(ctx); err != nil {
		return RuntimeInfo{}, err
	}
	return InspectRuntime(), nil
}

func unixDisplayInfo(sessionType, currentDesktop, desktopSession, display, waylandDisplay string) DisplayInfo {
	sessionType = strings.ToLower(strings.TrimSpace(sessionType))
	return DisplayInfo{
		Server:         unixDisplayServer(sessionType, display, waylandDisplay),
		SessionType:    sessionType,
		CurrentDesktop: currentDesktop,
		DesktopSession: desktopSession,
		Display:        display,
		WaylandDisplay: waylandDisplay,
	}
}

func unixDisplayServer(sessionType, display, waylandDisplay string) DisplayServer {
	sessionType = strings.ToLower(strings.TrimSpace(sessionType))
	display = strings.TrimSpace(display)
	waylandDisplay = strings.TrimSpace(waylandDisplay)
	switch sessionType {
	case "wayland":
		return DisplayServerWayland
	case "x11":
		return DisplayServerX11
	}
	if waylandDisplay != "" {
		return DisplayServerWayland
	}
	if display != "" {
		return DisplayServerX11
	}
	switch sessionType {
	case "", "tty", "unspecified":
		return DisplayServerHeadless
	default:
		return DisplayServerUnknown
	}
}
