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

	t.Run("nil only generator", func(t *testing.T) {
		fallbackGen := FallbackGenerator[string](nil)
		_, err := fallbackGen()
		if err == nil {
			t.Fatal("expected error, got none")
		}
		if err.Error() != "FallbackGenerator: nil generator provided at index 0" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("nil first generator", func(t *testing.T) {
		invoked := false
		gen := func() (*string, error) {
			invoked = true
			return &val1, nil
		}
		fallbackGen := FallbackGenerator(nil, gen)
		_, err := fallbackGen()
		if err == nil {
			t.Fatal("expected error, got none")
		}
		if err.Error() != "FallbackGenerator: nil generator provided at index 0" {
			t.Errorf("unexpected error message: %v", err)
		}
		if invoked {
			t.Error("expected later generator to not be invoked")
		}
	})

	t.Run("nil in middle", func(t *testing.T) {
		invokedFirst := false
		invokedLast := false
		genFirst := func() (*string, error) {
			invokedFirst = true
			return nil, errors.New("failed")
		}
		genLast := func() (*string, error) {
			invokedLast = true
			return &val1, nil
		}
		fallbackGen := FallbackGenerator(genFirst, nil, genLast)
		_, err := fallbackGen()
		if err == nil {
			t.Fatal("expected error, got none")
		}
		if err.Error() != "FallbackGenerator: nil generator provided at index 1" {
			t.Errorf("unexpected error message: %v", err)
		}
		if invokedFirst || invokedLast {
			t.Error("expected no generators to be invoked")
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
