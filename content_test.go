package utils

import (
	"errors"
	"runtime"
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
				// With the new semantics, if it's in-flight and cache is empty, it returns ErrNoContent.
				if b != nil || (err != nil && !errors.Is(err, ErrNoContent)) {
					break
				}
				time.Sleep(2 * time.Millisecond) // yield and try again
			}

			if err != nil && !errors.Is(err, ErrNoContent) { // wait, if it finally returned ErrNoContent because it timed out or is in flight, but here we expect it to eventually return non-nil.
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

func TestContent_StorageOptionSemantics(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() Option[string] // Using func allows dynamic string allocation for GC
		expectWeak bool                  // false means memory storage
	}{
		{"UseWeakStorage_true", func() Option[string] { return UseWeakStorage[string](true) }, true},
		{"UseWeakStorage_false", func() Option[string] { return UseWeakStorage[string](false) }, false},
		{"UseMemoryStorage_true", func() Option[string] { return UseMemoryStorage[string](true) }, false},
		{"UseMemoryStorage_false", func() Option[string] { return UseMemoryStorage[string](false) }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := string([]byte{'i', 'n', 'i', 't', 'i', 'a', 'l'})
			opts := []Option[string]{WithValue[string](val), tt.setup()}
			fc := NewContent[string](opts...)

			str := fc.String()
			if str != "initial" {
				t.Errorf("expected value to be preserved, got '%s'", str)
			}

			runtime.GC()
			survived := fc.String() != ""

			if tt.expectWeak && survived {
				t.Errorf("expected value to be GC'd under weak storage")
			} else if !tt.expectWeak && !survived {
				t.Errorf("expected value to survive GC under memory storage")
			}
		})
	}
}

func TestContent_StorageOptionSemantics_ValueOrdering(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(val string) []Option[string]
		expectWeak bool
	}{
		{"Value_then_WeakTrue", func(v string) []Option[string] {
			return []Option[string]{WithValue[string](v), UseWeakStorage[string](true)}
		}, true},
		{"WeakTrue_then_Value", func(v string) []Option[string] {
			return []Option[string]{UseWeakStorage[string](true), WithValue[string](v)}
		}, true},
		{"Value_then_WeakFalse", func(v string) []Option[string] {
			return []Option[string]{WithValue[string](v), UseWeakStorage[string](false)}
		}, false},
		{"MemoryFalse_then_Value", func(v string) []Option[string] {
			return []Option[string]{UseMemoryStorage[string](false), WithValue[string](v)}
		}, true},
		{"Weak_then_Memory", func(v string) []Option[string] {
			return []Option[string]{WithValue[string](v), UseWeakStorage[string](true), UseMemoryStorage[string](true)}
		}, false},
		{"Memory_then_Weak", func(v string) []Option[string] {
			return []Option[string]{WithValue[string](v), UseMemoryStorage[string](true), UseWeakStorage[string](true)}
		}, true},
		{"WeakTrue_then_MemoryFalse", func(v string) []Option[string] {
			return []Option[string]{WithValue[string](v), UseWeakStorage[string](true), UseMemoryStorage[string](false)}
		}, true},
		{"MemoryTrue_then_WeakFalse", func(v string) []Option[string] {
			return []Option[string]{WithValue[string](v), UseMemoryStorage[string](true), UseWeakStorage[string](false)}
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := string([]byte{'v', 'a', 'l', 'u', 'e'})
			fc := NewContent[string](tt.setup(val)...)

			str := fc.String()
			if str != "value" {
				t.Errorf("expected value to be preserved, got '%s'", str)
			}

			runtime.GC()
			survived := fc.String() != ""

			if tt.expectWeak && survived {
				t.Errorf("expected value to be GC'd under weak storage")
			} else if !tt.expectWeak && !survived {
				t.Errorf("expected value to survive GC under memory storage")
			}
		})
	}
}

