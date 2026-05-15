# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
for tagged releases.

## [0.2.0] - 2026-05-15

Quality pass: cross-platform correctness, performance, and a new Wayland
injection backend. Public API remains backward compatible; per-OS Option
helpers are added alongside the existing cross-platform constants.

### Added

- `WithMouseXDGPortal` / `WithKeyboardXDGPortal` Options route Linux
  injection through the XDG Desktop Portal RemoteDesktop interface,
  unlocking Wayland compositors. First Open call presents a one-time
  permission dialog; the granted session is reused for the lifetime of
  the Client.
- `WithMouseMotion` Option exposes Win32 `MOUSEEVENTF_MOVE_NOCOALESCE`
  and `MOUSEEVENTF_VIRTUALDESK` for high-frequency injection and
  multi-monitor absolute coordinates.
- `Mouse.SystemSpeed(ctx)` reports the OS pointer-speed setting on
  Windows (1–20 from `SPI_GETMOUSESPEED`); returns `ErrUnsupported` on
  other platforms.
- `Listener.Stats()` exposes atomic `Delivered` / `Dropped` counters so
  full-buffer drops are observable.
- macOS input listening implemented via `CGEventTap` on a dedicated
  `CFRunLoop`; previously returned `ErrUnsupported`.
- Per-OS Option helpers (`WithMouseSendInput`, `WithKeyboardCGEvent`,
  `WithMouseUInput`, …) for compile-time safety against
  wrong-platform backend selection. Cross-platform constants remain
  available.

### Fixed

- **Race on `Client.closed`.** Switched to `atomic.Bool` and gated
  backend `Close` with `sync.Once`; safe to call from multiple
  goroutines and idempotent.
- **Windows `GetLastError` reliability.** Migrated all Win32 calls to
  `windows.LazyProc.Call` so `syscall.Errno` is read from the same
  syscall site instead of a follow-up TLS read that the Go scheduler
  could interleave.
- **Windows `NewCallback` exhaustion.** Listen now allocates exactly
  three thunk slots for the lifetime of the backend (one per LL hook
  plus one raw-input WndProc) instead of per-`Listen` invocation,
  preventing eventual panic in long-running open/close loops.
- **Windows raw input cleanup.** `DefWindowProc` is now called for
  every `WM_INPUT` message regardless of `RIM_INPUT` /
  `RIM_INPUTSINK`, preventing kernel raw-input buffer accumulation.
- **macOS `CGEventTap` actually delivers events.** Source attached to
  `kCFRunLoopDefaultMode` directly (common-modes set is empty on a
  fresh non-main runloop and the source ended up attached to nothing);
  tap moved to `kCGSessionEventTap` so same-process inject→listen
  cycles produce callbacks; `CGEventTapIsEnabled` probes report
  missing Input Monitoring permission with a clear error.
- **macOS modifier-key events.** `CGEventFlagsChanged` now emits
  proper press/release events (derived from the post-change flag word
  and the changed keycode) instead of being silently dropped.
- **macOS injected/own detection.** Input tag is now stamped on every
  injected `CGEvent` (via `kCGEventSourceUserData`) and the listener
  reads `kCGEventSourceUnixProcessID` to populate `Injected`,
  matching Win32 `LLMHF_INJECTED` semantics.
- **Linux `KeyState` / `MouseButtonState` perf.** Per-call open-all-evdev
  pattern (~150 syscalls per query) replaced with a lazy fd cache
  populated once on first state query and reused.
- **Linux uinput readiness.** Replaced fixed `time.Sleep(20ms)` after
  `UI_DEV_CREATE` with `UI_GET_SYSNAME` plus sysfs polling on a
  bounded deadline; falls back to the legacy sleep on pre-3.15
  kernels.
- **Linux evdev disconnect surfacing.** Listener now returns a clear
  error when every input device disconnects mid-run instead of
  silently completing as if shutdown were requested.
