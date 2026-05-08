package makc

import (
	"math"
	"math/rand"
	"time"
)

// WheelDelta is one standard wheel detent in Windows mouse data units.
const WheelDelta = 120

// MouseEventKind describes the operation represented by a MouseEvent.
type MouseEventKind uint8

const (
	MouseEventMove MouseEventKind = iota + 1
	MouseEventButton
	MouseEventWheel
	MouseEventHWheel
	MouseEventPause
)

// MouseEvent is one mouse operation in a batch. Pause events are interpreted by
// the Go layer and are not sent to the backend.
type MouseEvent struct {
	// Kind selects which payload the event carries.
	Kind MouseEventKind

	// Move is the movement payload for MouseEventMove.
	Move MouseMove

	// Button is the button payload for MouseEventButton.
	Button MouseButton

	// State is the button state for MouseEventButton.
	State State

	// Delta is the raw wheel delta for MouseEventWheel and MouseEventHWheel.
	Delta int

	// Duration is the delay for MouseEventPause.
	Duration time.Duration
}

// ClickProfile describes one or more button clicks with optional hold and
// between-click timing.
type ClickProfile struct {
	// Count is the number of clicks to generate. Values less than one become
	// one click.
	Count int

	// Hold is the pause between button down and button up.
	Hold time.Duration

	// Interval controls pauses between repeated clicks.
	Interval IntervalProfile
}

// InstantClick is the default click profile: one down event followed by one up
// event without an explicit pause.
var InstantClick = ClickProfile{Count: 1}

// ClickWithHold creates a single-click profile that holds the button before
// release.
func ClickWithHold(hold time.Duration) ClickProfile {
	return ClickProfile{
		Count: 1,
		Hold:  hold,
	}
}

// MultiClick creates a profile for repeated clicks.
func MultiClick(count int, hold time.Duration, interval IntervalProfile) ClickProfile {
	return ClickProfile{
		Count:    count,
		Hold:     hold,
		Interval: interval,
	}
}

// DoubleClick creates a two-click profile with a fixed interval between clicks.
func DoubleClick(hold, interval time.Duration) ClickProfile {
	return MultiClick(2, hold, FixedInterval(interval))
}

// Events returns mouse events for clicking the given button.
func (p ClickProfile) Events(button MouseButton) []MouseEvent {
	p = p.normalized()
	intervals := p.Interval.Durations(p.Count - 1)
	events := make([]MouseEvent, 0, p.Count*4)
	for i := 0; i < p.Count; i++ {
		events = append(events, MouseButtonEvent(button, Down))
		if p.Hold > 0 {
			events = append(events, MousePauseEvent(p.Hold))
		}
		events = append(events, MouseButtonEvent(button, Up))
		if i < len(intervals) && intervals[i] > 0 {
			events = append(events, MousePauseEvent(intervals[i]))
		}
	}
	return events
}

func (p ClickProfile) normalized() ClickProfile {
	if p.Count < 1 {
		p.Count = 1
	}
	if p.Hold < 0 {
		p.Hold = 0
	}
	return p
}

// MouseMoveEvent creates a movement event.
func MouseMoveEvent(move MouseMove) MouseEvent {
	return MouseEvent{
		Kind: MouseEventMove,
		Move: move,
	}
}

// MouseButtonEvent creates a button state event.
func MouseButtonEvent(button MouseButton, state State) MouseEvent {
	return MouseEvent{
		Kind:   MouseEventButton,
		Button: button,
		State:  state,
	}
}

// MouseWheelEvent creates a vertical wheel event. The delta is in raw Windows
// units; use WheelDelta for one wheel detent.
func MouseWheelEvent(delta int) MouseEvent {
	return MouseEvent{
		Kind:  MouseEventWheel,
		Delta: delta,
	}
}

// MouseHWheelEvent creates a horizontal wheel event. The delta is in raw
// Windows units; use WheelDelta for one wheel detent.
func MouseHWheelEvent(delta int) MouseEvent {
	return MouseEvent{
		Kind:  MouseEventHWheel,
		Delta: delta,
	}
}

