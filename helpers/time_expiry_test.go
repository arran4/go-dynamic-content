package helpers

import (
	"sync"
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

func TestTimeExpiry_Concurrent(t *testing.T) {
	duration := 10 * time.Millisecond
	isValid, reset := TimeExpiry(duration)

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	numWorkers := 10
	iterations := 1000

	// Start goroutines validating constantly
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			for j := 0; j < iterations; j++ {
				isValid()
			}
		}()
	}

	// Start goroutines resetting constantly
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			for j := 0; j < iterations; j++ {
				reset()
			}
		}()
	}

	close(startCh)
	wg.Wait()
}
