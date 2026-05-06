package makc

import "context"

// Keyboard exposes keyboard state and injection operations.
type Keyboard struct {
	client *Client
}

// InjectionBackend returns the backend currently used for keyboard injection.
func (k *Keyboard) InjectionBackend() KeyboardInjectionBackend {
	if k == nil || k.client == nil || k.client.backend == nil {
		return KeyboardInjectionAuto
	}
	return k.client.backend.KeyboardInjection()
}

// State returns the current state of a key.
func (k *Keyboard) State(ctx context.Context, key Key) (State, error) {
	if err := k.ensureReady(ctx); err != nil {
		return Up, err
	}
	return k.client.backend.KeyState(ctx, key)
}

// Down reports whether a key is currently pressed.
func (k *Keyboard) Down(ctx context.Context, key Key) (bool, error) {
	state, err := k.State(ctx, key)
	return state.Bool(), err
}

// Press injects a key-down event.
func (k *Keyboard) Press(ctx context.Context, key Key) error {
	return k.Inject(ctx, KeyDownEvent(key))
}

// Release injects a key-up event.
func (k *Keyboard) Release(ctx context.Context, key Key) error {
	return k.Inject(ctx, KeyUpEvent(key))
}

// Tap injects a key-down followed by a key-up event.
func (k *Keyboard) Tap(ctx context.Context, key Key) error {
	return k.Inject(ctx, KeyTapEvents(key)...)
}

// Combo presses keys in order and releases them in reverse order.
func (k *Keyboard) Combo(ctx context.Context, keys ...Key) error {
	return k.Inject(ctx, ComboEvents(keys...)...)
}

// TypeText injects Unicode text. It does not depend on the active keyboard
// layout for the characters it can represent as UTF-16.
func (k *Keyboard) TypeText(ctx context.Context, text string) error {
	return k.Inject(ctx, TextEvent(text))
}

// ScanPress injects a scan-code key-down event.
func (k *Keyboard) ScanPress(ctx context.Context, scanCode uint16, extended bool) error {
	return k.Inject(ctx, ScanCodeEvent(scanCode, Down, extended))
}

// ScanRelease injects a scan-code key-up event.
func (k *Keyboard) ScanRelease(ctx context.Context, scanCode uint16, extended bool) error {
	return k.Inject(ctx, ScanCodeEvent(scanCode, Up, extended))
}

// ScanTap injects scan-code key-down and key-up events.
func (k *Keyboard) ScanTap(ctx context.Context, scanCode uint16, extended bool) error {
	return k.Inject(ctx,
		ScanCodeEvent(scanCode, Down, extended),
		ScanCodeEvent(scanCode, Up, extended),
	)
}

// Inject sends keyboard events. Consecutive non-pause events are sent as one
// backend batch; pause events flush the current batch and then sleep.
func (k *Keyboard) Inject(ctx context.Context, events ...KeyboardEvent) error {
	if err := k.ensureReady(ctx); err != nil {
		return err
	}

	batch := make([]KeyboardEvent, 0, len(events))
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		err := k.client.backend.InjectKeyboard(ctx, batch)
		batch = batch[:0]
		return err
	}

	for _, event := range events {
		if err := validateKeyboardEvent(event); err != nil {
			return err
		}
		if event.Kind != KeyboardEventPause {
			batch = append(batch, event)
			continue
		}
		if err := flush(); err != nil {
			return err
		}
		if err := sleepContext(ctx, event.Duration); err != nil {
			return err
		}
	}

	return flush()
}

func (k *Keyboard) ensureReady(ctx context.Context) error {
	if k == nil {
		return ErrClosed
	}
	return k.client.ensureReady(ctx)
}

func validateKeyboardEvent(event KeyboardEvent) error {
	switch event.Kind {
	case KeyboardEventKey:
		if event.Key == KeyUnknown {
			return unsupported("unknown key")
		}
		if !event.State.valid() {
			return unsupported("unknown key state")
		}
		return nil
	case KeyboardEventScanCode:
		if event.ScanCode == 0 {
			return unsupported("unknown scan code")
		}
		if !event.State.valid() {
			return unsupported("unknown key state")
		}
		return nil
	case KeyboardEventText, KeyboardEventPause:
		return nil
	default:
		return unsupported("unknown keyboard event")
	}
}