// MousePauseEvent creates a delay between event batches.
func MousePauseEvent(duration time.Duration) MouseEvent {
	return MouseEvent{
		Kind:     MouseEventPause,
		Duration: duration,
	}
}

// MovementCurve selects the deterministic curve used by MovementProfile.
type MovementCurve uint8

const (
	MovementLinear MovementCurve = iota
	MovementEaseInOut
	MovementNatural
)

// MovementProfile describes a deterministic absolute cursor path.
type MovementProfile struct {
	// Steps is the number of move events to generate. Values less than one
	// become one step.
	Steps int

	// Duration is the total movement duration distributed between steps.
	Duration time.Duration

	// Curve selects how points are distributed along the path.
	Curve MovementCurve

	// Jitter is the maximum natural-path offset in pixels. Zero lets makc pick a
	// distance-based value for MovementNatural.
	Jitter int

	// Seed makes natural paths and pauses reproducible.
	Seed int64
}

// InstantMovement jumps to the target in one event.
var InstantMovement = MovementProfile{Steps: 1}

// LinearMovement creates a straight deterministic movement profile.
func LinearMovement(steps int, duration time.Duration) MovementProfile {
	return MovementProfile{
		Steps:    steps,
		Duration: duration,
		Curve:    MovementLinear,
	}
}

// EaseInOutMovement creates a deterministic profile that accelerates and then
// decelerates along a straight path.
func EaseInOutMovement(steps int, duration time.Duration) MovementProfile {
	return MovementProfile{
		Steps:    steps,
		Duration: duration,
		Curve:    MovementEaseInOut,
	}
}

// NaturalMovement creates a seeded Bezier movement profile with varied pauses.
// Pass a different seed when you want a different reproducible path.
func NaturalMovement(steps int, duration time.Duration, seed int64) MovementProfile {
	return NaturalMovementWithJitter(steps, duration, 0, seed)
}

// NaturalMovementWithJitter creates a seeded movement profile with an explicit
// maximum path jitter in pixels. A zero jitter lets makc choose a distance-based
// value.
func NaturalMovementWithJitter(steps int, duration time.Duration, jitter int, seed int64) MovementProfile {
	return MovementProfile{
		Steps:    steps,
		Duration: duration,
		Curve:    MovementNatural,
		Jitter:   jitter,
		Seed:     seed,
	}
}

// Events returns the movement events from start to end.
func (p MovementProfile) Events(start, end Point) []MouseEvent {
	p = p.normalized()
	if p.Curve == MovementNatural {
		return p.naturalEvents(start, end)
	}

	events := make([]MouseEvent, 0, p.Steps*2)
	delay := time.Duration(0)
	if p.Duration > 0 && p.Steps > 1 {
		delay = p.Duration / time.Duration(p.Steps-1)
	}

	for i := 1; i <= p.Steps; i++ {
		t := float64(i) / float64(p.Steps)
		t = p.applyCurve(t)

		x := start.X + int(math.Round(float64(end.X-start.X)*t))
		y := start.Y + int(math.Round(float64(end.Y-start.Y)*t))
		events = append(events, MouseMoveEvent(Abs(x, y)))

		if delay > 0 && i < p.Steps {
			events = append(events, MousePauseEvent(delay))
		}
	}

	return events
}

func (p MovementProfile) normalized() MovementProfile {
	if p.Steps < 1 {
		p.Steps = 1
	}
	if p.Duration < 0 {
		p.Duration = 0
	}
	if p.Jitter < 0 {
		p.Jitter = 0
	}
	return p
}

func (p MovementProfile) applyCurve(t float64) float64 {
	switch p.Curve {
	case MovementEaseInOut, MovementNatural:
		return t * t * (3 - 2*t)
	default:
		return t
	}
}

