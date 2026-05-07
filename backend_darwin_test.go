//go:build darwin

package makc

import "testing"

func TestDarwinAPIEventCreation(t *testing.T) {
	api, err := newDarwinAPI()
	if err != nil {
		t.Fatal(err)
	}

	event := api.cgEventCreate(0)
	if event == 0 {
		t.Fatal("CGEventCreate returned 0")
	}
	_ = api.cgEventGetLocation(event)
	api.cfRelease(event)

	event = api.cgEventCreateMouseEvent(0, cgEventMouseMoved, cgPoint{X: 1, Y: 1}, cgMouseButtonLeft)
	if event == 0 {
		t.Fatal("CGEventCreateMouseEvent returned 0")
	}
	api.cfRelease(event)

	event = api.cgEventCreateScrollWheelEvent(0, cgScrollEventUnitLine, 2, int32(1), int32(0))
	if event == 0 {
		t.Fatal("CGEventCreateScrollWheelEvent returned 0")
	}
	api.cfRelease(event)

	event = api.cgEventCreateKeyboardEvent(0, 0, true)
	if event == 0 {
		t.Fatal("CGEventCreateKeyboardEvent returned 0")
	}
	units := []uint16{'a'}
	api.cgEventKeyboardSetUnicodeString(event, uintptr(len(units)), &units[0])
	api.cfRelease(event)
}

func TestDarwinKeyCode(t *testing.T) {
	tests := map[Key]uint16{
		KeyA:            0x00,
		KeyEnter:        0x24,
		KeyLeftControl:  0x3B,
		KeyRightControl: 0x3E,
		KeyF1:           0x7A,
		KeyLeft:         0x7B,
	}
	for key, want := range tests {
		got, err := darwinKeyCode(key)
		if err != nil {
			t.Fatalf("darwinKeyCode(%s) error = %v", key, err)
		}
		if got != want {
			t.Fatalf("darwinKeyCode(%s) = 0x%X, want 0x%X", key, got, want)
		}
	}
}

func TestDarwinMouseButton(t *testing.T) {
	tests := map[MouseButton]uint32{
		ButtonLeft:   cgMouseButtonLeft,
		ButtonRight:  cgMouseButtonRight,
		ButtonMiddle: cgMouseButtonCenter,
		ButtonX1:     3,
		ButtonX2:     4,
	}
	for button, want := range tests {
		got, err := darwinMouseButton(button)
		if err != nil {
			t.Fatalf("darwinMouseButton(%s) error = %v", button, err)
		}
		if got != want {
			t.Fatalf("darwinMouseButton(%s) = %d, want %d", button, got, want)
		}
	}
}

func TestDarwinWheelClicks(t *testing.T) {
	tests := map[int]int32{
		0:              0,
		1:              1,
		WheelDelta:     1,
		-WheelDelta:    -1,
		2 * WheelDelta: 2,
	}
	for delta, want := range tests {
		if got := darwinWheelClicks(delta); got != want {
			t.Fatalf("darwinWheelClicks(%d) = %d, want %d", delta, got, want)
		}
	}
}
