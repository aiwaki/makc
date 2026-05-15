# makc

<p align="center">
  <strong>Pronounced <code>mak-see</code></strong> —
  <strong>M</strong>ouse <strong>A</strong>nd <strong>K</strong>eyboard <strong>C</strong>ontrol for Go.
</p>

<p align="center">
  <a href="https://github.com/aiwaki/makc/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/aiwaki/makc/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/aiwaki/makc"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/aiwaki/makc.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/aiwaki/makc"></a>
</p>

Synthesize mouse and keyboard input on Windows, macOS, and Linux from a single
Go API. No cgo, no embedded DLL — just `purego`, `x/sys/windows`, and
`x/sys/unix`.

Use it for: desktop automation, RPA, accessibility tools, integration tests,
macro keyboards, custom input devices, anything that needs to drive a UI as if
a human were at the keyboard.

---

## Install

```sh
go get github.com/aiwaki/makc
```

Requires Go 1.23+.

---

## Hello, click

```go
package main

import (
	"context"
	"log"

	"github.com/aiwaki/makc"
)

func main() {
	client, err := makc.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Mouse.Click(ctx, makc.ButtonLeft); err != nil {
		log.Fatal(err)
	}
}
```

That's it. `makc.Open()` picks the right backend for the current OS, asks for
permissions if needed, and returns a `*Client`. Every method takes a `context`
so you can cancel mid-sequence.

### A few more recipes

```go
// Move the cursor 100 pixels right and down.
client.Mouse.MoveBy(ctx, 100, 100)

// Type some text.
client.Keyboard.TypeText(ctx, "hello, world")

// Press Cmd+Tab (Windows/Linux: Ctrl+Tab).
client.Keyboard.Combo(ctx, makc.KeyLeftControl, makc.KeyTab)

// Drag from the current spot to (500, 500), curving like a human hand.
client.Mouse.Drag(ctx, makc.ButtonLeft, makc.Point{X: 500, Y: 500},
	makc.NaturalMovement(40, 250*time.Millisecond, 42))
```

### Listening to input

```go
listener, err := client.Listen(ctx, makc.ListenOptions{Mask: makc.ListenAll})
if err != nil { log.Fatal(err) }
defer listener.Close()

for event := range listener.Events {
	if event.Kind == makc.InputEventMouseButton {
		log.Printf("button %s state %s", event.Mouse.Button, event.Mouse.State)
	}
}
```

`Listen` is supported on Windows (low-level hooks and Raw Input), Linux
(evdev), and macOS (CGEventTap). `listener.Stats()` reports delivered/dropped
counters — bump `ListenOptions.Buffer` if `Dropped` is climbing.

---

## Platform setup

Most setup is a one-time permission step. Once granted, `makc.Open()` Just
Works.

**Windows.** Nothing to install. The library picks `InjectMouseInput` /
`InjectKeyboardInput` when the running build of `user32.dll` exports them and
falls back to `SendInput` otherwise. No admin needed for normal injection;
some elevated targets reject input from non-elevated callers — same as any
SendInput-based tool.

**macOS.** Add your binary (or your terminal during dev) to **System Settings
→ Privacy & Security → Accessibility**. Without it, `Open()` succeeds but
injection returns an error pointing you at the missing permission. Listening
also requires this permission.

**Linux (X11).** Either run with access to `/dev/uinput` (uncommon for
non-root users) or run this once and re-login:

```sh
sudo bash scripts/linux-uinput-permissions.sh "$USER"
```

That installs a udev rule giving members of `input` group write access to
`/dev/uinput`. Listening reads `/dev/input/event*` — same permission story.

**Linux (Wayland).** Use the XDG desktop portal backend:

```go
client, err := makc.Open(
    makc.WithMouseXDGPortal(),
    makc.WithKeyboardXDGPortal(),
)
```

The first call shows a one-time GNOME/KDE permission dialog asking the user
to grant remote-desktop input to your app. Subsequent runs reuse the granted
session for the lifetime of the `Client`.

---

## Picking a backend

`makc.Open()` defaults to the right thing per OS. Override only if you need to.

| Platform | Default | Alternatives |
| --- | --- | --- |
| Windows | `MouseInjectionAuto` → `InjectMouseInput` if available, else `SendInput` | `WithMouseSendInput()`, `WithMouseInjectMouseInput()` |
| macOS | CoreGraphics `CGEvent` | (only one) |
| Linux | `/dev/uinput` | `WithMouseXDGPortal()` for Wayland |

Symmetric Options exist for keyboard.

A few useful tuning hints:

```go
// Disable Windows' mouse-move coalescing for high-frequency injection.
makc.WithMouseMotion(makc.MouseMotionNoCoalesce)

// Multi-monitor absolute coordinates against the virtual desktop.
makc.WithMouseMotion(makc.MouseMotionVirtualDesk)

// Tag injected events so listeners can identify your own input.
makc.WithInputTag(0xCAFE)
```

