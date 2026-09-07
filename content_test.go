package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
			if b != nil && string(*b) != "hello world" {
				t.Errorf("expected 'hello world', got '%s'", string(*b))
			}
		}()
	}
	WaitWgWithTimeout(t, &wg)

	if *generateCallsPtr < 1 {
		t.Errorf("expected at least 1 call to generate, got %d", *generateCallsPtr)
	}

	err := fc.Close()
	if err != nil {
		t.Errorf("expected no error from Close, got %v", err)
	}

	// With the removal of SetGenerator, we can't change the generator inline
	// So fetching String() will just trigger the generator again.
	// Since the original generator returns "hello world", we check for it.
	if fc.String() != "hello world" {
		t.Errorf("expected 'hello world' from String() after close (due to regeneration), got '%s'", fc.String())
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

	if fc.Error() == nil {
		t.Errorf("expected Error() to return an error initially as cache is empty")
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
	if fc.Error() != nil {
		t.Errorf("expected Error() to be nil after cache is populated")
	}

	// Should not generate again
	_, _ = fc.Data()
	if generateCalls != 1 {
		t.Errorf("expected still 1 generate call, got %d", generateCalls)
	}

	// Invalidate cache
	valid = false
	if fc.Error() == nil {
		t.Errorf("expected Error() to return an error after validator changes")
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
}

func TestContent_HasContentAndInvalidate(t *testing.T) {
	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			b := []byte("content")
			return &b, nil
		}),
	)

	if fc.HasContent() {
		t.Errorf("expected HasContent() to be false initially")
	}

	_, _ = fc.Data()
	if !fc.HasContent() {
		t.Errorf("expected HasContent() to be true after data generation")
	}

	_ = fc.Invalidate()
	if fc.HasContent() {
		t.Errorf("expected HasContent() to be false after invalidation")
	}
}

func TestContent_Callbacks(t *testing.T) {
	generateCalls := 0
	invalidateCalls := 0
	closeCalls := 0

	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			b := []byte("content")
			return &b, nil
		}),
		WithOnGenerate[[]byte](func(val *[]byte, err error) {
			generateCalls++
		}),
		WithOnInvalidate[[]byte](func() {
			invalidateCalls++
		}),
		WithOnClose[[]byte](func() {
			closeCalls++
		}),
	)

	_, _ = fc.Data()
	if generateCalls != 1 {
		t.Errorf("expected 1 generate call, got %d", generateCalls)
	}

	_ = fc.Invalidate()
	if invalidateCalls != 1 {
		t.Errorf("expected 1 invalidate call, got %d", invalidateCalls)
	}

	// Should not trigger again if already empty
	_ = fc.Invalidate()
	if invalidateCalls != 1 {
		t.Errorf("expected invalidate call count to remain 1, got %d", invalidateCalls)
	}

	_ = fc.Close()
	if closeCalls != 1 {
		t.Errorf("expected 1 close call, got %d", closeCalls)
	}
}

func TestContent_ValidatorFalseLivelock(t *testing.T) {
	var generateCalls int32

	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			b := []byte("content")
			return &b, nil
		}),
		WithValidator[[]byte](func() bool {
			return false // always invalid to trigger continuous invalidation
		}),
	)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = fc.Data()
		}()
	}
	WaitWgWithTimeout(t, &wg)

	if atomic.LoadInt32(&generateCalls) > 100 { // We expect roughly 50, but definitely not thousands.
		t.Errorf("expected bound generation calls, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_InvalidationRacingGeneration(t *testing.T) {
	var generateCalls int32
	var invalidateCalls int32

	genStarted := make(chan struct{})
	genWait := make(chan struct{})
	var genStartedOnce sync.Once

	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			genStartedOnce.Do(func() { close(genStarted) })

			// We only want to wait on genWait the first time.
			// The second time (fresh generation), we just return.
			if atomic.LoadInt32(&generateCalls) == 1 {
				<-genWait
			}

			b := []byte("stale content")
			return &b, nil
		}),
		WithOnInvalidate[[]byte](func() {
			atomic.AddInt32(&invalidateCalls, 1)
		}),
	)

	// Force a generation
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = fc.Data()
	}()

	// Wait for generator to actually hold ownership
	<-genStarted

	// Issue invalidation while the generator holds ownership but hasn't returned
	_ = fc.Invalidate()

	// Unblock generator to commit
	close(genWait)

	// Wait for generator to finish
	WaitWgWithTimeout(t, &wg)

	// Given we invalidated during an active generation cycle, the epoch advanced.
	// The generator's commit MUST have been rejected.
	if fc.HasContent() {
		t.Errorf("expected cache to be empty due to rejected stale commit, but got content")
	}

	// A new fetch should generate fresh content.
	b, err := fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if b == nil || string(*b) != "stale content" {
		t.Errorf("expected fresh generation, got %v", b)
	}
	if atomic.LoadInt32(&generateCalls) != 2 {
		t.Errorf("expected 2 generate calls, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_ReentrantGenerate(t *testing.T) {
	var generateCalls int32
	var fc Content[[]byte]

	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)

			// Should not deadlock on re-entry during generation
			b, err := fc.Data()
			if err != nil {
				t.Errorf("expected no error on re-entry, got %v", err)
			}
			if b != nil {
				t.Errorf("expected nil cache during initial generation re-entry, got %v", string(*b))
			}

			val := []byte("content")
			return &val, nil
		}),
	)

	done := make(chan struct{})
	go func() {
		_, _ = fc.Data()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out due to deadlock")
	}

	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected 1 generate call, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func WaitWgWithTimeout(t *testing.T, wg *sync.WaitGroup) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("test timed out waiting for goroutines")
	}
}
