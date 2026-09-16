package helpers

import (
	"os"
	"sync"
)

// FileModified returns a validator function that returns true as long as the specified file
// has not been modified since the validator was created or last reset. It also returns a reset function.
// The returned validator and reset functions are safe for concurrent use.
//
// FileModified detects changes by capturing a baseline of inexpensive metadata:
// - exact modification time (mtime differences in either direction invalidate the file);
// - file size;
// - file identity (detectable through file identity via os.SameFile, such as file replacements).
// Missing or unstatable paths are considered invalid.
//
// Explicit Limitation: An in-place rewrite of the same file that preserves file identity,
// size, and exactly the same modification time does not have to be detected. This is a lightweight
// metadata validator, not a content hasher.
//
// If the file is modified exactly during a reset operation, the baseline metadata
// recorded by the reset may capture either the pre-modification or post-modification state,
// depending on when the OS updates the file's stat metadata.
func FileModified(filepath string) (func() bool, func()) {
	var mu sync.RWMutex
	var baselineInfo os.FileInfo
	var baselineErr error

	reset := func() {
		info, err := os.Stat(filepath)

		mu.Lock()
		baselineInfo = info
		baselineErr = err
		mu.Unlock()
	}

	reset()

	validator := func() bool {
		info, err := os.Stat(filepath)

		mu.RLock()
		baseInfo := baselineInfo
		baseErr := baselineErr
		mu.RUnlock()

		if err != nil || baseErr != nil {
			// If file no longer exists or can't be accessed, or was missing originally,
			// consider it invalid unless both are identically missing (which means no change,
			// but we generally consider missing files as invalid for cache use).
			// Following previous behavior: if missing or unstatable, it's invalid.
			return false
		}

		// Exact modification time equality
		if !info.ModTime().Equal(baseInfo.ModTime()) {
			return false
		}

		// File size
		if info.Size() != baseInfo.Size() {
			return false
		}

		// File identity
		if !os.SameFile(info, baseInfo) {
			return false
		}

		return true
	}

	return validator, reset
}
