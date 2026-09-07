package utils

import (
	"fmt"
	"sync"
	"weak"
)

type Content[T any] interface {
	Data() (*T, error)
	Close() error
	String() string
	Error() error
	HasContent() bool
	Invalidate() error
}

type Store[T any] interface {
	Get() *T
	Set(*T)
	Clear()
}

type WeakStore[T any] struct {
	ptr weak.Pointer[T]
}

func (s *WeakStore[T]) Get() *T {
	return s.ptr.Value()
}

func (s *WeakStore[T]) Set(val *T) {
	if val == nil {
		s.ptr = weak.Pointer[T]{}
	} else {
		s.ptr = weak.Make(val)
	}
}

func (s *WeakStore[T]) Clear() {
	s.ptr = weak.Pointer[T]{}
}

type MemoryStore[T any] struct {
	val *T
}

func (s *MemoryStore[T]) Get() *T {
	return s.val
}

func (s *MemoryStore[T]) Set(val *T) {
	s.val = val
}

func (s *MemoryStore[T]) Clear() {
	s.val = nil
}

type Option[T any] func(*contentImpl[T])

func UseWeakStorage[T any](use bool) Option[T] {
	return func(fc *contentImpl[T]) {
		if use {
			fc.store = &WeakStore[T]{}
		}
	}
}

func UseMemoryStorage[T any](use bool) Option[T] {
	return func(fc *contentImpl[T]) {
		if use {
			fc.store = &MemoryStore[T]{}
		}
	}
}

func UseLazyLoading[T any](use bool) Option[T] {
	return func(fc *contentImpl[T]) {
		if use {
			fc.lazy = true
		}
	}
}

func UseEagerLoading[T any](use bool) Option[T] {
	return func(fc *contentImpl[T]) {
		if use {
			fc.lazy = false
		}
	}
}

func WithGenerator[T any](generate func() (*T, error)) Option[T] {
	return func(fc *contentImpl[T]) {
		fc.generate = generate
	}
}

func WithValidator[T any](isValid func() bool) Option[T] {
	return func(fc *contentImpl[T]) {
		fc.isValid = isValid
	}
}

func WithValue[T any](val T) Option[T] {
	return func(fc *contentImpl[T]) {
		fc.store.Set(&val)
	}
}

func WithOnGenerate[T any](cb func(val *T, err error)) Option[T] {
	return func(fc *contentImpl[T]) {
		fc.onGenerate = cb
	}
}

func WithOnInvalidate[T any](cb func()) Option[T] {
	return func(fc *contentImpl[T]) {
		fc.onInvalidate = cb
	}
}

func WithOnClose[T any](cb func()) Option[T] {
	return func(fc *contentImpl[T]) {
		fc.onClose = cb
	}
}

type contentImpl[T any] struct {
	mu           sync.Mutex
	store        Store[T]
	lazy         bool
	generate     func() (*T, error)
	isValid      func() bool
	onGenerate   func(val *T, err error)
	onInvalidate func()
	onClose      func()

	epoch      uint64
	generating bool
	validating bool
}

func NewContent[T any](opts ...Option[T]) Content[T] {
	fc := &contentImpl[T]{
		store: &MemoryStore[T]{},
		lazy:  true,
		epoch: 1, // Start at 1
	}

	for _, opt := range opts {
		opt(fc)
	}

	if !fc.lazy {
		_, _ = fc.Data()
	}

	return fc
}

