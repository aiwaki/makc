# makc

<p align="center">
  <strong>Pronounced <code>mak-see</code></strong><br>
  <sub>Like <code>Maksim</code> without the final <code>m</code>.</sub><br>
  <sub><strong>M</strong>ouse <strong>A</strong>nd <strong>K</strong>eyboard <strong>C</strong>ontrol for Go.</sub>
</p>

<p align="center">
  <a href="https://github.com/aiwaki/makc/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/aiwaki/makc/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/aiwaki/makc"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/aiwaki/makc.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/aiwaki/makc"></a>
</p>

`makc` is a no-cgo mouse and keyboard control package for Windows, macOS, and
Linux.

The current rewrite replaces the old C header, embedded DLL, and `skip_hook`
submodule with pure Go backends built on:

- [`github.com/ebitengine/purego`](https://github.com/ebitengine/purego)
- [`golang.org/x/sys/windows`](https://pkg.go.dev/golang.org/x/sys/windows)
- [`golang.org/x/sys/unix`](https://pkg.go.dev/golang.org/x/sys/unix)

## Install

```sh
go get github.com/aiwaki/makc
```

The module path is currently `github.com/aiwaki/makc`; it does not use a `/v2`
suffix yet.

## Quick Start

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

	client, err := makc.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	profile := makc.BalancedInputProfile(42)
	sequence := makc.NewInputSequence(
		makc.MoveStep(makc.Rel(10, 0)),
		makc.PauseStep(80*time.Millisecond),
		makc.MoveStep(makc.Rel(-10, 0)),
		makc.TextStep("makc", profile.Typing),
	)

	if err := client.Run(ctx, sequence); err != nil {
		log.Fatal(err)
	}
}
```

For a safer runnable version that does not click or type unless explicitly
asked:

```sh
go run ./examples/sequence
go run ./examples/sequence -text "hello"
go run ./examples/sequence -click
```

## Features

- Mouse state: cursor position, screen size, and button state.
- Mouse injection: relative/absolute movement, buttons, wheel, horizontal
  wheel, batches, drags, deterministic paths, and seeded natural paths.
- Keyboard state: key state, virtual-key events, scan-code events, combos, and
  Unicode text where the backend supports it.
- Timing helpers: click holds, double-click cadence, key holds, fixed/variable
  typing delays, and reusable `Fast`, `Balanced`, and `Careful` input profiles.
- Mixed sequences: build `move -> pause -> click -> type -> hotkey` workflows
  with `InputSequence`, then run them through `Client.Run(ctx, sequence)`.
- Input listeners: Windows low-level hooks, Windows Raw Input, and Linux evdev.
- Runtime diagnostics: display stack, backend dependency probes, Linux portal
  hints, and smoke tooling for local or VM validation.

The legacy `pkg/types`, `pkg/types/buttons`, and `pkg/types/keys` packages are
kept as deprecated compatibility aliases. New code should import the root
`github.com/aiwaki/makc` package directly.

Release notes are tracked in [CHANGELOG.md](CHANGELOG.md).

## Backends

`MouseInjectionAuto` and `KeyboardInjectionAuto` select the preferred backend
for the current platform:

| Platform | Mouse | Keyboard | Notes |
| --- | --- | --- | --- |
| Windows | `InjectMouseInput` when exported, otherwise `SendInput` | `InjectKeyboardInput` when exported, otherwise `SendInput` | `SendInput` supports per-client `dwExtraInfo` tagging. Tested Windows 11 `Inject*Input` builds reject non-zero extra info, so makc sends zero extra info there. |
| macOS | CoreGraphics `CGEvent` | CoreGraphics `CGEvent` | Requires Accessibility permission for event injection. |
| Linux | `/dev/uinput` | `/dev/uinput` | Requires permission to open `/dev/uinput`; listening uses evdev. |

Explicit backend selection:

```go
client, err := makc.Open(
	makc.WithMouseInjection(makc.MouseInjectionInjectMouseInput),
	makc.WithKeyboardInjection(makc.KeyboardInjectionInjectKeyboardInput),
)
```

Linux notes:

- `uinput` supports relative mouse movement, buttons, wheel events, mapped key
  events, and raw Linux key-code scan events.
- Linux input state and listening use `/dev/input/event*` devices.
- Cursor position, screen size, and absolute movement use an optional purego
  X11/Xlib layer when `DISPLAY` is set.
- Wayland absolute cursor control and Unicode text injection still need a
  display-server-specific layer and currently return `ErrUnsupported`.
- GNOME/XDG Desktop Portal diagnostics are available, but the actual libei
  backend is still future work.

## API Map

Common operations:

- `makc.Open`, `Client.Close`, `Client.RuntimeInfo`
- `Client.Mouse.Move`, `MoveTo`, `MoveBy`, `Click`, `DoubleClick`, `Wheel`,
  `HWheel`, `Drag`, `DragBy`, `Inject`
- `Client.Keyboard.Tap`, `TapWithHold`, `Combo`, `TypeText`,
  `TypeTextWithProfile`, `ScanTap`, `Inject`
- `Client.Listen`
- `Client.Run`, `Client.RunSteps`

Profiles and sequences:

- Movement: `InstantMovement`, `LinearMovement`, `EaseInOutMovement`,
  `NaturalMovement`, `NaturalMovementWithJitter`
- Timing: `FixedInterval`, `VariableInterval`, `ClickProfile`,
  `TypingProfile`
- Presets: `InstantInputProfile`, `FastInputProfile`,
  `BalancedInputProfile`, `CarefulInputProfile`
- Sequence steps: `MoveStep`, `ClickStep`, `DoubleClickStep`, `KeyTapStep`,
  `ComboStep`, `TextStep`, `PauseStep`, `MouseStep`, `KeyboardStep`

Parsing helpers for CLIs and config files:

- `ParseKey`
- `ParseMouseButton`

## Examples

```sh
go run ./examples/mouse
go run ./examples/keyboard
go run ./examples/sequence
```

The sequence example only performs a tiny relative move by default. Pass
`-click` or `-text "hello"` when you intentionally want those events.

## Checks

Run the default local checks:

```sh
bash scripts/check.sh
```

Set `MAKC_CHECK_PARALLELS=1` to include a short Windows Parallels smoke pass.

Build smoke binaries manually:

```sh
GOOS=windows GOARCH=arm64 go build -o dist/makc-smoke-windows-arm64.exe ./cmd/makc-smoke
GOOS=darwin GOARCH=arm64 go build -o dist/makc-smoke-darwin-arm64 ./cmd/makc-smoke
GOOS=linux GOARCH=arm64 go build -o dist/makc-smoke-linux-arm64 ./cmd/makc-smoke
```

The smoke command opens the selected backend and reads current state where the
backend supports it. Useful flags:

```sh
./dist/makc-smoke -runtime-info
./dist/makc-smoke -capabilities
./dist/makc-smoke -inject -dx 1 -dy 1
./dist/makc-smoke -tap shift -tap-hold 30ms
./dist/makc-smoke -type "hello" -type-profile variable -type-min-delay 40ms -type-max-delay 120ms
./dist/makc-smoke -click -click-count 2 -click-hold 30ms -click-interval 120ms
```

## Parallels Smoke

For Windows 11 on Parallels Desktop:

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

The script uses `prlctl exec --current-user` so Windows APIs run in the
interactive user session. If Parallels reports a successful resume but the VM
immediately returns to `paused`, disable the VM idle pause option:

```sh
prlctl set "Windows 11" --pause-idle off
```

`MAKC_PARALLELS_TIMEOUT` controls the `prlctl exec` watchdog in seconds. Set it
to `0` to disable the watchdog while debugging Parallels itself.

## Linux Diagnostics

On a Linux desktop or VM with Go installed:

```sh
bash scripts/linux-smoke.sh
```

The helper loads `uinput` when possible, builds `makc-smoke`, and sends a tiny
relative mouse move plus a `Shift` tap through the Linux `uinput` backend by
default. Pass regular `makc-smoke` flags after the script name to customize the
run.

On macOS with Parallels Desktop and a Linux VM with Parallels Tools installed:

```sh
bash scripts/parallels-linux-smoke.sh
```

Inside a Linux guest, discover the active GUI/session-bus environment:

```sh
bash scripts/linux-session-env.sh
bash scripts/linux-session-env.sh --exec ./dist/makc-smoke-linux -runtime-info
```

Read-only XDG Desktop Portal RemoteDesktop diagnostics:

```sh
bash scripts/linux-session-env.sh --exec bash scripts/linux-portal-info.sh
bash scripts/linux-session-env.sh --exec bash scripts/linux-gnome-remote-desktop-info.sh
```

Stateful portal handshake diagnostic:

```sh
GOOS=linux GOARCH=arm64 go build -o dist/makc-portal-handshake-linux-arm64 ./cmd/makc-portal-handshake
bash scripts/linux-session-env.sh --exec ./dist/makc-portal-handshake-linux-arm64
bash scripts/linux-session-env.sh --exec ./dist/makc-portal-handshake-linux-arm64 -select-devices -start -timeout 300s
```

`cmd/makc-portal-handshake` default mode creates and closes a transient portal
session. It does not call `Start`, request permissions, or inject input unless
you pass the corresponding flags. `-start` may show a desktop permission
prompt.

To allow non-root Linux uinput injection, run this inside the Linux guest and
then log out and back in:

```sh
sudo bash scripts/linux-uinput-permissions.sh "$USER"
```
