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

			// Non-blocking single-flight model means concurrent callers get nil or stale
			// if generation is in-flight. To assert strong eventual consistency, we retry.
			var b *[]byte
			var err error
			for j := 0; j < 50; j++ {
				b, err = fc.Data()
				if err != nil || b != nil {
					break
				}
				time.Sleep(2 * time.Millisecond) // yield and try again
			}

			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if b == nil {
				t.Errorf("expected non-nil data eventually")
			} else if string(*b) != "hello world" {
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

func TestContent_ReentrantOnGenerate(t *testing.T) {
	var fc Content[[]byte]
	var generateCalls int32
	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			b := []byte("content")
			return &b, nil
		}),
		WithOnGenerate[[]byte](func(val *[]byte, err error) {
			atomic.AddInt32(&generateCalls, 1)

			// Re-entry check: should not deadlock and should observe generated state.
			done := make(chan struct{})
			go func() {
				v, _ := fc.Data()
				if string(*v) != "content" {
					t.Errorf("expected observed 'content', got %v", v)
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatalf("TestContent_ReentrantOnGenerate timed out (deadlock)")
			}
		}),
	)

	_, _ = fc.Data()
	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected 1 generate call, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_ReentrantOnInvalidate(t *testing.T) {
	var fc Content[[]byte]
	var generateCalls int32
	var invalidateCalls int32

	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			b := []byte("content")
			return &b, nil
		}),
		WithOnInvalidate[[]byte](func() {
			atomic.AddInt32(&invalidateCalls, 1)

			// Re-entry check: should not deadlock and should observe empty state (HasContent = false)
			done := make(chan struct{})
			go func() {
				if fc.HasContent() {
					t.Errorf("expected false, got true")
				}
				// Fetching Data() would trigger regeneration, which is also fine.
				_, _ = fc.Data()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatalf("TestContent_ReentrantOnInvalidate timed out (deadlock)")
			}
		}),
	)

	_, _ = fc.Data()

	_ = fc.Invalidate()
	if atomic.LoadInt32(&invalidateCalls) != 1 {
		t.Errorf("expected 1 invalidate call, got %d", atomic.LoadInt32(&invalidateCalls))
	}
	if atomic.LoadInt32(&generateCalls) != 2 {
		t.Errorf("expected 2 generate calls, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_ReentrantOnClose(t *testing.T) {
	var fc Content[[]byte]
	var closeCalls int32
	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			b := []byte("content")
			return &b, nil
		}),
		WithOnClose[[]byte](func() {
			atomic.AddInt32(&closeCalls, 1)

			// Re-entry check: should not deadlock and observe empty state
			done := make(chan struct{})
			go func() {
				if fc.HasContent() {
					t.Errorf("expected false, got true")
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatalf("TestContent_ReentrantOnClose timed out (deadlock)")
			}
		}),
	)

	_, _ = fc.Data()
	_ = fc.Close()
	if atomic.LoadInt32(&closeCalls) != 1 {
		t.Errorf("expected 1 close call, got %d", atomic.LoadInt32(&closeCalls))
	}
}

func TestContent_ReentrantIsValid(t *testing.T) {
	var fc Content[[]byte]
	var generateCalls int32
	var isValidCalls int32

	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			b := []byte("content")
			return &b, nil
		}),
		WithValidator[[]byte](func() bool {
			calls := atomic.AddInt32(&isValidCalls, 1)
			if calls == 2 {
				// Direct Re-entry during validation evaluation should not infinitely recurse
				// or deadlock.
				done := make(chan struct{})
				go func() {
					// These should both safely return the cached state without recursing isValid
					_ = fc.Error()
					_, _ = fc.Data()
					close(done)
				}()
				select {
				case <-done:
				case <-time.After(1 * time.Second):
					t.Fatalf("TestContent_ReentrantIsValid timed out (infinite recursion or deadlock)")
				}
			}
			return true
		}),
	)

	_, _ = fc.Data()
	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected 1 generate call, got %d", atomic.LoadInt32(&generateCalls))
	}

	_, _ = fc.Data()
	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected 1 generate call, got %d", atomic.LoadInt32(&generateCalls))
	}
	if atomic.LoadInt32(&isValidCalls) < 1 {
		t.Errorf("expected isValid to be called at least once")
	}
}

