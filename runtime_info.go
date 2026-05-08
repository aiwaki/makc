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
	OS      string
	Arch    string
	Display DisplayInfo
	Linux   LinuxRuntimeInfo
}

// DisplayInfo describes the active display server environment.
type DisplayInfo struct {
	Server         DisplayServer
	SessionType    string
	CurrentDesktop string
	DesktopSession string
	Display        string
	WaylandDisplay string
}

// LinuxRuntimeInfo contains Linux-specific runtime diagnostics.
type LinuxRuntimeInfo struct {
	UInput        RuntimeDeviceInfo
	EvdevDevices  int
	X11           RuntimeDependency
	WaylandClient RuntimeDependency
	LibEI         RuntimeDependency
	LibOeffis     RuntimeDependency
	Portal        RuntimePortalInfo
}

// RuntimeDeviceInfo describes a local device path used by an input backend.
type RuntimeDeviceInfo struct {
	Path     string
	Exists   bool
	Readable bool
	Writable bool
	Error    string
}

// RuntimeDependency describes an optional runtime library or service.
type RuntimeDependency struct {
	Name      string
	Available bool
	Error     string
}

// RuntimePortalInfo describes session-bus signals relevant to desktop portals.
type RuntimePortalInfo struct {
	SessionBusAddress string
	SessionBus        bool
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
