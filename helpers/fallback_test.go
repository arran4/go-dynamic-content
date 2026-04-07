package helpers

import (
	"errors"
	"testing"
)

func TestFallbackGenerator(t *testing.T) {
	val1 := "value 1"
	val2 := "value 2"

	gen1Error := func() (*string, error) {
		return nil, errors.New("gen1 failed")
	}

	gen2Success := func() (*string, error) {
		return &val1, nil
	}

	gen3Success := func() (*string, error) {
		return &val2, nil
	}

	t.Run("first succeeds", func(t *testing.T) {
		fallbackGen := FallbackGenerator(gen2Success, gen3Success)
		val, err := fallbackGen()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *val != val1 {
			t.Errorf("expected %s, got %s", val1, *val)
		}
	})

	t.Run("second succeeds", func(t *testing.T) {
		fallbackGen := FallbackGenerator(gen1Error, gen3Success)
		val, err := fallbackGen()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *val != val2 {
			t.Errorf("expected %s, got %s", val2, *val)
		}
	})

	t.Run("all fail", func(t *testing.T) {
		fallbackGen := FallbackGenerator(gen1Error, gen1Error)
		_, err := fallbackGen()
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	t.Run("no generators", func(t *testing.T) {
		var noGens []func() (*string, error)
		fallbackGen := FallbackGenerator(noGens...)
		_, err := fallbackGen()
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}
