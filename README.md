# makc

`makc` is a no-cgo mouse and keyboard control package for Windows, macOS, and
Linux.
It is pronounced `mak-see`, like `Maksim` without the final `m`. The name is
also a compact acronym for **Mouse And Keyboard Control**.

The current v2 work-in-progress replaces the old C header and embedded DLL
with a pure Go backend built on:

- [`github.com/ebitengine/purego`](https://github.com/ebitengine/purego)
- [`golang.org/x/sys/windows`](https://pkg.go.dev/golang.org/x/sys/windows)
- [`golang.org/x/sys/unix`](https://pkg.go.dev/golang.org/x/sys/unix)

Windows uses Win32 `SendInput` or `Inject*Input` backends. macOS uses
CoreGraphics `CGEvent` through ApplicationServices and requires Accessibility
permission for event injection. Linux uses `/dev/uinput` for virtual-device
injection and requires permission to open the uinput device.

## Example

```go
package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/aiwaki/makc"
)

func main() {
	ctx := context.Background()

	client, err := makc.Open(makc.WithMouseInjection(makc.MouseInjectionAuto))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	log.Printf("mouse injection backend: %s", client.Mouse.InjectionBackend())
	log.Printf("keyboard injection backend: %s", client.Keyboard.InjectionBackend())

	if err := client.Mouse.Move(ctx, makc.Rel(10, 10)); err != nil {
		log.Fatal(err)
	}
	if err := client.Mouse.Click(ctx, makc.ButtonLeft); err != nil {
		log.Fatal(err)
	}

	if err := client.Keyboard.Combo(ctx, makc.KeyControl, makc.KeyA); err != nil {
		log.Fatal(err)
	}
	if err := client.Keyboard.TypeText(ctx, "makc"); err != nil {
		log.Fatal(err)
	}

	listener, err := client.Listen(ctx, makc.ListenOptions{Mask: makc.ListenAll})
	if err == nil {
		defer listener.Close()
	} else if !errors.Is(err, makc.ErrUnsupported) {
		log.Fatal(err)
	}

	profile := makc.EaseInOutMovement(12, 180*time.Millisecond)
	if err := client.Mouse.DragBy(ctx, makc.ButtonLeft, 80, 40, profile); err != nil {
		log.Fatal(err)
	}
}
```

## Backends

`MouseInjectionAuto` prefers `user32!InjectMouseInput` on Windows when Windows
exports the symbol and falls back to `SendInput` otherwise. On macOS it selects
the CoreGraphics `CGEvent` backend. On Linux it selects the kernel `uinput`
backend. `KeyboardInjectionAuto` follows the same platform split. You can
explicitly request backends:

```go
client, err := makc.Open(
	makc.WithMouseInjection(makc.MouseInjectionInjectMouseInput),
	makc.WithKeyboardInjection(makc.KeyboardInjectionInjectKeyboardInput),
)
```

On macOS:

```go
client, err := makc.Open(
	makc.WithMouseInjection(makc.MouseInjectionCGEvent),
	makc.WithKeyboardInjection(makc.KeyboardInjectionCGEvent),
)
```

On Linux:

```go
client, err := makc.Open(
	makc.WithMouseInjection(makc.MouseInjectionUInput),
	makc.WithKeyboardInjection(makc.KeyboardInjectionUInput),
)
```

The Linux `uinput` backend is injection-focused: it supports relative mouse
movement, mouse buttons, wheel events, mapped key events, and raw Linux key-code
scan events. Linux input state and listening use evdev `/dev/input/event*`
devices. Cursor position, screen size, and absolute movement are available
through an optional purego X11/Xlib layer when `DISPLAY` is set. Wayland
absolute cursor control and Unicode text injection still need a display-server
specific layer and currently return `ErrUnsupported`.

The old cgo header, embedded DLL, and `skip_hook` submodule have been removed
from the active codebase.

## API Surface

- Mouse state: `Position`, `ScreenSize`, `State`, `Down`.
- Mouse injection: `Move`, `MoveTo`, `MoveBy`, `Press`, `Release`, `Click`,
  `Wheel`, `HWheel`, and `Inject` batches.
- Deterministic and seeded mouse paths: `InstantMovement`, `LinearMovement`,
  `EaseInOutMovement`, `NaturalMovement`, `NaturalMovementWithJitter`,
  `MoveToProfile`, `Drag`, `DragFrom`, and `DragBy`.
- Keyboard state: `State`, `Down`.
- Keyboard injection: `Press`, `Release`, `Tap`, `Combo`, `TypeText`,
  `ScanPress`, `ScanRelease`, `ScanTap`, and `Inject` batches.
- Input listener: `Client.Listen` with mouse/keyboard masks, low-level hook,
  Raw Input, or Linux evdev backends, and optional injected-event reporting on
  Windows.
- Runtime diagnostics: `InspectRuntime` and `Client.RuntimeInfo` expose the
  visible display stack plus Linux `/dev/uinput`, evdev, X11, Wayland, libei,
  liboeffis, and portal hints.
- Own-event tagging: `SendInput` events get a per-client `dwExtraInfo` tag by
  default; `InputEvent.Own` and `InputEvent.ExtraInfo` expose that tag inside
  `makc`. `InjectMouseInput` and `InjectKeyboardInput` are sent with zero
  extra info on tested Windows 11 builds because non-zero extra info is
  rejected by those APIs. macOS `CGEvent` and Linux `uinput` injection do not
  currently expose backend tagging.
- Raw Input listening is opt-in because Windows keeps one raw-input
  registration per device class per process. Raw events include
  `InputEvent.Raw`, `InputEvent.Device`, and relative `MouseInputEvent.Move`
  data when the device reports it.
- Linux evdev listening is raw `/dev/input/event*` input. It emits relative
  mouse moves, buttons, wheels, and mapped key events, but it does not provide
  display-server cursor positions or injected-event markers. Linux cursor
  position, screen size, and absolute movement use X11/Xlib when `DISPLAY` is
  available.
- Name parsing: `ParseKey` and `ParseMouseButton` for CLIs and config files.

The legacy `pkg/types`, `pkg/types/buttons`, and `pkg/types/keys` packages are
kept as deprecated compatibility aliases. New code should import the root
`github.com/aiwaki/makc` package directly.

## Smoke Test

Run the default local checks:

```sh
bash scripts/check.sh
```

Set `MAKC_CHECK_PARALLELS=1` to include a short Parallels smoke pass.

Build a Windows ARM64 smoke binary from macOS:

```sh
GOOS=windows GOARCH=arm64 go build -o dist/makc-smoke-windows-arm64.exe ./cmd/makc-smoke
```

Build a macOS ARM64 smoke binary:

```sh
GOOS=darwin GOARCH=arm64 go build -o dist/makc-smoke-darwin-arm64 ./cmd/makc-smoke
```

Build a Linux ARM64 smoke binary:

```sh
GOOS=linux GOARCH=arm64 go build -o dist/makc-smoke-linux-arm64 ./cmd/makc-smoke
```

On a Linux desktop or VM with Go installed, run the uinput smoke helper:

```sh
bash scripts/linux-smoke.sh
```

The helper loads `uinput` when possible, builds `makc-smoke`, and sends a tiny
relative mouse move plus a `Shift` tap through the Linux `uinput` backend by
default. Pass regular `makc-smoke` flags after the script name to customize the
run.

On macOS with Parallels Desktop and a Linux VM with Parallels Tools installed,
run the host-side Fedora smoke helper:

```sh
bash scripts/parallels-linux-smoke.sh
```

The helper builds the Linux smoke binary on the Mac, copies it through the
Parallels shared Home folder using a temporary staging directory without spaces,
and runs the uinput plus evdev listener smoke suite in the guest via
`prlctl exec`. Set `MAKC_PARALLELS_LINUX_VM` when the VM is not named
`Fedora Linux (1)`.

To allow non-root Linux uinput injection, run this inside the Linux guest and
then log out and back in:

```sh
sudo bash scripts/linux-uinput-permissions.sh "$USER"
```

By default the smoke command opens the backend and reads current state where the
backend supports it. Add `-runtime-info` to print display and Linux dependency
diagnostics without opening an input backend. Add `-inject` to perform a tiny
relative mouse move, and `-click` to also click the left mouse button. Add
`-capabilities` to print backend probes for relative movement, absolute
movement, and listener startup without visible clicks or text input.

For Parallels Desktop on Apple Silicon:

```sh
bash scripts/parallels-smoke.sh
bash scripts/parallels-smoke.sh -backend injectmouseinput -inject -dx 1 -dy 1
bash scripts/parallels-smoke.sh -backend injectmouseinput -drag -dx 80 -dy 40
bash scripts/parallels-smoke.sh -backend injectmouseinput -drag -profile natural -seed 42 -dx 80 -dy 40
bash scripts/parallels-smoke.sh -keyboard-backend injectkeyboardinput -tap shift -scan 0x2A
bash scripts/parallels-smoke.sh -listen -include-injected -listen-count 3
bash scripts/parallels-smoke.sh -listen -listen-backend rawinput -listen-count 1
bash scripts/parallels-smoke.sh -backend sendinput -keyboard-backend sendinput -input-tag 0x1234 -listen -normalize-own-injected -listen-count 3
```

The Raw Input smoke command validates that the backend registers and starts; it
prints raw events if you move the mouse or press a key inside the VM during the
short listen window.

The script uses `prlctl exec --current-user` so Windows APIs run in the
interactive user session. If Parallels reports a successful resume but the VM
immediately returns to `paused`, disable the VM's idle pause option:

```sh
prlctl set "Windows 11" --pause-idle off
```

`MAKC_PARALLELS_TIMEOUT` controls the `prlctl exec` watchdog in seconds. Set it
to `0` to disable the watchdog while debugging Parallels itself.
