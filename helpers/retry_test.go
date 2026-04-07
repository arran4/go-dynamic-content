package helpers

import (
	"errors"
	"testing"
	"time"
)

func TestRetryGenerator(t *testing.T) {
	val := "success"

	t.Run("succeeds on first try", func(t *testing.T) {
		attempts := 0
		gen := func() (*string, error) {
			attempts++
			return &val, nil
		}

		retryGen := RetryGenerator(3, 0, gen)
		res, err := retryGen()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil || *res != val {
			t.Errorf("expected %s, got %v", val, res)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("succeeds after retries", func(t *testing.T) {
		attempts := 0
		gen := func() (*string, error) {
			attempts++
			if attempts < 3 {
				return nil, errors.New("failed")
			}
			return &val, nil
		}

		retryGen := RetryGenerator(3, 0, gen)
		res, err := retryGen()

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == nil || *res != val {
			t.Errorf("expected %s, got %v", val, res)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("fails completely", func(t *testing.T) {
		attempts := 0
		gen := func() (*string, error) {
			attempts++
			return nil, errors.New("failed")
		}

		retryGen := RetryGenerator(3, 0, gen)
		_, err := retryGen()

		if err == nil {
			t.Fatal("expected error, got none")
		}
		// 1 initial try + 3 retries = 4 attempts
		if attempts != 4 {
			t.Errorf("expected 4 attempts, got %d", attempts)
		}
	})

	t.Run("respects delay", func(t *testing.T) {
		attempts := 0
		gen := func() (*string, error) {
			attempts++
			if attempts < 2 {
				return nil, errors.New("failed")
			}
			return &val, nil
		}

		delay := 50 * time.Millisecond
		retryGen := RetryGenerator(1, delay, gen)

		start := time.Now()
		_, _ = retryGen()
		elapsed := time.Since(start)

		if elapsed < delay {
			t.Errorf("expected at least %v delay, got %v", delay, elapsed)
		}
	})
}
