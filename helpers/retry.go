package helpers

import (
	"errors"
	"time"
)

// RetryGenerator wraps a generator and retries it up to `maxRetries` times if it fails,
// waiting `delay` between attempts.
// If maxRetries is negative, it defaults to 0 (meaning one initial attempt and no retries).
// If delay is negative, it defaults to 0 (no delay).
// If gen is nil, a descriptive error is returned immediately upon invocation.
func RetryGenerator[T any](maxRetries int, delay time.Duration, gen func() (*T, error)) func() (*T, error) {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if delay < 0 {
		delay = 0
	}

	return func() (*T, error) {
		if gen == nil {
			return nil, errors.New("RetryGenerator: nil generator provided")
		}

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
