package helpers

import (
	"sync"
	"time"
)

// TimeExpiry returns a validator function that returns true until the given duration has elapsed
// since the validator was created or last reset. It also returns a reset function.
// The returned validator and reset functions are safe for concurrent use.
func TimeExpiry(duration time.Duration) (func() bool, func()) {
	var mu sync.RWMutex
	var expiry time.Time

	reset := func() {
		newExpiry := time.Now().Add(duration)
		mu.Lock()
		expiry = newExpiry
		mu.Unlock()
	}

	reset()

	validator := func() bool {
		mu.RLock()
		currentExpiry := expiry
		mu.RUnlock()
		return time.Now().Before(currentExpiry)
	}

	return validator, reset
}
