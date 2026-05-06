// Package makc provides no-cgo mouse and keyboard control primitives for
// Windows.
//
// The v2 API is centered around Client, which owns a platform backend and
// exposes Mouse, Keyboard, and low-level listener APIs. Windows calls are bound
// directly from Go through purego and x/sys/windows.
package makc
