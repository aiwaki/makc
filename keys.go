package makc

// Key is a Windows virtual-key code.
type Key uint16

const (
	KeyUnknown Key = 0x00

	KeyLeftButton   Key = 0x01
	KeyRightButton  Key = 0x02
	KeyCancel       Key = 0x03
	KeyMiddleButton Key = 0x04
	KeyXButton1     Key = 0x05
	KeyXButton2     Key = 0x06

	KeyBackspace   Key = 0x08
	KeyTab         Key = 0x09
	KeyClear       Key = 0x0C
	KeyEnter       Key = 0x0D
	KeyShift       Key = 0x10
	KeyControl     Key = 0x11
	KeyAlt         Key = 0x12
	KeyPause       Key = 0x13
	KeyCapsLock    Key = 0x14
	KeyEscape      Key = 0x1B
	KeySpace       Key = 0x20
	KeyPageUp      Key = 0x21
	KeyPageDown    Key = 0x22
	KeyEnd         Key = 0x23
	KeyHome        Key = 0x24
	KeyLeft        Key = 0x25
	KeyUp          Key = 0x26
	KeyRight       Key = 0x27
	KeyDown        Key = 0x28
	KeySelect      Key = 0x29
	KeyPrint       Key = 0x2A
	KeyExecute     Key = 0x2B
	KeyPrintScreen Key = 0x2C
	KeyInsert      Key = 0x2D
	KeyDelete      Key = 0x2E
	KeyHelp        Key = 0x2F

	Key0 Key = 0x30
	Key1 Key = 0x31
	Key2 Key = 0x32
	Key3 Key = 0x33
	Key4 Key = 0x34
	Key5 Key = 0x35
	Key6 Key = 0x36
	Key7 Key = 0x37
	Key8 Key = 0x38
	Key9 Key = 0x39

	KeyA Key = 0x41
	KeyB Key = 0x42
	KeyC Key = 0x43
	KeyD Key = 0x44
	KeyE Key = 0x45
	KeyF Key = 0x46
	KeyG Key = 0x47
	KeyH Key = 0x48
	KeyI Key = 0x49
	KeyJ Key = 0x4A
	KeyK Key = 0x4B
	KeyL Key = 0x4C
	KeyM Key = 0x4D
	KeyN Key = 0x4E
	KeyO Key = 0x4F
	KeyP Key = 0x50
	KeyQ Key = 0x51
	KeyR Key = 0x52
	KeyS Key = 0x53
	KeyT Key = 0x54
	KeyU Key = 0x55
	KeyV Key = 0x56
	KeyW Key = 0x57
	KeyX Key = 0x58
	KeyY Key = 0x59
	KeyZ Key = 0x5A

	KeyLeftWindows  Key = 0x5B
	KeyRightWindows Key = 0x5C
	KeyApps         Key = 0x5D
	KeySleep        Key = 0x5F

	KeyNumpad0   Key = 0x60
	KeyNumpad1   Key = 0x61
	KeyNumpad2   Key = 0x62
	KeyNumpad3   Key = 0x63
	KeyNumpad4   Key = 0x64
	KeyNumpad5   Key = 0x65
	KeyNumpad6   Key = 0x66
	KeyNumpad7   Key = 0x67
	KeyNumpad8   Key = 0x68
	KeyNumpad9   Key = 0x69
	KeyMultiply  Key = 0x6A
	KeyAdd       Key = 0x6B
	KeySeparator Key = 0x6C
	KeySubtract  Key = 0x6D
	KeyDecimal   Key = 0x6E
	KeyDivide    Key = 0x6F

	KeyF1  Key = 0x70
	KeyF2  Key = 0x71
	KeyF3  Key = 0x72
	KeyF4  Key = 0x73
	KeyF5  Key = 0x74
	KeyF6  Key = 0x75
	KeyF7  Key = 0x76
	KeyF8  Key = 0x77
	KeyF9  Key = 0x78
	KeyF10 Key = 0x79
	KeyF11 Key = 0x7A
	KeyF12 Key = 0x7B

	KeyNumLock      Key = 0x90
	KeyScrollLock   Key = 0x91
	KeyLeftShift    Key = 0xA0
	KeyRightShift   Key = 0xA1
	KeyLeftControl  Key = 0xA2
	KeyRightControl Key = 0xA3
	KeyLeftAlt      Key = 0xA4
	KeyRightAlt     Key = 0xA5

	KeySemicolon          Key = 0xBA
	KeyEquals             Key = 0xBB
	KeyComma              Key = 0xBC
	KeyMinus              Key = 0xBD
	KeyDot                Key = 0xBE
	KeySlash              Key = 0xBF
	KeyBackQuote          Key = 0xC0
	KeyLeftSquareBracket  Key = 0xDB
	KeyBackslash          Key = 0xDC
	KeyRightSquareBracket Key = 0xDD
	KeySingleQuote        Key = 0xDE

	KeyQuestionMark = KeySlash
	KeyMenu         = KeyAlt
)

