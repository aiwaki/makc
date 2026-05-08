//go:build !windows && !darwin && !linux

package makc

func fillRuntimeInfo(info *RuntimeInfo) {
	info.Display = DisplayInfo{Server: DisplayServerUnknown}
}
