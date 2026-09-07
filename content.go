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

	epoch        uint64
	generating   bool
	genWait      chan struct{} // blocks concurrent callers during generation
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
		if fc.generating {
			waitChan := fc.genWait
			fc.mu.Unlock()
			<-waitChan
			continue
		}

		val := fc.store.Get()
		snapshotEpoch := fc.epoch
		fc.mu.Unlock()

		// 2. Evaluate validity without lock
		if val != nil && fc.isValid != nil {
			valid := fc.isValid()
			if !valid {
				var triggeredInvalidate bool

				fc.mu.Lock()
				// Only clear if the epoch hasn't changed (meaning no new generation/invalidation happened)
				// and we still aren't generating.
				if fc.epoch == snapshotEpoch && fc.store.Get() != nil {
					fc.store.Clear()
					fc.epoch++
					triggeredInvalidate = true
				}
				fc.mu.Unlock()

				if triggeredInvalidate && fc.onInvalidate != nil {
					fc.onInvalidate()
				}
				continue // start over since it was invalid
			}
		}

		// 3. Return valid value or take ownership of generation
		fc.mu.Lock()
		if fc.generating {
			waitChan := fc.genWait
			fc.mu.Unlock()
			<-waitChan
			continue
		}

		if val := fc.store.Get(); val != nil {
			fc.mu.Unlock()
			return val, nil
		}

		// Take ownership
		fc.generating = true
		waitChan := make(chan struct{})
		fc.genWait = waitChan
		myEpoch := fc.epoch
		fc.mu.Unlock()

		if fc.generate == nil {
			fc.mu.Lock()
			fc.generating = false
			fc.genWait = nil
			close(waitChan)
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
				fc.genWait = nil
				close(waitChan)
				fc.mu.Unlock()
			}()
			genVal, genErr = fc.generate()
		}()

		// 5. Execute callback outside lock
		if fc.onGenerate != nil {
			fc.onGenerate(genVal, genErr)
		}

		// To prevent livelock, we return the successfully generated result
		// without re-validating it in this operation. Waiters who were blocked
		// will wake up, loop, and evaluate validity against the new epoch.
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
	defer fc.mu.Unlock()

	if fc.isValid != nil && !fc.isValid() {
		return fmt.Errorf("content is invalid")
	}
	if fc.store.Get() == nil {
		return fmt.Errorf("no content available")
	}
	return nil
}