func (p MovementProfile) naturalEvents(start, end Point) []MouseEvent {
	rng := rand.New(rand.NewSource(p.Seed))
	pauses := p.naturalPauses(rng)
	events := make([]MouseEvent, 0, p.Steps*2)

	from := floatPoint{X: float64(start.X), Y: float64(start.Y)}
	to := floatPoint{X: float64(end.X), Y: float64(end.Y)}
	dx := to.X - from.X
	dy := to.Y - from.Y
	distance := math.Hypot(dx, dy)

	control1, control2 := naturalControlPoints(rng, from, to, distance, p.naturalJitter(distance))

	for i := 1; i <= p.Steps; i++ {
		t := p.applyCurve(float64(i) / float64(p.Steps))
		point := cubicBezier(from, control1, control2, to, t)
		if i == p.Steps {
			point = to
		} else {
			point = addNaturalJitter(rng, point, p.naturalJitter(distance)*0.18, t)
		}

		events = append(events, MouseMoveEvent(Abs(
			int(math.Round(point.X)),
			int(math.Round(point.Y)),
		)))
		if i < p.Steps && len(pauses) > 0 && pauses[i-1] > 0 {
			events = append(events, MousePauseEvent(pauses[i-1]))
		}
	}

	return events
}

func (p MovementProfile) naturalPauses(rng *rand.Rand) []time.Duration {
	count := p.Steps - 1
	if count <= 0 || p.Duration <= 0 {
		return nil
	}

	weights := make([]float64, count)
	var total float64
	for i := range weights {
		weights[i] = 0.65 + rng.Float64()*0.7
		total += weights[i]
	}

	pauses := make([]time.Duration, count)
	remaining := p.Duration
	for i, weight := range weights {
		if i == len(weights)-1 {
			pauses[i] = remaining
			break
		}
		pause := time.Duration(float64(p.Duration) * weight / total)
		if pause > remaining {
			pause = remaining
		}
		pauses[i] = pause
		remaining -= pause
	}
	return pauses
}

func (p MovementProfile) naturalJitter(distance float64) float64 {
	if distance < 2 {
		return 0
	}
	if p.Jitter > 0 {
		return float64(p.Jitter)
	}
	return math.Min(32, math.Max(1, distance*0.08))
}

func naturalControlPoints(rng *rand.Rand, from, to floatPoint, distance, jitter float64) (floatPoint, floatPoint) {
	dx := to.X - from.X
	dy := to.Y - from.Y
	if distance == 0 {
		return from, to
	}

	perpX := -dy / distance
	perpY := dx / distance
	offset1 := signedFloat(rng, jitter)
	offset2 := signedFloat(rng, jitter)
	tangent1 := signedFloat(rng, jitter*0.25)
	tangent2 := signedFloat(rng, jitter*0.25)

	return floatPoint{
			X: from.X + dx*0.33 + perpX*offset1 + (dx/distance)*tangent1,
			Y: from.Y + dy*0.33 + perpY*offset1 + (dy/distance)*tangent1,
		}, floatPoint{
			X: from.X + dx*0.66 + perpX*offset2 + (dx/distance)*tangent2,
			Y: from.Y + dy*0.66 + perpY*offset2 + (dy/distance)*tangent2,
		}
}

func addNaturalJitter(rng *rand.Rand, point floatPoint, amount float64, t float64) floatPoint {
	if amount <= 0 {
		return point
	}
	falloff := math.Sin(math.Pi * t)
	return floatPoint{
		X: point.X + signedFloat(rng, amount)*falloff,
		Y: point.Y + signedFloat(rng, amount)*falloff,
	}
}

func cubicBezier(p0, p1, p2, p3 floatPoint, t float64) floatPoint {
	u := 1 - t
	uu := u * u
	tt := t * t
	return floatPoint{
		X: uu*u*p0.X + 3*uu*t*p1.X + 3*u*tt*p2.X + tt*t*p3.X,
		Y: uu*u*p0.Y + 3*uu*t*p1.Y + 3*u*tt*p2.Y + tt*t*p3.Y,
	}
}

func signedFloat(rng *rand.Rand, magnitude float64) float64 {
	if magnitude <= 0 {
		return 0
	}
	return (rng.Float64()*2 - 1) * magnitude
}

type floatPoint struct {
	X float64
	Y float64
}
