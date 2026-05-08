//go:build linux

package makc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

func fillRuntimeInfo(info *RuntimeInfo) {
	info.Display = unixDisplayInfo(
		linuxEnv("XDG_SESSION_TYPE"),
		linuxEnv("XDG_CURRENT_DESKTOP"),
		linuxEnv("DESKTOP_SESSION"),
		linuxEnv("DISPLAY"),
		linuxEnv("WAYLAND_DISPLAY"),
	)
	info.Linux = LinuxRuntimeInfo{
		UInput:        linuxDeviceInfo("/dev/uinput"),
		EvdevDevices:  linuxEvdevDeviceCount(),
		X11:           linuxLibraryInfo("libX11", "libX11.so.6", "libX11.so"),
		WaylandClient: linuxLibraryInfo("libwayland-client", "libwayland-client.so.0", "libwayland-client.so"),
		LibEI:         linuxLibraryInfo("libei", "libei.so.1", "libei.so.0", "libei.so"),
		LibOeffis:     linuxLibraryInfo("liboeffis", "liboeffis.so.1", "liboeffis.so.0", "liboeffis.so"),
		Portal:        linuxPortalInfo(info.Display),
	}
}

func linuxEnv(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func linuxDeviceInfo(path string) RuntimeDeviceInfo {
	info := RuntimeDeviceInfo{Path: path}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return info
		}
		info.Error = err.Error()
		return info
	}
	info.Exists = true
	info.Readable = unix.Access(path, unix.R_OK) == nil
	info.Writable = unix.Access(path, unix.W_OK) == nil
	return info
}

func linuxEvdevDeviceCount() int {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return 0
	}
	return len(matches)
}

func linuxLibraryInfo(label string, names ...string) RuntimeDependency {
	info := RuntimeDependency{Name: label}
	for _, name := range names {
		handle, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err == nil {
			_ = purego.Dlclose(handle)
			info.Name = name
			info.Available = true
			info.Error = ""
			return info
		}
		info.Error = fmt.Sprintf("%s: %v", name, err)
	}
	return info
}

func linuxPortalInfo(display DisplayInfo) RuntimePortalInfo {
	address := linuxEnv("DBUS_SESSION_BUS_ADDRESS")
	portal := RuntimePortalInfo{
		SessionBusAddress: address,
		SessionBus:        address != "",
	}
	portal.RemoteDesktopHint = portal.SessionBus && display.Server == DisplayServerWayland
	return portal
}
