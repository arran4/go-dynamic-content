package helpers

import (
	"testing"
)

func TestDynamicGenerator(t *testing.T) {
	dg := NewDynamicGenerator[string](func() (*string, error) {
		val := "state 1"
		return &val, nil
	})

	val, err := dg.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil || *val != "state 1" {
		t.Fatalf("expected 'state 1', got %v", val)
	}

	dg.SetGenerator(func() (*string, error) {
		val := "state 2"
		return &val, nil
	})

	val, err = dg.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil || *val != "state 2" {
		t.Fatalf("expected 'state 2', got %v", val)
	}
}
