package makc

import "testing"

func TestNormalizeListenOptions(t *testing.T) {
	opts := normalizeListenOptions(ListenOptions{})
	if opts.Mask != ListenAll {
		t.Fatalf("Mask = %d, want ListenAll", opts.Mask)
	}
	if opts.Buffer != 64 {
		t.Fatalf("Buffer = %d, want 64", opts.Buffer)
	}

	opts = normalizeListenOptions(ListenOptions{
		Mask:   ListenMouse,
		Buffer: 1,
	})
	if opts.Mask != ListenMouse {
		t.Fatalf("Mask = %d, want ListenMouse", opts.Mask)
	}
	if opts.Buffer != 1 {
		t.Fatalf("Buffer = %d, want 1", opts.Buffer)
	}
}

func TestPrepareInputEventFiltersInjectedByDefault(t *testing.T) {
	event := InputEvent{Injected: true}
	if prepareInputEvent(&event, ListenOptions{}) {
		t.Fatal("expected injected event to be filtered")
	}
}

func TestPrepareInputEventIncludesInjected(t *testing.T) {
	event := InputEvent{Injected: true}
	if !prepareInputEvent(&event, ListenOptions{IncludeInjected: true}) {
		t.Fatal("expected injected event to be included")
	}
	if !event.Injected {
		t.Fatal("expected injected marker to be preserved")
	}
}

func TestPrepareInputEventNormalizesOwnInjected(t *testing.T) {
	event := InputEvent{
		Injected:               true,
		LowerIntegrityInjected: true,
		Own:                    true,
	}
	if !prepareInputEvent(&event, ListenOptions{NormalizeOwnInjected: true}) {
		t.Fatal("expected own normalized event to be included")
	}
	if event.Injected || event.LowerIntegrityInjected {
		t.Fatalf("expected injected markers to be normalized: %+v", event)
	}
}

func TestMarkOwnInputEvent(t *testing.T) {
	event := InputEvent{ExtraInfo: 0xCAFE}
	markOwnInputEvent(&event, 0xCAFE)
	if !event.Own {
		t.Fatal("expected matching tag to mark event as own")
	}

	event = InputEvent{ExtraInfo: 0xCAFE}
	markOwnInputEvent(&event, 0)
	if event.Own {
		t.Fatal("expected zero tag not to mark event as own")
	}
}
