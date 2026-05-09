package makc

import (
	"context"
	"errors"
	"testing"
)

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

func TestValidateListenOptionsRejectsUnknownMask(t *testing.T) {
	err := validateListenOptions(ListenOptions{Mask: ListenMask(1 << 7)})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("validateListenOptions() error = %v, want ErrUnsupported", err)
	}
}

func TestClientListenUsesBackgroundForNilContext(t *testing.T) {
	backend := &listenTestBackend{}
	client := &Client{backend: backend}

	//lint:ignore SA1012 Listen intentionally accepts nil contexts for API consistency.
	_, err := client.Listen(nil, ListenOptions{})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Listen() error = %v, want ErrUnsupported", err)
	}
	if backend.ctx == nil {
		t.Fatal("ListenInput context is nil, want background context")
	}
}

func TestClientListenRejectsUnknownMaskBeforeBackend(t *testing.T) {
	backend := &listenTestBackend{}
	client := &Client{backend: backend}

	_, err := client.Listen(context.Background(), ListenOptions{Mask: ListenMask(1 << 7)})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Listen() error = %v, want ErrUnsupported", err)
	}
	if backend.called {
		t.Fatal("ListenInput was called for an invalid mask")
	}
}

func TestListenBackendString(t *testing.T) {
	tests := map[ListenBackend]string{
		ListenBackendAuto:         "auto",
		ListenBackendLowLevelHook: "hook",
		ListenBackendRawInput:     "rawinput",
		ListenBackendEvdev:        "evdev",
		ListenBackend(0xFF):       "unknown",
	}
	for backend, want := range tests {
		if got := backend.String(); got != want {
			t.Fatalf("ListenBackend(%d).String() = %q, want %q", backend, got, want)
		}
	}
}

type listenTestBackend struct {
	sequenceTestBackend
	called bool
	ctx    context.Context
}

func (b *listenTestBackend) ListenInput(ctx context.Context, _ ListenOptions) (*Listener, error) {
	b.called = true
	b.ctx = ctx
	return nil, ErrUnsupported
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
