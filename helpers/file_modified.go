package helpers

import (
	"os"
	"sync"
	"time"
)

// FileModified returns a validator function that returns true as long as the specified file
// has not been modified since the validator was created or last reset. It also returns a reset function.
// The returned validator and reset functions are safe for concurrent use.
// If the file is modified exactly during a reset operation, the baseline modification time
// recorded by the reset may capture either the pre-modification or post-modification state,
// depending on when the OS updates the file's stat metadata.
func FileModified(filepath string) (func() bool, func()) {
	var mu sync.RWMutex
	var lastModified time.Time

	reset := func() {
		var newLastModified time.Time
		info, err := os.Stat(filepath)
		if err == nil {
			newLastModified = info.ModTime()
		}

		mu.Lock()
		lastModified = newLastModified
		mu.Unlock()
	}

	reset()

	validator := func() bool {
		info, err := os.Stat(filepath)
		if err != nil {
			// If file no longer exists or can't be accessed, we could consider it invalid
			return false
		}
		modTime := info.ModTime()

		mu.RLock()
		currentLastModified := lastModified
		mu.RUnlock()

		// It's valid if it's equal or older. If it's newer, it's invalid.
		return !modTime.After(currentLastModified)
	}

	return validator, reset
}
