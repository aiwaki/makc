# makc

`makc` is a no-cgo mouse and keyboard control package for Windows.

The current v2 work-in-progress replaces the old C header and embedded DLL
with a pure Go backend built on:

- [`github.com/ebitengine/purego`](https://github.com/ebitengine/purego)
- [`golang.org/x/sys/windows`](https://pkg.go.dev/golang.org/x/sys/windows)

## Example

```go
package main

import (
	"context"
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
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	profile := makc.EaseInOutMovement(12, 180*time.Millisecond)
	if err := client.Mouse.DragBy(ctx, makc.ButtonLeft, 80, 40, profile); err != nil {
		log.Fatal(err)
	}
}
```

## Backends

`MouseInjectionAuto` prefers `user32!InjectMouseInput` when Windows exports the
symbol and falls back to `SendInput` otherwise. `KeyboardInjectionAuto` does the
same for `user32!InjectKeyboardInput`, falling back to keyboard `SendInput`
when the symbol is absent. You can explicitly request backends:

```go
client, err := makc.Open(
	makc.WithMouseInjection(makc.MouseInjectionInjectMouseInput),
	makc.WithKeyboardInjection(makc.KeyboardInjectionInjectKeyboardInput),
)
```

The old cgo header, embedded DLL, and `skip_hook` submodule have been removed
from the active codebase.

## API Surface

- Mouse state: `Position`, `ScreenSize`, `State`, `Down`.
- Mouse injection: `Move`, `MoveTo`, `MoveBy`, `Press`, `Release`, `Click`,
  `Wheel`, `HWheel`, and `Inject` batches.
- Deterministic mouse paths: `InstantMovement`, `LinearMovement`,
  `EaseInOutMovement`, `MoveToProfile`, `Drag`, `DragFrom`, and `DragBy`.
- Keyboard state: `State`, `Down`.
- Keyboard injection: `Press`, `Release`, `Tap`, `Combo`, `TypeText`,
  `ScanPress`, `ScanRelease`, `ScanTap`, and `Inject` batches.
- Input listener: `Client.Listen` with mouse/keyboard masks, low-level hook or
  Raw Input backends, and optional injected-event reporting.
- Own-event tagging: `SendInput` events get a per-client `dwExtraInfo` tag by
  default; `InputEvent.Own` and `InputEvent.ExtraInfo` expose that tag inside
  `makc`. `InjectMouseInput` and `InjectKeyboardInput` are sent with zero
  extra info on tested Windows 11 builds because non-zero extra info is
  rejected by those APIs.
- Raw Input listening is opt-in because Windows keeps one raw-input
  registration per device class per process. Raw events include
  `InputEvent.Raw`, `InputEvent.Device`, and relative `MouseInputEvent.Move`
  data when the device reports it.
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

By default the smoke command only opens the backend and reads current state. Add
`-inject` to perform a tiny relative mouse move, and `-click` to also click the
left mouse button.

For Parallels Desktop on Apple Silicon:

```sh
bash scripts/parallels-smoke.sh
bash scripts/parallels-smoke.sh -backend injectmouseinput -inject -dx 1 -dy 1
bash scripts/parallels-smoke.sh -backend injectmouseinput -drag -dx 80 -dy 40
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
