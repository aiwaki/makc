//go:build windows

package makc

func fillRuntimeInfo(info *RuntimeInfo) {
	info.Display = DisplayInfo{Server: DisplayServerWin32}
}
