package utils

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

func testContentImpl(t *testing.T, fc Content, generateCallsPtr *int) {
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
	fc.SetGenerator(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBufferString("new world")), nil
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
	fc := NewContent(WithGenerator(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}), UseWeakStorage(true), UseLazyLoading(true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_LazyMemory(t *testing.T) {
	generateCalls := 0
	fc := NewContent(WithGenerator(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}), UseMemoryStorage(true), UseLazyLoading(true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_EagerWeak(t *testing.T) {
	generateCalls := 0
	fc := NewContent(WithGenerator(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}), UseWeakStorage(true), UseEagerLoading(true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_EagerMemory(t *testing.T) {
	generateCalls := 0
	fc := NewContent(WithGenerator(func() (io.ReadCloser, error) {
		generateCalls++
		return io.NopCloser(bytes.NewBufferString("hello world")), nil
	}), UseMemoryStorage(true), UseEagerLoading(true))
	testContentImpl(t, fc, &generateCalls)
}

func TestContent_WithOptions(t *testing.T) {
	fc := NewContent(WithBytes([]byte("hello bytes")))
	if fc.String() != "hello bytes" {
		t.Errorf("expected 'hello bytes', got '%s'", fc.String())
	}

	fc2 := NewContent(WithString("hello string"))
	if fc2.String() != "hello string" {
		t.Errorf("expected 'hello string', got '%s'", fc2.String())
	}
}

func TestContent_Validator(t *testing.T) {
	generateCalls := 0
	valid := true

	fc := NewContent(
		WithGenerator(func() (io.ReadCloser, error) {
			generateCalls++
			return io.NopCloser(bytes.NewBufferString("valid world")), nil
		}),
		WithValidator(func() bool {
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
