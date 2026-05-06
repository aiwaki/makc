//go:build !windows

package makc

import (
	"errors"
	"testing"
)

func TestOpenUnsupported(t *testing.T) {
	client, err := Open()
	if client != nil {
		t.Fatalf("Open() client = %#v, want nil", client)
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Open() error = %v, want ErrUnsupported", err)
	}
}