func TestContent_LoadingOptionSemantics(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() []Option[string]
		expectLazy bool
	}{
		{"UseLazyLoading_true", func() []Option[string] { return []Option[string]{UseLazyLoading[string](true)} }, true},
		{"UseLazyLoading_false", func() []Option[string] { return []Option[string]{UseLazyLoading[string](false)} }, false},
		{"UseEagerLoading_true", func() []Option[string] { return []Option[string]{UseEagerLoading[string](true)} }, false},
		{"UseEagerLoading_false", func() []Option[string] { return []Option[string]{UseEagerLoading[string](false)} }, true},
		{"Lazy_then_Eager", func() []Option[string] {
			return []Option[string]{UseLazyLoading[string](true), UseEagerLoading[string](true)}
		}, false},
		{"Eager_then_Lazy", func() []Option[string] {
			return []Option[string]{UseEagerLoading[string](true), UseLazyLoading[string](true)}
		}, true},
		{"LazyTrue_then_EagerFalse", func() []Option[string] {
			return []Option[string]{UseLazyLoading[string](true), UseEagerLoading[string](false)}
		}, true},
		{"EagerTrue_then_LazyFalse", func() []Option[string] {
			return []Option[string]{UseEagerLoading[string](true), UseLazyLoading[string](false)}
		}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generated := false
			genOpt := WithGenerator(func() (*string, error) {
				generated = true
				s := "gen"
				return &s, nil
			})

			opts := append([]Option[string]{genOpt}, tt.setup()...)
			fc := NewContent[string](opts...)

			if tt.expectLazy && generated {
				t.Errorf("expected lazy loading, but generator was called immediately")
			} else if !tt.expectLazy && !generated {
				t.Errorf("expected eager loading, but generator was not called immediately")
			}

			// For eager, it should have content immediately
			if !tt.expectLazy && !fc.HasContent() {
				t.Errorf("expected eager loading to make content available immediately")
			}

			// Requesting data should always invoke it if it hasn't been yet
			_, _ = fc.Data()
			if !generated {
				t.Errorf("generator was never called even after Data()")
			}
		})
	}
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
	var onGenerateCalls int32

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
		WithOnGenerate[[]byte](func(val *[]byte, err error) {
			atomic.AddInt32(&onGenerateCalls, 1)
		}),
	)

	// Force a generation
	var wg sync.WaitGroup
	wg.Add(1)

	// Use channels to capture the result of the first blocked Data() call
	firstDataDone := make(chan struct{})
	var firstDataErr error
	var firstDataVal *[]byte

	go func() {
		defer wg.Done()
		firstDataVal, firstDataErr = fc.Data()
		close(firstDataDone)
	}()

	// Wait for generator to actually hold ownership
	<-genStarted

	// Issue invalidation while the generator holds ownership but hasn't returned
	_ = fc.Invalidate()

	// Unblock generator to commit
	close(genWait)

	// Wait for generator to finish
	WaitWgWithTimeout(t, &wg)
	<-firstDataDone

	// Given we invalidated during an active generation cycle, the epoch advanced.
	// The generator's commit MUST have been rejected.
	if fc.HasContent() {
		t.Errorf("expected cache to be empty due to rejected stale commit, but got content")
	}

	if firstDataVal != nil {
		t.Errorf("expected Data() caller to receive nil due to stale commit rejection, got %v", firstDataVal)
	}

	if !errors.Is(firstDataErr, ErrNoContent) {
		t.Errorf("expected Data() caller to receive ErrNoContent due to stale commit rejection, got %v", firstDataErr)
	}

	if atomic.LoadInt32(&onGenerateCalls) != 0 {
		t.Errorf("expected onGenerate NOT to be called for rejected result, but it was called %d times", atomic.LoadInt32(&onGenerateCalls))
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
	if atomic.LoadInt32(&onGenerateCalls) != 1 {
		t.Errorf("expected onGenerate to be called once for successful generation, got %d", atomic.LoadInt32(&onGenerateCalls))
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

	var generateCalls int32

	var fc Content[[]byte]
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

	var generateCalls int32
	var invalidateCalls int32

	var fc Content[[]byte]
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

	var closeCalls int32

	var fc Content[[]byte]
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
			if err != nil && err != ErrNoContent { // We expect ErrNoContent when in-flight and cache is empty
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
			if err != nil && err != ErrNoContent {
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

func TestContent_ValidationDeadlock_Concurrent(t *testing.T) {
	// Tests explicit stale-while-validation visibility for overlapping callers (#43)
	// 1. cached content exists;
	// 2. caller A enters validator and blocks;
	// 3. caller B calls Data() and gets cached value (no blocking);
	// 4. caller B calls Error() and gets optimistic validity;
	// 5. caller A unblocks, returns false;
	// 6. next call to Data()/Error() behaves correctly.

	var generateCalls int32
	var isValidCalls int32

	valStarted := make(chan struct{})
	valWait := make(chan struct{})
	var valStartedOnce sync.Once

	fc := NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			b := []byte("content")
			return &b, nil
		}),
		WithValidator[[]byte](func() bool {
			calls := atomic.AddInt32(&isValidCalls, 1)
			if calls == 2 {
				// Block only on the second evaluation (which we'll trigger explicitly)
				valStartedOnce.Do(func() { close(valStarted) })
				<-valWait
				return false
			}
			return true
		}),
	)

	// Populate cache first
	_, _ = fc.Data()
	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Fatalf("expected 1 generate call, got %d", atomic.LoadInt32(&generateCalls))
	}

	// Caller A triggers validation
	var wgA sync.WaitGroup
	wgA.Add(1)
	var errA error
	go func() {
		defer wgA.Done()
		errA = fc.Error()
	}()

	// Wait for Caller A to enter the validator and block
	<-valStarted

	// Caller B reads concurrent to in-flight validation
	var wgB sync.WaitGroup
	wgB.Add(1)

	bDataDone := make(chan struct{})
	bErrDone := make(chan struct{})
	go func() {
		defer wgB.Done()

		// Caller B calling Error() overlapping validation observes optimistic validity
		errB := fc.Error()
		if errB != nil {
			t.Errorf("expected Error() while validation is in-flight to return nil (optimistic), got %v", errB)
		}
		close(bErrDone)

		// Caller B calls Data() while validation is in-flight
		valB, dataErrB := fc.Data()
		if dataErrB != nil {
			t.Errorf("expected Data() while validation is in-flight to succeed, got %v", dataErrB)
		}
		if valB == nil || string(*valB) != "content" {
			t.Errorf("expected Data() to return current cached value, got %v", valB)
		}
		close(bDataDone)
	}()

	// Ensure caller B actually completes without blocking on Caller A
	select {
	case <-bErrDone:
	case <-time.After(1 * time.Second):
		t.Fatalf("Caller B blocked calling Error() during active validation")
	}

	select {
	case <-bDataDone:
	case <-time.After(1 * time.Second):
		t.Fatalf("Caller B blocked calling Data() during active validation")
	}

	WaitWgWithTimeout(t, &wgB)

	// Unblock Caller A so validation completes and returns false
	close(valWait)
	WaitWgWithTimeout(t, &wgA)

	// Caller A should observe the false validation result
	if !errors.Is(errA, ErrInvalidContent) {
		t.Errorf("expected Caller A to receive ErrInvalidContent due to false validator return, got %v", errA)
	}

	// Next call to Error() should trigger re-evaluation (calls == 3), which returns true (valid).
	errAfter := fc.Error()
	if errAfter != nil { // Because calls == 3 returns true now
		t.Errorf("expected Error() to evaluate true on subsequent call, got %v", errAfter)
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

func TestContent_ReentrantOnGenerateLifecycle(t *testing.T) {
	var generateCalls int32

	var fc Content[[]byte]
	fc = NewContent[[]byte](
		WithGenerator[[]byte](func() (*[]byte, error) {
			atomic.AddInt32(&generateCalls, 1)
			b := []byte("content")
			return &b, nil
		}),
		WithOnGenerate[[]byte](func(val *[]byte, err error) {
			// Because onGenerate happens while `generating == true`,
			// calling Data() here will return the already-committed `val` (stale-while-revalidate),
			// and importantly, it will NOT trigger another generation.
			b, _ := fc.Data()
			if b == nil || string(*b) != "content" {
				t.Errorf("expected observed 'content', got %v", b)
			}
		}),
	)

	_, _ = fc.Data()

	if atomic.LoadInt32(&generateCalls) != 1 {
		t.Errorf("expected exactly 1 generation call, but onGenerate incorrectly became a second owner: got %d calls", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_EagerFailure_Error(t *testing.T) {
	expectedErr := errors.New("eager failure")
	fc := NewContent(
		WithGenerator(func() (*string, error) {
			return nil, expectedErr
		}),
		UseEagerLoading[string](true),
	)

	err := fc.Error()
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent, got %v", err)
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped expectedErr, got %v", err)
	}
}

func TestContent_EagerFailure_SuccessfulData(t *testing.T) {
	callCount := 0
	expectedErr := errors.New("eager failure")
	fc := NewContent(
		WithGenerator(func() (*string, error) {
			callCount++
			if callCount == 1 {
				return nil, expectedErr
			}
			val := "success"
			return &val, nil
		}),
		UseEagerLoading[string](true),
	)

	// Verify error state after eager failure
	err := fc.Error()
	if !errors.Is(err, ErrNoContent) || !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped ErrNoContent and expectedErr, got %v", err)
	}

	// Retry via Data()
	val, err := fc.Data()
	if err != nil {
		t.Fatalf("expected successful Data() call, got error: %v", err)
	}
	if val == nil || *val != "success" {
		t.Errorf("expected 'success', got %v", val)
	}
}

func TestContent_RepeatedGenerationFailure(t *testing.T) {
	callCount := 0
	expectedErr1 := errors.New("failure 1")
	expectedErr2 := errors.New("failure 2")
	fc := NewContent(
		WithGenerator(func() (*string, error) {
			callCount++
			if callCount == 1 {
				return nil, expectedErr1
			}
			return nil, expectedErr2
		}),
	)

	// First call
	_, err := fc.Data()
	if !errors.Is(err, ErrNoContent) || !errors.Is(err, expectedErr1) {
		t.Errorf("expected wrapped ErrNoContent and expectedErr1, got %v", err)
	}

	// Verify Error()
	err = fc.Error()
	if !errors.Is(err, expectedErr1) {
		t.Errorf("expected Error() to return expectedErr1, got %v", err)
	}

	// Second call
	_, err = fc.Data()
	if !errors.Is(err, ErrNoContent) || !errors.Is(err, expectedErr2) {
		t.Errorf("expected wrapped ErrNoContent and expectedErr2, got %v", err)
	}

	// Verify Error()
	err = fc.Error()
	if !errors.Is(err, expectedErr2) {
		t.Errorf("expected Error() to return expectedErr2, got %v", err)
	}
}

func TestContent_InvalidateClearsError(t *testing.T) {
	callCount := 0
	expectedErr := errors.New("generation error")
	fc := NewContent(
		WithGenerator(func() (*string, error) {
			callCount++
			if callCount == 1 {
				return nil, expectedErr
			}
			val := "success"
			return &val, nil
		}),
		UseEagerLoading[string](true),
	)

	// Verify error state after eager failure
	err := fc.Error()
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected expectedErr, got %v", err)
	}

	// Invalidate should clear the error
	_ = fc.Invalidate()

	// Error should now just be ErrNoContent, not wrapped expectedErr (or we can just check Data)
	// Actually, Error() will trigger if we check.
	// Wait, if generator is still there, Data() will run it again.
	// If we just check Error() after Invalidate, it might still have the error unless Invalidate cleared it?
	// Wait, Invalidate clears store. Clear() clears the error!
	err = fc.Error()
	if errors.Is(err, expectedErr) {
		t.Errorf("expected expectedErr to be cleared, got %v", err)
	}
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent, got %v", err)
	}
}

func TestContent_ValidatorFailureOverridesGeneratorError(t *testing.T) {
	expectedErr := errors.New("generation error")
	fc := NewContent(
		WithGenerator(func() (*string, error) {
			return nil, expectedErr
		}),
		WithValidator[string](func() bool {
			return false // Validator says invalid
		}),
		UseEagerLoading[string](true),
	)

	// Since validator fails, Error() should return ErrInvalidContent.
	err := fc.Error()
	if !errors.Is(err, ErrInvalidContent) {
		t.Errorf("expected ErrInvalidContent, got %v", err)
	}

	// Data() should clear the invalid content (and epoch) via Invalidate, then try to generate.
	// Since generator fails again, it will return the new error.
	// Wait, if it fails validation, Invalidate is called.
	_, err = fc.Data()
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected wrapped expectedErr after regeneration, got %v", err)
	}
}

