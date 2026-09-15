package helpers

import (
	"errors"
	"fmt"
)

// FallbackGenerator takes a list of generator functions and returns a new generator function.
// It will try each generator in order and return the result of the first one that succeeds.
// If any generator in the list is nil, the returned generator will fail immediately with a
// descriptive error upon invocation, without executing any generators.
// If all generators fail, it returns an error.
func FallbackGenerator[T any](generators ...func() (*T, error)) func() (*T, error) {
	return func() (*T, error) {
		for i, gen := range generators {
			if gen == nil {
				return nil, fmt.Errorf("FallbackGenerator: nil generator provided at index %d", i)
			}
		}

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
