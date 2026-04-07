package helpers

import "time"

// TimeExpiry returns a validator function that returns true until the given duration has elapsed
// since the validator was created or last reset. It also returns a reset function.
func TimeExpiry(duration time.Duration) (func() bool, func()) {
	var expiry time.Time

	reset := func() {
		expiry = time.Now().Add(duration)
	}

	reset()

	validator := func() bool {
		return time.Now().Before(expiry)
	}

	return validator, reset
}
