package main

import (
	"testing"
	"time"

	"github.com/aiwaki/makc"
)

func TestParseTypingProfile(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		delay   time.Duration
		min     time.Duration
		max     time.Duration
		want    []makc.KeyboardEvent
		wantErr bool
	}{
		{
			name:  "instant",
			value: "instant",
			want:  []makc.KeyboardEvent{makc.TextEvent("a"), makc.TextEvent("b")},
		},
		{
			name:  "fixed",
			value: "fixed",
			delay: 10 * time.Millisecond,
			want: []makc.KeyboardEvent{
				makc.TextEvent("a"),
				makc.KeyboardPauseEvent(10 * time.Millisecond),
				makc.TextEvent("b"),
			},
		},
		{
			name:  "variable",
			value: "variable",
			min:   10 * time.Millisecond,
			max:   20 * time.Millisecond,
			want: []makc.KeyboardEvent{
				makc.TextEvent("a"),
				makc.KeyboardPauseEvent(17*time.Millisecond + 155986*time.Nanosecond),
				makc.TextEvent("b"),
			},
		},
		{name: "unknown", value: "burst", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := parseTypingProfile(tt.value, tt.delay, tt.min, tt.max, 1)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseTypingProfile() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTypingProfile() error = %v", err)
			}
			got := profile.Events("ab")
			if len(got) != len(tt.want) {
				t.Fatalf("len(events) = %d, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("event %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTypingProfileNameDefault(t *testing.T) {
	if got := typingProfileName(""); got != "instant" {
		t.Fatalf("typingProfileName(\"\") = %q, want instant", got)
	}
	if got := typingProfileName(" Variable "); got != "variable" {
		t.Fatalf("typingProfileName() = %q, want variable", got)
	}
}
