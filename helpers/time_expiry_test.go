package helpers

import (
	"testing"
	"time"
)

func TestTimeExpiry(t *testing.T) {
	// Use a short duration for testing
	duration := 50 * time.Millisecond

	isValid, reset := TimeExpiry(duration)

	// Should be valid immediately
	if !isValid() {
		t.Error("expected validator to return true immediately after creation")
	}

	// Wait for duration to pass
	time.Sleep(duration * 2)

	// Should be invalid now
	if isValid() {
		t.Error("expected validator to return false after duration has passed")
	}

	// Reset and check again
	reset()
	if !isValid() {
		t.Error("expected validator to return true immediately after reset")
	}
}