func (fc *contentImpl[T]) Data() (*T, error) {
	for {
		// 1. Snapshot state under lock
		fc.mu.Lock()
		val := fc.store.Get()
		isGen := fc.generating
		isVal := fc.validating
		snapshotEpoch := fc.epoch
		fc.mu.Unlock()

		// IN-FLIGHT CONCURRENCY STATE:
		// If a generation is already active, or we are currently validating, we do not wait.
		// We immediately return the currently committed cache state (which may be nil or stale).
		// This mathematically prevents self-deadlocks on direct `generate -> Data()`
		// and `isValid -> Data()` re-entry, while fulfilling the requirement that concurrent
		// callers do not start uncontrolled duplicate generations.
		if isGen || isVal {
			return val, nil
		}

		// 2. Evaluate validity without lock
		if val != nil && fc.isValid != nil {
			fc.mu.Lock()
			// Another goroutine might have started validating between our unlock and here.
			if fc.validating {
				fc.mu.Unlock()
				return val, nil
			}
			fc.validating = true
			fc.mu.Unlock()

			var isValidPanic bool
			var valid bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						isValidPanic = true
						fc.mu.Lock()
						fc.validating = false
						fc.mu.Unlock()
						panic(r)
					}
				}()
				valid = fc.isValid()
			}()

			// We safely got past isValid without panicking
			if !isValidPanic {
				var triggeredInvalidate bool

				fc.mu.Lock()
				fc.validating = false
				// Only clear if the epoch hasn't changed (meaning no new generation/invalidation happened)
				// and we aren't currently generating.
				if !valid && fc.epoch == snapshotEpoch && fc.store.Get() != nil && !fc.generating {
					fc.store.Clear()
					fc.epoch++
					triggeredInvalidate = true
				}
				fc.mu.Unlock()

				if triggeredInvalidate && fc.onInvalidate != nil {
					fc.onInvalidate()
				}

				if !valid {
					continue // start over since it was invalid
				}
			}
		}

		// 3. Return valid value or take ownership of generation
		fc.mu.Lock()
		if fc.generating {
			val := fc.store.Get()
			fc.mu.Unlock()
			return val, nil
		}

		if val := fc.store.Get(); val != nil {
			fc.mu.Unlock()
			return val, nil
		}

		// Take ownership
		fc.generating = true
		myEpoch := fc.epoch
		fc.mu.Unlock()

		if fc.generate == nil {
			fc.mu.Lock()
			fc.generating = false
			fc.mu.Unlock()
			return nil, nil
		}

		// 4. Generate without lock, catching panics
		var genVal *T
		var genErr error

		func() {
			defer func() {
				// Commit and release ownership
				fc.mu.Lock()
				// Only commit if epoch hasn't been advanced by Invalidate/Close
				if fc.epoch == myEpoch {
					if genErr == nil && genVal != nil {
						fc.store.Set(genVal)
					}
					fc.epoch++
				}
				fc.generating = false
				fc.mu.Unlock()
			}()
			genVal, genErr = fc.generate()
		}()

		// 5. Execute callback outside lock
		if fc.onGenerate != nil {
			fc.onGenerate(genVal, genErr)
		}

		// To prevent livelock, we return the successfully generated result
		// without re-validating it in this operation.
		if genErr != nil {
			return nil, genErr
		}
		return genVal, nil
	}
}

func (fc *contentImpl[T]) Close() error {
	fc.mu.Lock()
	fc.store.Clear()
	fc.epoch++
	fc.mu.Unlock()

	if fc.onClose != nil {
		fc.onClose()
	}
	return nil
}

func (fc *contentImpl[T]) HasContent() bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.store.Get() != nil
}

func (fc *contentImpl[T]) Invalidate() error {
	var triggeredInvalidate bool
	fc.mu.Lock()
	if fc.store.Get() != nil {
		fc.store.Clear()
		triggeredInvalidate = true
	}
	fc.epoch++
	fc.mu.Unlock()

	if triggeredInvalidate && fc.onInvalidate != nil {
		fc.onInvalidate()
	}
	return nil
}

func (fc *contentImpl[T]) String() string {
	val, err := fc.Data()
	if err != nil {
		return "" // Suppress error for templates
	}
	if val == nil {
		return ""
	}

	// We use any(*val) to be able to switch its type safely
	switch v := any(*val).(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (fc *contentImpl[T]) Error() error {
	fc.mu.Lock()
	val := fc.store.Get()
	isGen := fc.generating
	isVal := fc.validating
	fc.mu.Unlock()

	// IN-FLIGHT CONCURRENCY STATE:
	// If generating or validating is actively in progress, we return the currently committed
	// cache snapshot state. This deliberately assumes optimistic validity if content exists,
	// securely breaking direct `isValid -> Error()` infinite recursion cycles without deadlocks,
	// and providing a coherent non-blocking state to concurrent callers.
	if isGen || isVal {
		if val == nil {
			return fmt.Errorf("no content available")
		}
		return nil
	}

	// 1. Evaluate validity first to preserve error precedence
	if fc.isValid != nil {
		fc.mu.Lock()
		if fc.validating {
			fc.mu.Unlock()
			val = fc.store.Get()
			if val == nil {
				return fmt.Errorf("no content available")
			}
			return nil
		}
		fc.validating = true
		fc.mu.Unlock()

		var valid bool
		var isValidPanic bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					isValidPanic = true
					fc.mu.Lock()
					fc.validating = false
					fc.mu.Unlock()
					panic(r)
				}
			}()
			valid = fc.isValid()
		}()

		if !isValidPanic {
			fc.mu.Lock()
			fc.validating = false
			fc.mu.Unlock()

			if !valid {
				return fmt.Errorf("content is invalid")
			}
		}
	}

	// 2. Evaluate content presence second
	fc.mu.Lock()
	val = fc.store.Get()
	fc.mu.Unlock()

	if val == nil {
		return fmt.Errorf("no content available")
	}

	return nil
}
