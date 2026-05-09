# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
for tagged releases.

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
