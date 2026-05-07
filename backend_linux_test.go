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
		KeyA:           30,
		KeyEnter:       28,
		KeyLeftControl: 29,
		KeyRightAlt:    100,
		KeyF12:         88,
		KeyLeft:        105,
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