---

## Profiles, sequences, and human-like movement

`Client.Mouse.Click` is the simple path. For deterministic timing or
"human-like" curves, use a profile:

```go
// One click with a 50ms hold.
client.Mouse.ClickWithProfile(ctx, makc.ButtonLeft,
	makc.ClickWithHold(50*time.Millisecond))

// Type text with a randomized but reproducible cadence.
client.Keyboard.TypeTextWithProfile(ctx, "hello",
	makc.VariableTyping(40*time.Millisecond, 120*time.Millisecond, 42))

// Bezier-curved movement instead of teleport.
client.Mouse.MoveToProfile(ctx, makc.Point{X: 500, Y: 500},
	makc.NaturalMovement(60, 400*time.Millisecond, 42))
```

For workflows that mix moves, clicks, pauses, and typing, build an
`InputSequence`:

```go
seq := makc.NewInputSequence(
	makc.MoveStep(makc.Abs(300, 200)),
	makc.PauseStep(80*time.Millisecond),
	makc.ClickStep(makc.ButtonLeft, makc.InstantClick),
	makc.TextStep("makc", makc.InstantTyping),
)
client.Run(ctx, seq)
```

`Fast`, `Balanced`, and `Careful` presets bundle interval timing for the most
common cases:

```go
profile := makc.BalancedInputProfile(42) // seed
client.Keyboard.TypeTextWithProfile(ctx, "hello", profile.Typing)
```

---

## API map

Common entry points:

- `makc.Open(opts...)` → `*Client`
- `Client.Mouse.{Move, MoveTo, MoveBy, Click, DoubleClick, Wheel, HWheel, Drag, Position, State, SystemSpeed, Inject}`
- `Client.Keyboard.{Tap, TapWithHold, Combo, TypeText, ScanTap, State, Inject}`
- `Client.Listen(ctx, opts)` → `*Listener` with `Events`, `Stats`, `Wait`, `Close`
- `Client.Run(ctx, sequence)` / `Client.RunSteps(ctx, steps...)`
- `Client.RuntimeInfo(ctx)` for diagnostics

Movement / timing primitives:

- `InstantMovement`, `LinearMovement`, `EaseInOutMovement`, `NaturalMovement`,
  `NaturalMovementWithJitter`
- `FixedInterval`, `VariableInterval`, `ClickProfile`, `TypingProfile`
- `InstantInputProfile`, `FastInputProfile`, `BalancedInputProfile`,
  `CarefulInputProfile`

Parsing for CLIs / config:

- `ParseKey("ctrl+shift+a")`, `ParseMouseButton("left")`

Full reference:
[pkg.go.dev/github.com/aiwaki/makc](https://pkg.go.dev/github.com/aiwaki/makc).

---

## Examples

```sh
go run ./examples/mouse
go run ./examples/keyboard
go run ./examples/sequence            # tiny relative move only by default
go run ./examples/sequence -click     # opt in to clicking
go run ./examples/sequence -text "hi"
```

---

## Diagnostics

`makc-smoke` is a small CLI that opens a backend and reports what it can do
without injecting anything unless asked:

```sh
go run ./cmd/makc-smoke -runtime-info
go run ./cmd/makc-smoke -capabilities
go run ./cmd/makc-smoke -inject -dx 1 -dy 1
```

Linux portal handshake (no input until you pass `-start`):

```sh
go run ./cmd/makc-portal-handshake
go run ./cmd/makc-portal-handshake -select-devices -start -timeout 300s
```

For full local validation, hardware test scripts, and CI smoke runs (Windows
on Parallels, Linux VMs, etc.) see [`scripts/`](scripts/) and the comments in
each script.

---

## A note on detection-evasion

`makc` deliberately does **not** scrub the operating system's "this event was
injected" markers (`LLMHF_INJECTED` on Windows,
`kCGEventSourceUnixProcessID` on macOS) before forwarding to other hooks
installed in the system. Those flags exist so accessibility software, security
tools, and yes — anti-cheat — can distinguish synthetic from real input.
Stripping them out of shared kernel structures to fool other software is out
of scope.

What `makc` does provide: `WithInputTag` so *your own* listener can identify
*your own* injection, plus `Listener.NormalizeOwnInjected` to clear the flags
on events you produced before your code sees them. That's a callback-private
operation; the kernel struct stays intact.

---

## Versioning, license, security

- Module path is currently `github.com/aiwaki/makc` — no `/v2` yet.
- Release notes: [CHANGELOG.md](CHANGELOG.md).
- Security policy: [SECURITY.md](SECURITY.md).
- Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

The legacy `pkg/types`, `pkg/types/buttons`, `pkg/types/keys` packages are
deprecated compatibility shims. New code should import the root
`github.com/aiwaki/makc` package directly.
