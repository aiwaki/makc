//go:build linux

package makc

import (
	"context"
	"fmt"

	"github.com/ebitengine/purego"
)

type linuxX11Display struct {
	api     *linuxX11API
	display uintptr
	screen  int32
	root    uintptr
}

func newLinuxX11Display() (*linuxX11Display, error) {
	api, err := newLinuxX11API()
	if err != nil {
		return nil, err
	}
	display := api.xOpenDisplay(nil)
	if display == 0 {
		_ = api.close()
		return nil, unsupported("linux X11 display is not available; set DISPLAY")
	}
	screen := api.xDefaultScreen(display)
	root := api.xRootWindow(display, screen)
	if root == 0 {
		_ = api.xCloseDisplay(display)
		_ = api.close()
		return nil, unsupported("linux X11 root window is not available")
	}
	return &linuxX11Display{
		api:     api,
		display: display,
		screen:  screen,
		root:    root,
	}, nil
}

func (x *linuxX11Display) Close() error {
	if x == nil {
		return nil
	}
	if x.display != 0 {
		x.api.xCloseDisplay(x.display)
		x.display = 0
	}
	return x.api.close()
}

func (x *linuxX11Display) screenSize(ctx context.Context) (Point, error) {
	if err := checkContext(ctx); err != nil {
		return Point{}, err
	}
	return Point{
		X: int(x.api.xDisplayWidth(x.display, x.screen)),
		Y: int(x.api.xDisplayHeight(x.display, x.screen)),
	}, nil
}

func (x *linuxX11Display) cursorPos(ctx context.Context) (Point, error) {
	if err := checkContext(ctx); err != nil {
		return Point{}, err
	}
	var rootReturn uintptr
	var childReturn uintptr
	var rootX int32
	var rootY int32
	var winX int32
	var winY int32
	var mask uint32
	if ok := x.api.xQueryPointer(x.display, x.root, &rootReturn, &childReturn, &rootX, &rootY, &winX, &winY, &mask); ok == 0 {
		return Point{}, fmt.Errorf("makc: XQueryPointer failed")
	}
	return Point{X: int(rootX), Y: int(rootY)}, nil
}

func (x *linuxX11Display) movePointer(ctx context.Context, point Point) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if ok := x.api.xWarpPointer(x.display, 0, x.root, 0, 0, 0, 0, int32(point.X), int32(point.Y)); ok == 0 {
		return fmt.Errorf("makc: XWarpPointer failed")
	}
	x.api.xFlush(x.display)
	return nil
}

func newLinuxX11API() (api *linuxX11API, err error) {
	x11, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, unsupported(fmt.Sprintf("load libX11.so.6: %v", err))
	}

	api = &linuxX11API{handle: x11}
	defer func() {
		if err != nil {
			_ = api.close()
		}
	}()
	if err := registerLinuxX11Proc(x11, &api.xOpenDisplay, "XOpenDisplay"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xDefaultScreen, "XDefaultScreen"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xRootWindow, "XRootWindow"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xDisplayWidth, "XDisplayWidth"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xDisplayHeight, "XDisplayHeight"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xQueryPointer, "XQueryPointer"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xWarpPointer, "XWarpPointer"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xFlush, "XFlush"); err != nil {
		return nil, err
	}
	if err := registerLinuxX11Proc(x11, &api.xCloseDisplay, "XCloseDisplay"); err != nil {
		return nil, err
	}
	return api, nil
}

func (api *linuxX11API) close() error {
	if api == nil || api.handle == 0 {
		return nil
	}
	handle := api.handle
	api.handle = 0
	return purego.Dlclose(handle)
}

func registerLinuxX11Proc(handle uintptr, fptr any, name string) error {
	proc, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("makc: load X11 %s: %w", name, err)
	}
	purego.RegisterFunc(fptr, proc)
	return nil
}

type linuxX11API struct {
	handle         uintptr
	xOpenDisplay   func(*byte) uintptr
	xDefaultScreen func(uintptr) int32
	xRootWindow    func(uintptr, int32) uintptr
	xDisplayWidth  func(uintptr, int32) int32
	xDisplayHeight func(uintptr, int32) int32
	xQueryPointer  func(uintptr, uintptr, *uintptr, *uintptr, *int32, *int32, *int32, *int32, *uint32) int32
	xWarpPointer   func(uintptr, uintptr, uintptr, int32, int32, uint32, uint32, int32, int32) int32
	xFlush         func(uintptr) int32
	xCloseDisplay  func(uintptr) int32
}
