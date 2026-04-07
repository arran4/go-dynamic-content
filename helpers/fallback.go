package helpers

import "errors"

// FallbackGenerator takes a list of generator functions and returns a new generator function.
// It will try each generator in order and return the result of the first one that succeeds.
// If all generators fail, it returns an error.
func FallbackGenerator[T any](generators ...func() (*T, error)) func() (*T, error) {
	return func() (*T, error) {
		var lastErr error
		for _, gen := range generators {
			val, err := gen()
			if err == nil {
				return val, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, errors.New("no generators provided")
	}
}
