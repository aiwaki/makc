package makc

import (
	"reflect"
	"testing"
	"time"
)

func TestInputProfilePresets(t *testing.T) {
	tests := []struct {
		name          string
		profile       InputProfile
		wantSteps     int
		wantMoveTime  time.Duration
		wantClickHold time.Duration
		wantKeyHold   time.Duration
	}{
		{
			name:          "instant",
			profile:       InstantInputProfile(),
			wantSteps:     1,
			wantMoveTime:  0,
			wantClickHold: 0,
			wantKeyHold:   0,
		},
		{
			name:          "fast",
			profile:       FastInputProfile(42),
			wantSteps:     8,
			wantMoveTime:  90 * time.Millisecond,
			wantClickHold: 18 * time.Millisecond,
			wantKeyHold:   18 * time.Millisecond,
		},
		{
			name:          "balanced",
			profile:       BalancedInputProfile(42),
			wantSteps:     14,
			wantMoveTime:  180 * time.Millisecond,
			wantClickHold: 35 * time.Millisecond,
			wantKeyHold:   35 * time.Millisecond,
		},
		{
			name:          "careful",
			profile:       CarefulInputProfile(42),
			wantSteps:     22,
			wantMoveTime:  320 * time.Millisecond,
			wantClickHold: 55 * time.Millisecond,
			wantKeyHold:   50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.profile.Movement.Steps != tt.wantSteps {
				t.Fatalf("movement steps = %d, want %d", tt.profile.Movement.Steps, tt.wantSteps)
			}
			if tt.profile.Movement.Duration != tt.wantMoveTime {
				t.Fatalf("movement duration = %s, want %s", tt.profile.Movement.Duration, tt.wantMoveTime)
			}
			if tt.profile.Click.Count != 1 {
				t.Fatalf("click count = %d, want 1", tt.profile.Click.Count)
			}
			if tt.profile.Click.Hold != tt.wantClickHold {
				t.Fatalf("click hold = %s, want %s", tt.profile.Click.Hold, tt.wantClickHold)
			}
			if tt.profile.DoubleClick.Count != 2 {
				t.Fatalf("double-click count = %d, want 2", tt.profile.DoubleClick.Count)
			}
			if tt.profile.KeyHold != tt.wantKeyHold {
				t.Fatalf("key hold = %s, want %s", tt.profile.KeyHold, tt.wantKeyHold)
			}
		})
	}
}

func TestInputProfileEventHelpers(t *testing.T) {
	profile := BalancedInputProfile(7)
	start := Point{X: 10, Y: 20}
	end := Point{X: 110, Y: 60}

	if got, want := profile.MovementEvents(start, end), profile.Movement.Events(start, end); !reflect.DeepEqual(got, want) {
		t.Fatalf("MovementEvents() = %+v, want %+v", got, want)
	}
	if got, want := profile.ClickEvents(ButtonLeft), profile.Click.Events(ButtonLeft); !reflect.DeepEqual(got, want) {
		t.Fatalf("ClickEvents() = %+v, want %+v", got, want)
	}
	if got, want := profile.DoubleClickEvents(ButtonLeft), profile.DoubleClick.Events(ButtonLeft); !reflect.DeepEqual(got, want) {
		t.Fatalf("DoubleClickEvents() = %+v, want %+v", got, want)
	}
	if got, want := profile.KeyTapEvents(KeyEnter), KeyTapEventsWithHold(KeyEnter, profile.KeyHold); !reflect.DeepEqual(got, want) {
		t.Fatalf("KeyTapEvents() = %+v, want %+v", got, want)
	}
	if got, want := profile.TextEvents("ab"), profile.Typing.Events("ab"); !reflect.DeepEqual(got, want) {
		t.Fatalf("TextEvents() = %+v, want %+v", got, want)
	}
}

func TestInputProfilePresetsAreSeeded(t *testing.T) {
	start := Point{X: 0, Y: 0}
	end := Point{X: 320, Y: 140}

	a := BalancedInputProfile(42)
	b := BalancedInputProfile(42)
	if !reflect.DeepEqual(a.MovementEvents(start, end), b.MovementEvents(start, end)) {
		t.Fatal("expected same movement seed to produce same events")
	}
	if !reflect.DeepEqual(a.TextEvents("abcd"), b.TextEvents("abcd")) {
		t.Fatal("expected same typing seed to produce same events")
	}

	c := BalancedInputProfile(43)
	if reflect.DeepEqual(a.MovementEvents(start, end), c.MovementEvents(start, end)) {
		t.Fatal("expected different movement seeds to produce different events")
	}
	if reflect.DeepEqual(a.TextEvents("abcd"), c.TextEvents("abcd")) {
		t.Fatal("expected different typing seeds to produce different events")
	}
}
