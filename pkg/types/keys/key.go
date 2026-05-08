// Package keys keeps compatibility aliases for the legacy API.
//
// Deprecated: import github.com/aiwaki/makc and use makc.Key constants
// directly.
package keys

import "github.com/aiwaki/makc"

// Key is an alias for makc.Key.
//
// Deprecated: use makc.Key.
type Key = makc.Key

const (
	K1     = makc.Key1
	K2     = makc.Key2
	K3     = makc.Key3
	K4     = makc.Key4
	K5     = makc.Key5
	K6     = makc.Key6
	K7     = makc.Key7
	K8     = makc.Key8
	K9     = makc.Key9
	K0     = makc.Key0
	Minus  = makc.KeyMinus
	Equals = makc.KeyEquals

	F1  = makc.KeyF1
	F2  = makc.KeyF2
	F3  = makc.KeyF3
	F4  = makc.KeyF4
	F5  = makc.KeyF5
	F6  = makc.KeyF6
	F7  = makc.KeyF7
	F8  = makc.KeyF8
	F9  = makc.KeyF9
	F10 = makc.KeyF10
	F11 = makc.KeyF11
	F12 = makc.KeyF12

	A = makc.KeyA
	S = makc.KeyS
	D = makc.KeyD
	F = makc.KeyF
	G = makc.KeyG
	H = makc.KeyH
	J = makc.KeyJ
	K = makc.KeyK
	L = makc.KeyL
	Q = makc.KeyQ
	W = makc.KeyW
	E = makc.KeyE
	R = makc.KeyR
	T = makc.KeyT
	Y = makc.KeyY
	U = makc.KeyU
	I = makc.KeyI
	O = makc.KeyO
	P = makc.KeyP
	Z = makc.KeyZ
	X = makc.KeyX
	C = makc.KeyC
	V = makc.KeyV
	B = makc.KeyB
	N = makc.KeyN
	M = makc.KeyM

	LeftSquareBracket  = makc.KeyLeftSquareBracket
	RightSquareBracket = makc.KeyRightSquareBracket
	BackQuote          = makc.KeyBackQuote
	Backslash          = makc.KeyBackslash

	Semicolon    = makc.KeySemicolon
	SingleQuote  = makc.KeySingleQuote
	Comma        = makc.KeyComma
	Dot          = makc.KeyDot
	QuestionMark = makc.KeyQuestionMark

	Escape       = makc.KeyEscape
	Delete       = makc.KeyDelete
	Tab          = makc.KeyTab
	Enter        = makc.KeyEnter
	Control      = makc.KeyControl
	ControlLeft  = makc.KeyLeftControl
	ControlRight = makc.KeyRightControl
	Shift        = makc.KeyShift
	ShiftLeft    = makc.KeyLeftShift
	ShiftRight   = makc.KeyRightShift
	Space        = makc.KeySpace
	Backspace    = makc.KeyBackspace
	Capslock     = makc.KeyCapsLock
	Insert       = makc.KeyInsert
	Printscreen  = makc.KeyPrintScreen
	End          = makc.KeyEnd
	Home         = makc.KeyHome
	Menu         = makc.KeyMenu
	AltLeft      = makc.KeyLeftAlt
	AltRight     = makc.KeyRightAlt
)

// Keys contains the legacy key set.
//
// Deprecated: use makc.Key constants directly.
var Keys = []Key{
	K1,
	K2,
	K3,
	K4,
	K5,
	K6,
	K7,
	K8,
	K9,
	K0,
	Minus,
	Equals,
	F1,
	F2,
	F3,
	F4,
	F5,
	F6,
	F7,
	F8,
	F9,
	F10,
	F11,
	F12,
	A,
	S,
	D,
	F,
	G,
	H,
	J,
	K,
	L,
	Q,
	W,
	E,
	R,
	T,
	Y,
	U,
	I,
	O,
	P,
	Z,
	X,
	C,
	V,
	B,
	N,
	M,
	LeftSquareBracket,
	RightSquareBracket,
	BackQuote,
	Backslash,
	Semicolon,
	SingleQuote,
	Comma,
	Dot,
	QuestionMark,
	Escape,
	Delete,
	Tab,
	Enter,
	Control,
	ControlLeft,
	ControlRight,
	Shift,
	ShiftLeft,
	ShiftRight,
	Space,
	Backspace,
	Capslock,
	Insert,
	Printscreen,
	End,
	Home,
	Menu,
	AltLeft,
	AltRight,
}
