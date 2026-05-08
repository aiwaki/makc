//go:build darwin

package makc

func fillRuntimeInfo(info *RuntimeInfo) {
	info.Display = DisplayInfo{Server: DisplayServerQuartz}
}
