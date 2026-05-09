package makc

import "testing"

func TestZeroValueClientClose(t *testing.T) {
	var client Client
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}
}