func TestContent_GeneratorReturnsNilNil(t *testing.T) {
	fc := NewContent(
		WithGenerator(func() (*string, error) {
			return nil, nil // Should be mapped to ErrNoContent
		}),
	)

	_, err := fc.Data()
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent, got %v", err)
	}

	err = fc.Error()
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent from Error(), got %v", err)
	}
}

func TestContent_NoGeneratorConfigured(t *testing.T) {
	fc := NewContent[string]() // No generator

	_, err := fc.Data()
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent, got %v", err)
	}

	err = fc.Error()
	if !errors.Is(err, ErrNoContent) {
		t.Errorf("expected ErrNoContent from Error(), got %v", err)
	}
}

func TestContent_ZeroValueIsLegitimate(t *testing.T) {
	fc := NewContent(
		WithGenerator(func() (*int, error) {
			val := 0 // Zero value
			return &val, nil
		}),
	)

	val, err := fc.Data()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val == nil || *val != 0 {
		t.Errorf("expected 0, got %v", val)
	}

	err = fc.Error()
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestContent_PartialValueWithGeneratorError(t *testing.T) {
	callCount := 0
	expectedErr := errors.New("generation failure with partial value")

	fc := NewContent(
		WithGenerator(func() (*string, error) {
			callCount++
			if callCount == 1 {
				partial := "partial value"
				return &partial, expectedErr
			}
			success := "success value"
			return &success, nil
		}),
	)

	// First call should fail and not cache the partial value, but still return it to caller.
	val, err := fc.Data()
	if val == nil || *val != "partial value" {
		t.Errorf("expected partial value, got %v", val)
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}

	// Error() should report the retained error.
	err = fc.Error()
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected Error() to return %v, got %v", expectedErr, err)
	}

	// Retry via Data(), should succeed and clear the error state.
	val, err = fc.Data()
	if err != nil {
		t.Fatalf("expected successful Data(), got error: %v", err)
	}
	if val == nil || *val != "success value" {
		t.Errorf("expected 'success value', got %v", val)
	}

	// Error() should now report nil error since it succeeded.
	err = fc.Error()
	if err != nil {
		t.Errorf("expected nil error after successful generation, got %v", err)
	}
}