var keyNames = map[Key]string{
	KeyUnknown:            "unknown",
	KeyLeftButton:         "leftbutton",
	KeyRightButton:        "rightbutton",
	KeyCancel:             "cancel",
	KeyMiddleButton:       "middlebutton",
	KeyXButton1:           "xbutton1",
	KeyXButton2:           "xbutton2",
	KeyBackspace:          "backspace",
	KeyTab:                "tab",
	KeyClear:              "clear",
	KeyEnter:              "enter",
	KeyShift:              "shift",
	KeyControl:            "control",
	KeyAlt:                "alt",
	KeyPause:              "pause",
	KeyCapsLock:           "capslock",
	KeyEscape:             "escape",
	KeySpace:              "space",
	KeyPageUp:             "pageup",
	KeyPageDown:           "pagedown",
	KeyEnd:                "end",
	KeyHome:               "home",
	KeyLeft:               "left",
	KeyUp:                 "up",
	KeyRight:              "right",
	KeyDown:               "down",
	KeySelect:             "select",
	KeyPrint:              "print",
	KeyExecute:            "execute",
	KeyPrintScreen:        "printscreen",
	KeyInsert:             "insert",
	KeyDelete:             "delete",
	KeyHelp:               "help",
	Key0:                  "0",
	Key1:                  "1",
	Key2:                  "2",
	Key3:                  "3",
	Key4:                  "4",
	Key5:                  "5",
	Key6:                  "6",
	Key7:                  "7",
	Key8:                  "8",
	Key9:                  "9",
	KeyA:                  "a",
	KeyB:                  "b",
	KeyC:                  "c",
	KeyD:                  "d",
	KeyE:                  "e",
	KeyF:                  "f",
	KeyG:                  "g",
	KeyH:                  "h",
	KeyI:                  "i",
	KeyJ:                  "j",
	KeyK:                  "k",
	KeyL:                  "l",
	KeyM:                  "m",
	KeyN:                  "n",
	KeyO:                  "o",
	KeyP:                  "p",
	KeyQ:                  "q",
	KeyR:                  "r",
	KeyS:                  "s",
	KeyT:                  "t",
	KeyU:                  "u",
	KeyV:                  "v",
	KeyW:                  "w",
	KeyX:                  "x",
	KeyY:                  "y",
	KeyZ:                  "z",
	KeyLeftWindows:        "leftwindows",
	KeyRightWindows:       "rightwindows",
	KeyApps:               "apps",
	KeySleep:              "sleep",
	KeyNumpad0:            "numpad0",
	KeyNumpad1:            "numpad1",
	KeyNumpad2:            "numpad2",
	KeyNumpad3:            "numpad3",
	KeyNumpad4:            "numpad4",
	KeyNumpad5:            "numpad5",
	KeyNumpad6:            "numpad6",
	KeyNumpad7:            "numpad7",
	KeyNumpad8:            "numpad8",
	KeyNumpad9:            "numpad9",
	KeyMultiply:           "multiply",
	KeyAdd:                "add",
	KeySeparator:          "separator",
	KeySubtract:           "subtract",
	KeyDecimal:            "decimal",
	KeyDivide:             "divide",
	KeyF1:                 "f1",
	KeyF2:                 "f2",
	KeyF3:                 "f3",
	KeyF4:                 "f4",
	KeyF5:                 "f5",
	KeyF6:                 "f6",
	KeyF7:                 "f7",
	KeyF8:                 "f8",
	KeyF9:                 "f9",
	KeyF10:                "f10",
	KeyF11:                "f11",
	KeyF12:                "f12",
	KeyNumLock:            "numlock",
	KeyScrollLock:         "scrolllock",
	KeyLeftControl:        "controlleft",
	KeyRightControl:       "controlright",
	KeyLeftShift:          "shiftleft",
	KeyRightShift:         "shiftright",
	KeyLeftAlt:            "altleft",
	KeyRightAlt:           "altright",
	KeyMinus:              "-",
	KeyEquals:             "=",
	KeyLeftSquareBracket:  "[",
	KeyRightSquareBracket: "]",
	KeyBackQuote:          "`",
	KeyBackslash:          "\\",
	KeySemicolon:          ";",
	KeySingleQuote:        "'",
	KeyComma:              ",",
	KeyDot:                ".",
	KeySlash:              "/",
}

// String returns a stable, lower-case name for a key.
func (k Key) String() string {
	if name, ok := keyNames[k]; ok {
		return name
	}
	return "unknown"
}