- **`Listener.Wait` re-entrancy.** Repeat callers and concurrent
  callers now observe the same cached exit error instead of one
  caller draining the underlying channel and the rest blocking
  forever.

### Performance

- macOS injection: `CGEventSource` cached for the lifetime of the
  backend instead of created per event.
- Windows `Mouse.Position`: served from an atomic cursor cache
  populated by an active mouse hook listener, avoiding the
  `GetCursorPos` syscall while listening.

### Documentation

- README rewritten for first-time users: leads with a five-line
  "Hello, click" example, per-OS setup as one paragraph each,
  diagnostics relegated to brief pointers, and an explicit section
  stating that injected-flag scrubbing is out of scope.
- Mascot added to the repository.

## [0.1.2] - 2026-05-09

Code hardening release.

### Fixed

- Made `Client.Close` safe for zero-value clients.
- Made `Client.Listen` handle a nil context consistently with the rest of the
  API.
- Rejected unknown `ListenMask` bits before starting a backend listener.

### Changed

- Moved Linux-only display helper wiring behind Linux build constraints.
- Lowered the module's Go version from `1.25.0` to `1.23.0` by using
  `golang.org/x/sys v0.35.0`.

## [0.1.1] - 2026-05-09

Public repository hardening release.

### Changed

- Removed generated assistant/ECC files from the public module tree.
- Added `CONTRIBUTING.md`, `SECURITY.md`, issue templates, and a pull request
  template.
- Updated `.gitignore` to keep local assistant workspaces out of the public Go
  module.
- Updated `LICENSE` with the current maintainer copyright line.

### Repository

- Enabled GitHub Issues.
- Enabled GitHub secret scanning and push protection where available.

## [0.1.0] - 2026-05-09

Initial public rewrite release.

### Added

- New no-cgo root package API built around `Client`, `Mouse`, and `Keyboard`.
- Windows mouse injection through `InjectMouseInput` when available, with
  `SendInput` fallback.
- Windows keyboard injection through `InjectKeyboardInput` when available, with
  `SendInput` fallback.
- macOS mouse and keyboard injection through CoreGraphics `CGEvent`.
- Linux mouse and keyboard injection through `/dev/uinput`.
- Optional Linux X11 support for cursor position, screen size, and absolute
  cursor movement.
- Windows low-level hook listener and Raw Input listener backends.
- Linux evdev state and listener support.
- Mouse movement, button, wheel, drag, and batch injection helpers.
- Keyboard virtual-key, scan-code, combo, and Unicode text helpers.
- Deterministic timing profiles for clicks, typing, key holds, and cursor
  movement.
- Reusable `Instant`, `Fast`, `Balanced`, and `Careful` input profile presets.
- Seeded natural movement paths with reproducible pauses and path variation.
- `InputSequence` helpers for mixed mouse, keyboard, and pause workflows.
- Runtime diagnostics for display stacks, backend dependencies, Linux devices,
  and desktop portal hints.
- Smoke commands and Parallels helpers for local Windows, macOS, and Linux
  validation.
- Read-only Linux XDG Desktop Portal and GNOME RemoteDesktop diagnostic tools.
- String parsing helpers for key and mouse button names.
- Backwards compatibility aliases for the legacy `pkg/types`,
  `pkg/types/buttons`, and `pkg/types/keys` packages.

### Changed

- Replaced the legacy C header, embedded DLL, and `skip_hook` submodule with
  pure Go backend bindings.
- Moved the module path to `github.com/aiwaki/makc`.
- Refreshed README, examples, package overview, and exported GoDoc comments for
  the rewritten API.

### Notes

- The module path is `github.com/aiwaki/makc`; this release intentionally does
  not use a `/v2` suffix.
- Linux Wayland absolute cursor control and Unicode text injection still need a
  display-server-specific backend and currently return `ErrUnsupported`.
- GNOME/XDG Desktop Portal support is diagnostic-only in this release; a real
  libei backend remains future work.
