package makc

import (
	"math/rand"
	"time"
)

// IntervalProfile generates deterministic delays between input events.
type IntervalProfile struct {
	// Min is the lower bound for generated delays.
	Min time.Duration

	// Max is the upper bound for generated delays.
	Max time.Duration

	// Seed makes variable delays reproducible.
	Seed int64
}

// FixedInterval creates a profile that returns the same delay every time.
func FixedInterval(delay time.Duration) IntervalProfile {
	return IntervalProfile{
		Min: delay,
		Max: delay,
	}
}

// VariableInterval creates a seeded profile that returns delays in [min, max].
func VariableInterval(min, max time.Duration, seed int64) IntervalProfile {
	return IntervalProfile{
		Min:  min,
		Max:  max,
		Seed: seed,
	}
}

// Durations returns count deterministic delays.
func (p IntervalProfile) Durations(count int) []time.Duration {
	if count <= 0 {
		return nil
	}
	p = p.normalized()
	durations := make([]time.Duration, count)
	if p.Min == p.Max {
		for i := range durations {
			durations[i] = p.Min
		}
		return durations
	}

	rng := rand.New(rand.NewSource(p.Seed))
	span := int64(p.Max - p.Min)
	for i := range durations {
		durations[i] = p.Min + time.Duration(rng.Int63n(span+1))
	}
	return durations
}

func (p IntervalProfile) normalized() IntervalProfile {
	if p.Min < 0 {
		p.Min = 0
	}
	if p.Max < 0 {
		p.Max = 0
	}
	if p.Max < p.Min {
		p.Max = p.Min
	}
	return p
}
