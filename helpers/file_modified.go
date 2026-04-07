package helpers

import (
	"os"
	"time"
)

// FileModified returns a validator function that returns true as long as the specified file
// has not been modified since the validator was created or last reset. It also returns a reset function.
func FileModified(filepath string) (func() bool, func()) {
	var lastModified time.Time

	reset := func() {
		info, err := os.Stat(filepath)
		if err == nil {
			lastModified = info.ModTime()
		} else {
			// If we can't stat the file initially, we assume it hasn't been modified yet
			// or we just set a zero time.
			lastModified = time.Time{}
		}
	}

	reset()

	validator := func() bool {
		info, err := os.Stat(filepath)
		if err != nil {
			// If file no longer exists or can't be accessed, we could consider it invalid
			return false
		}
		// It's valid if it's equal or older. If it's newer, it's invalid.
		return !info.ModTime().After(lastModified)
	}

	return validator, reset
}
