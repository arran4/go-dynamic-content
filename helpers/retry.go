package helpers

import (
	"time"
)

// RetryGenerator wraps a generator and retries it up to `maxRetries` times if it fails,
// waiting `delay` between attempts.
func RetryGenerator[T any](maxRetries int, delay time.Duration, gen func() (*T, error)) func() (*T, error) {
	return func() (*T, error) {
		var lastErr error
		for i := 0; i <= maxRetries; i++ {
			val, err := gen()
			if err == nil {
				return val, nil
			}
			lastErr = err
			if i < maxRetries && delay > 0 {
				time.Sleep(delay)
			}
		}
		return nil, lastErr
	}
}
