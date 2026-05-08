package makc

import (
	"reflect"
	"testing"
	"time"
)

func TestFixedIntervalDurations(t *testing.T) {
	got := FixedInterval(12 * time.Millisecond).Durations(3)
	want := []time.Duration{12 * time.Millisecond, 12 * time.Millisecond, 12 * time.Millisecond}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("durations = %v, want %v", got, want)
	}
}

func TestVariableIntervalDurationsAreSeededAndBounded(t *testing.T) {
	profile := VariableInterval(10*time.Millisecond, 20*time.Millisecond, 42)
	a := profile.Durations(8)
	b := profile.Durations(8)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("expected same seed to produce same durations")
	}
	for i, duration := range a {
		if duration < 10*time.Millisecond || duration > 20*time.Millisecond {
			t.Fatalf("duration %d = %s, want within [10ms,20ms]", i, duration)
		}
	}

	c := VariableInterval(10*time.Millisecond, 20*time.Millisecond, 43).Durations(8)
	if reflect.DeepEqual(a, c) {
		t.Fatal("expected different seeds to produce different durations")
	}
}

func TestIntervalProfileNormalizesInvalidDurations(t *testing.T) {
	got := VariableInterval(-10*time.Millisecond, -5*time.Millisecond, 1).Durations(2)
	want := []time.Duration{0, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("negative durations = %v, want %v", got, want)
	}

	got = VariableInterval(20*time.Millisecond, 10*time.Millisecond, 1).Durations(2)
	want = []time.Duration{20 * time.Millisecond, 20 * time.Millisecond}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inverted durations = %v, want %v", got, want)
	}
}
