// Package makc provides no-cgo mouse and keyboard control primitives.
//
// The v2 API is centered around Client, which owns a platform backend and
// exposes Mouse, Keyboard, and low-level listener APIs. Native calls are bound
// directly from Go through purego, with x/sys/windows used for Win32 helpers.
package makc