func TestContent_ConcurrentData_InFlight(t *testing.T) {
	var generateCalls int32

	genStarted := make(chan struct{})
	genWait := make(chan struct{})

	// Ensure we only close genStarted once. We place it here so it lives
	// across generator invocations.
	var genStartedOnce sync.Once

	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			genStartedOnce.Do(func() { close(genStarted) }) // Signal that generation block has locked
			<-genWait                                       // Keep generation artificially in-flight
			b := []byte("concurrent content")
			return &b, nil
		}),
	)

	// Fire the initial generator
	var initWg sync.WaitGroup
	initWg.Add(1)
	go func() {
		defer initWg.Done()
		_, _ = fc.Data()
	}()

	// Ensure the generator owns the process before starting concurrents
	<-genStarted

	var concurrentFirstReadWg sync.WaitGroup
	var concurrentSecondReadWg sync.WaitGroup
	var errorCount int32

	concurrentFirstReadWg.Add(50)
	concurrentSecondReadWg.Add(50)

	// Create a channel to signal concurrent callers that the generator has committed
	genCommitted := make(chan struct{})

	// Start concurrent callers while in-flight
	for i := 0; i < 50; i++ {
		go func() {
			defer concurrentSecondReadWg.Done()

			// 1. Explicit in-flight read
			b, err := fc.Data()
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
			}
			// Should be nil because it's an in-flight fetch and cache is empty
			if b != nil {
				atomic.AddInt32(&errorCount, 1)
			}
			concurrentFirstReadWg.Done()

			// 2. Wait for generator to finish explicitly using channels instead of sleeps
			<-genCommitted

			// 3. Second read should get the committed value
			b2, err2 := fc.Data()
			if err2 != nil {
				atomic.AddInt32(&errorCount, 1)
			}
			if b2 == nil || string(*b2) != "concurrent content" {
				atomic.AddInt32(&errorCount, 1)
			}
		}()
	}

	// Synchronize without sleeps: wait until all 50 concurrent callers have hit Data()
	// at least once, verifying they safely observed the in-flight state without deadlocking.
	WaitWgWithTimeout(t, &concurrentFirstReadWg)

	// Release the generator
	close(genWait)

	// Wait for the initial generator to securely finish and commit
	WaitWgWithTimeout(t, &initWg)

	// Signal to all concurrent callers that they can now do their second read
	close(genCommitted)

	// Ensure everyone finishes gracefully
	WaitWgWithTimeout(t, &concurrentSecondReadWg)

	// Verify behavior
	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected exactly 1 generation call, got %d", atomic.LoadInt32(&generateCalls))
	}
	if atomic.LoadInt32(&errorCount) != 0 {
		t.Errorf("expected 0 errors and matching strings across concurrents, got %d misses", atomic.LoadInt32(&errorCount))
	}
}

func TestContent_ReentrantGenerate(t *testing.T) {
	var generateCalls int32
	var fc Content[[]byte]

	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)

			// Direct re-entry check: should return nil immediately, not deadlock
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
		t.Fatalf("TestContent_ReentrantGenerate timed out (deadlock)")
	}

	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected 1 generate call, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_ReentrantIsValidDirect(t *testing.T) {
	var generateCalls int32
	var isValidCalls int32
	var fc Content[[]byte]

	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			b := []byte("content")
			return &b, nil
		}),
		WithValidator[[]byte](func() bool {
			calls := atomic.AddInt32(&isValidCalls, 1)
			if calls == 2 {
				// Direct Re-entry! The validator ITSELF calls Data/Error,
				// NOT a separate goroutine.
				_ = fc.Error()
				_, _ = fc.Data()
			}
			return true
		}),
	)

	// Fetch 1: Populates
	_, _ = fc.Data()

	done := make(chan struct{})
	go func() {
		// Fetch 2: Evaluates isValid (calls == 2), which triggers the direct re-entry
		_, _ = fc.Data()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("TestContent_ReentrantIsValidDirect timed out (infinite recursion or deadlock)")
	}
}

func TestContent_ErrorPrecedence(t *testing.T) {
	fc := NewContent[[]byte](
		WithValidator[[]byte](func() bool {
			return false
		}),
	)

	err := fc.Error()
	if err == nil || err.Error() != "content is invalid" {
		t.Errorf("expected 'content is invalid', got %v", err)
	}
}

func TestContent_ErrorInFlightState(t *testing.T) {
	var isValidCalls int32
	fc := NewContent[[]byte](
		WithValidator[[]byte](func() bool {
			atomic.AddInt32(&isValidCalls, 1)
			return false
		}),
	)

	// Start with empty cache. The error should natively report invalid.
	_ = fc.Error()
	if atomic.LoadInt32(&isValidCalls) != 1 {
		t.Errorf("expected 1 call to isValid, got %d", atomic.LoadInt32(&isValidCalls))
	}
}
