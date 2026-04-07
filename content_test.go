package utils

import (
	"sync"
	"testing"
)

func testContentImpl(t *testing.T, fc Content[[]byte], generateCallsPtr *int) {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := fc.Data()
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if string(*b) != "hello world" {
				t.Errorf("expected 'hello world', got '%s'", string(*b))
			}
		}()
	}
	wg.Wait()

	if *generateCallsPtr < 1 {
		t.Errorf("expected at least 1 call to generate, got %d", *generateCallsPtr)
	}

	// Test SetGenerator and Close
	fc.SetGenerator(func() (*[]byte, error) {
		b := []byte("new world")
		return &b, nil
	})

	b, err := fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if string(*b) != "new world" {
		t.Errorf("expected 'new world', got '%s'", string(*b))
	}

	err = fc.Close()
	if err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}

	if fc.String() != "new world" {
		t.Errorf("expected 'new world' from String(), got '%s'", fc.String())
	}
}

func TestContent_LazyWeak(t *testing.T) {
	generateCalls := 0
	fc := NewContent[[]byte](WithGenerator[[]byte](func() (*[]byte, error) {
		generateCalls++
		b := []byte("hello world")
		return &b, nil
	}), UseWeakStorage[[]byte](true), UseLazyLoading[[]byte](true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_LazyMemory(t *testing.T) {
	generateCalls := 0
	fc := NewContent[[]byte](WithGenerator[[]byte](func() (*[]byte, error) {
		generateCalls++
		b := []byte("hello world")
		return &b, nil
	}), UseMemoryStorage[[]byte](true), UseLazyLoading[[]byte](true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_EagerWeak(t *testing.T) {
	generateCalls := 0
	fc := NewContent[[]byte](WithGenerator[[]byte](func() (*[]byte, error) {
		generateCalls++
		b := []byte("hello world")
		return &b, nil
	}), UseWeakStorage[[]byte](true), UseEagerLoading[[]byte](true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_EagerMemory(t *testing.T) {
	generateCalls := 0
	fc := NewContent[[]byte](WithGenerator[[]byte](func() (*[]byte, error) {
		generateCalls++
		b := []byte("hello world")
		return &b, nil
	}), UseMemoryStorage[[]byte](true), UseEagerLoading[[]byte](true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_WithOptions(t *testing.T) {
	fc := NewContent[[]byte](WithValue[[]byte]([]byte("hello bytes")))
	if fc.String() != "hello bytes" {
		t.Errorf("expected 'hello bytes', got '%s'", fc.String())
	}

	fc2 := NewContent[string](WithValue[string]("hello string"))
	if fc2.String() != "hello string" {
		t.Errorf("expected 'hello string', got '%s'", fc2.String())
	}
}

func TestContent_Validator(t *testing.T) {
	generateCalls := 0
	valid := true

	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			generateCalls++
			b := []byte("valid world")
			return &b, nil
		}),
		WithValidator[[]byte](func() bool {
			return valid
		}),
	)

	if fc.IsValid() {
		t.Errorf("expected IsValid() to be false initially as cache is empty")
	}

	b, err := fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if string(*b) != "valid world" {
		t.Errorf("expected 'valid world', got '%s'", string(*b))
	}
	if generateCalls != 1 {
		t.Errorf("expected 1 generate call, got %d", generateCalls)
	}
	if !fc.IsValid() {
		t.Errorf("expected IsValid() to be true after cache is populated")
	}

	// Should not generate again
	_, _ = fc.Data()
	if generateCalls != 1 {
		t.Errorf("expected still 1 generate call, got %d", generateCalls)
	}

	// Invalidate cache
	valid = false
	if fc.IsValid() {
		t.Errorf("expected IsValid() to be false after validator changes")
	}

	// Fetching data should now generate again
	b, err = fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if string(*b) != "valid world" {
		t.Errorf("expected 'valid world', got '%s'", string(*b))
	}
	if generateCalls != 2 {
		t.Errorf("expected 2 generate calls, got %d", generateCalls)
	}

	// Test SetValidator
	fc.SetValidator(func() bool { return false })
	if fc.IsValid() {
		t.Errorf("expected IsValid() to be false after SetValidator(false)")
	}
}
