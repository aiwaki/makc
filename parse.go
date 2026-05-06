package makc

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseMouseButton returns a mouse button by its stable lower-case name.
func ParseMouseButton(name string) (MouseButton, error) {
	raw := strings.ToLower(strings.TrimSpace(name))
	if button, ok := mouseButtonsByName[raw]; ok {
		return button, nil
	}
	if button, ok := mouseButtonsByName[normalizeInputName(raw)]; ok {
		return button, nil
	}
	return 0, fmt.Errorf("makc: unknown mouse button %q", name)
}

// ParseKey returns a virtual-key code by its stable lower-case name.
//
// Numeric values are accepted with Go-style base prefixes, for example 0x41
// for KeyA.
func ParseKey(name string) (Key, error) {
	raw := strings.ToLower(strings.TrimSpace(name))
	if key, ok := keysByName[raw]; ok {
		return key, nil
	}
	if key, ok := keysByName[normalizeInputName(raw)]; ok {
		return key, nil
	}
	value, err := strconv.ParseUint(raw, 0, 16)
	if err == nil {
		key := Key(value)
		if key != KeyUnknown {
			return key, nil
		}
	}
	return KeyUnknown, fmt.Errorf("makc: unknown key %q", name)
}

var mouseButtonsByName = buildNameMap(map[string]MouseButton{
	"left":         ButtonLeft,
	"buttonleft":   ButtonLeft,
	"leftbutton":   ButtonLeft,
	"right":        ButtonRight,
	"buttonright":  ButtonRight,
	"rightbutton":  ButtonRight,
	"middle":       ButtonMiddle,
	"buttonmiddle": ButtonMiddle,
	"middlebutton": ButtonMiddle,
	"x1":           ButtonX1,
	"buttonx1":     ButtonX1,
	"xbutton1":     ButtonX1,
	"x2":           ButtonX2,
	"buttonx2":     ButtonX2,
	"xbutton2":     ButtonX2,
	"side":         ButtonSide,
	"buttonside":   ButtonSide,
	"sidebutton":   ButtonSide,
})

var keysByName = func() map[string]Key {
	names := make(map[string]Key, len(keyNames)+48)
	for key, name := range keyNames {
		if key != KeyUnknown {
			addName(names, name, key)
		}
	}

	aliases := map[string]Key{
		"esc":          KeyEscape,
		"return":       KeyEnter,
		"ctrl":         KeyControl,
		"lctrl":        KeyLeftControl,
		"rctrl":        KeyRightControl,
		"leftcontrol":  KeyLeftControl,
		"rightcontrol": KeyRightControl,
		"leftctrl":     KeyLeftControl,
		"rightctrl":    KeyRightControl,
		"lshift":       KeyLeftShift,
		"rshift":       KeyRightShift,
		"leftshift":    KeyLeftShift,
		"rightshift":   KeyRightShift,
		"lalt":         KeyLeftAlt,
		"ralt":         KeyRightAlt,
		"leftalt":      KeyLeftAlt,
		"rightalt":     KeyRightAlt,
		"win":          KeyLeftWindows,
		"lwin":         KeyLeftWindows,
		"rwin":         KeyRightWindows,
		"leftwin":      KeyLeftWindows,
		"rightwin":     KeyRightWindows,
		"windows":      KeyLeftWindows,
		"cmd":          KeyLeftWindows,
		"super":        KeyLeftWindows,
		"app":          KeyApps,
		"contextmenu":  KeyApps,
		"menu":         KeyMenu,
		"pgup":         KeyPageUp,
		"pgdn":         KeyPageDown,
		"del":          KeyDelete,
		"ins":          KeyInsert,
		"prtsc":        KeyPrintScreen,
		"printscr":     KeyPrintScreen,
		"slash":        KeySlash,
		"questionmark": KeyQuestionMark,
	}
	for name, key := range aliases {
		addName(names, name, key)
	}

	return names
}()

func buildNameMap[T comparable](values map[string]T) map[string]T {
	names := make(map[string]T, len(values)*2)
	for name, value := range values {
		addName(names, name, value)
	}
	return names
}

func addName[T comparable](names map[string]T, name string, value T) {
	raw := strings.ToLower(strings.TrimSpace(name))
	if raw == "" {
		return
	}
	names[raw] = value
	if normalized := normalizeInputName(raw); normalized != "" {
		names[normalized] = value
	}
}

func normalizeInputName(name string) string {
	return strings.NewReplacer(" ", "", "_", "", "-", "").Replace(name)
}
