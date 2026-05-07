//go:build linux

package makc

import (
	"os"
	"testing"
	"unsafe"
)

func TestLinuxBackendStrings(t *testing.T) {
	if got := MouseInjectionUInput.String(); got != "uinput" {
		t.Fatalf("MouseInjectionUInput.String() = %q, want uinput", got)
	}
	if got := KeyboardInjectionUInput.String(); got != "uinput" {
		t.Fatalf("KeyboardInjectionUInput.String() = %q, want uinput", got)
	}
}

func TestLinuxMouseButton(t *testing.T) {
	tests := map[MouseButton]uint16{
		ButtonLeft:   linuxBtnLeft,
		ButtonRight:  linuxBtnRight,
		ButtonMiddle: linuxBtnMiddle,
		ButtonX1:     linuxBtnSide,
		ButtonX2:     linuxBtnExtra,
	}
	for button, want := range tests {
		got, err := linuxMouseButton(button)
		if err != nil {
			t.Fatalf("linuxMouseButton(%s) error = %v", button, err)
		}
		if got != want {
			t.Fatalf("linuxMouseButton(%s) = %d, want %d", button, got, want)
		}
	}
}

func TestLinuxKeyCode(t *testing.T) {
	tests := map[Key]uint16{
		KeyA:            30,
		KeyEnter:        28,
		KeyLeftControl:  29,
		KeyRightControl: 97,
		KeyRightAlt:     100,
		KeyF12:          88,
		KeyLeft:         105,
	}
	for key, want := range tests {
		got, err := linuxKeyCode(key)
		if err != nil {
			t.Fatalf("linuxKeyCode(%s) error = %v", key, err)
		}
		if got != want {
			t.Fatalf("linuxKeyCode(%s) = %d, want %d", key, got, want)
		}
	}
}

func TestLinuxKeyFromCode(t *testing.T) {
	tests := map[uint16]Key{
		30:  KeyA,
		42:  KeyLeftShift,
		54:  KeyRightShift,
		97:  KeyRightControl,
		100: KeyRightAlt,
		105: KeyLeft,
	}
	for code, want := range tests {
		got, ok := linuxKeyFromCode(code)
		if !ok {
			t.Fatalf("linuxKeyFromCode(%d) ok = false", code)
		}
		if got != want {
			t.Fatalf("linuxKeyFromCode(%d) = %s, want %s", code, got, want)
		}
	}
}

func TestLinuxMouseButtonFromCode(t *testing.T) {
	tests := map[uint16]MouseButton{
		linuxBtnLeft:   ButtonLeft,
		linuxBtnRight:  ButtonRight,
		linuxBtnMiddle: ButtonMiddle,
		linuxBtnSide:   ButtonX1,
		linuxBtnExtra:  ButtonX2,
	}
	for code, want := range tests {
		got, ok := linuxMouseButtonFromCode(code)
		if !ok {
			t.Fatalf("linuxMouseButtonFromCode(%d) ok = false", code)
		}
		if got != want {
			t.Fatalf("linuxMouseButtonFromCode(%d) = %s, want %s", code, got, want)
		}
	}
}

func TestLinuxBitSet(t *testing.T) {
	bits := []byte{0b0000_0010, 0b1000_0000}
	if !bitSet(bits, 1) {
		t.Fatal("bitSet(bits, 1) = false, want true")
	}
	if !bitSet(bits, 15) {
		t.Fatal("bitSet(bits, 15) = false, want true")
	}
	if bitSet(bits, 2) {
		t.Fatal("bitSet(bits, 2) = true, want false")
	}
	if bitSet(bits, 16) {
		t.Fatal("bitSet(bits, 16) = true, want false")
	}
}

func TestLinuxUInputConstants(t *testing.T) {
	if uiDevCreate != 0x5501 {
		t.Fatalf("uiDevCreate = 0x%X, want 0x5501", uiDevCreate)
	}
	if uiSetEvBit != 0x40045564 {
		t.Fatalf("uiSetEvBit = 0x%X, want 0x40045564", uiSetEvBit)
	}
	if unsafe.Sizeof(uinputUserDev{}) != 1116 {
		t.Fatalf("sizeof(uinputUserDev) = %d, want 1116", unsafe.Sizeof(uinputUserDev{}))
	}
	if evIOCGKey(96) != 0x80604518 {
		t.Fatalf("evIOCGKey(96) = 0x%X, want 0x80604518", evIOCGKey(96))
	}
}

func TestLinuxUInputDeviceOpenIntegration(t *testing.T) {
	if os.Getenv("MAKC_TEST_UINPUT") != "1" {
		t.Skip("set MAKC_TEST_UINPUT=1 to open /dev/uinput")
	}

	device, err := newUInputDevice()
	if err != nil {
		t.Fatal(err)
	}
	if err := device.Close(); err != nil {
		t.Fatal(err)
	}
}
