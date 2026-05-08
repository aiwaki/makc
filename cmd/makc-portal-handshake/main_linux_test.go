//go:build linux

package main

import "testing"

func TestParseDeviceMask(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    uint32
		wantErr bool
	}{
		{name: "names", value: "keyboard,pointer", want: deviceKeyboard | devicePointer},
		{name: "aliases", value: "keys,mouse,touch", want: deviceKeyboard | devicePointer | deviceTouchscreen},
		{name: "numeric", value: "7", want: 7},
		{name: "hex", value: "0x3", want: 3},
		{name: "empty", value: "", wantErr: true},
		{name: "unknown", value: "pen", wantErr: true},
		{name: "none", value: "none", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDeviceMask(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDeviceMask(%q) error = nil, want error", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDeviceMask(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("parseDeviceMask(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
