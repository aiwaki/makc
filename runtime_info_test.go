package makc

import "testing"

func TestUnixDisplayServer(t *testing.T) {
	tests := []struct {
		name           string
		sessionType    string
		display        string
		waylandDisplay string
		want           DisplayServer
	}{
		{
			name:           "wayland session wins over Xwayland display",
			sessionType:    "wayland",
			display:        ":0",
			waylandDisplay: "wayland-0",
			want:           DisplayServerWayland,
		},
		{
			name:           "x11 session",
			sessionType:    "x11",
			display:        ":0",
			waylandDisplay: "",
			want:           DisplayServerX11,
		},
		{
			name:           "wayland display without session type",
			sessionType:    "",
			display:        "",
			waylandDisplay: "wayland-0",
			want:           DisplayServerWayland,
		},
		{
			name:           "x11 display without session type",
			sessionType:    "",
			display:        ":1",
			waylandDisplay: "",
			want:           DisplayServerX11,
		},
		{
			name:           "headless",
			sessionType:    "",
			display:        "",
			waylandDisplay: "",
			want:           DisplayServerHeadless,
		},
		{
			name:           "tty",
			sessionType:    "tty",
			display:        "",
			waylandDisplay: "",
			want:           DisplayServerHeadless,
		},
		{
			name:           "unknown explicit session",
			sessionType:    "mir",
			display:        "",
			waylandDisplay: "",
			want:           DisplayServerUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unixDisplayServer(tt.sessionType, tt.display, tt.waylandDisplay)
			if got != tt.want {
				t.Fatalf("unixDisplayServer() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestInspectRuntimeBasicFields(t *testing.T) {
	info := InspectRuntime()
	if info.OS == "" {
		t.Fatal("InspectRuntime().OS is empty")
	}
	if info.Arch == "" {
		t.Fatal("InspectRuntime().Arch is empty")
	}
	if info.Display.Server == "" {
		t.Fatal("InspectRuntime().Display.Server is empty")
	}
}
