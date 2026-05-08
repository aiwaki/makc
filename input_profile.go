package makc

import "time"

// InputProfile groups movement, click, and keyboard timing into one reusable
// profile for higher-level workflows.
type InputProfile struct {
	Movement    MovementProfile
	Click       ClickProfile
	DoubleClick ClickProfile
	Typing      TypingProfile
	KeyHold     time.Duration
}

// InstantInputProfile returns a profile with no explicit timing pauses.
func InstantInputProfile() InputProfile {
	return InputProfile{
		Movement:    InstantMovement,
		Click:       InstantClick,
		DoubleClick: MultiClick(2, 0, FixedInterval(0)),
		Typing:      InstantTyping,
	}
}

// FastInputProfile returns a short, responsive profile for quick UI work.
func FastInputProfile(seed int64) InputProfile {
	return InputProfile{
		Movement:    NaturalMovement(8, 90*time.Millisecond, seed),
		Click:       ClickWithHold(18 * time.Millisecond),
		DoubleClick: DoubleClick(18*time.Millisecond, 85*time.Millisecond),
		Typing:      VariableTyping(25*time.Millisecond, 75*time.Millisecond, seed+1),
		KeyHold:     18 * time.Millisecond,
	}
}

// BalancedInputProfile returns the default profile for ordinary UI automation.
func BalancedInputProfile(seed int64) InputProfile {
	return InputProfile{
		Movement:    NaturalMovement(14, 180*time.Millisecond, seed),
		Click:       ClickWithHold(35 * time.Millisecond),
		DoubleClick: DoubleClick(35*time.Millisecond, 125*time.Millisecond),
		Typing:      VariableTyping(45*time.Millisecond, 120*time.Millisecond, seed+1),
		KeyHold:     35 * time.Millisecond,
	}
}

// CarefulInputProfile returns a slower profile for fragile or latency-prone UI.
func CarefulInputProfile(seed int64) InputProfile {
	return InputProfile{
		Movement:    NaturalMovement(22, 320*time.Millisecond, seed),
		Click:       ClickWithHold(55 * time.Millisecond),
		DoubleClick: DoubleClick(55*time.Millisecond, 180*time.Millisecond),
		Typing:      VariableTyping(80*time.Millisecond, 180*time.Millisecond, seed+1),
		KeyHold:     50 * time.Millisecond,
	}
}

// MovementEvents returns mouse movement events using the profile movement.
func (p InputProfile) MovementEvents(from, to Point) []MouseEvent {
	return p.Movement.Events(from, to)
}

// ClickEvents returns mouse click events using the profile click timing.
func (p InputProfile) ClickEvents(button MouseButton) []MouseEvent {
	return p.Click.Events(button)
}

// DoubleClickEvents returns mouse double-click events using the profile timing.
func (p InputProfile) DoubleClickEvents(button MouseButton) []MouseEvent {
	return p.DoubleClick.Events(button)
}

// KeyTapEvents returns key tap events using the profile key hold.
func (p InputProfile) KeyTapEvents(key Key) []KeyboardEvent {
	return KeyTapEventsWithHold(key, p.KeyHold)
}

// TextEvents returns keyboard text events using the profile typing timing.
func (p InputProfile) TextEvents(text string) []KeyboardEvent {
	return p.Typing.Events(text)
}
