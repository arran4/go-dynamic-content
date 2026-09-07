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
}

type versionedStore[T any] struct {
	underlying Store[T]
	epoch      uint64
	mu         sync.Mutex
}

func (s *versionedStore[T]) Get() *T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.underlying.Get()
}

func (s *versionedStore[T]) Set(val *T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.underlying.Set(val)
	s.epoch++
}

func (s *versionedStore[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.underlying.Clear()
	s.epoch++
}

func (s *versionedStore[T]) CurrentEpoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

func (s *versionedStore[T]) Commit(val *T, expectedEpoch uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch == expectedEpoch {
		s.underlying.Set(val)
		s.epoch++
		return true
	}
	return false
}

func NewContent[T any](opts ...Option[T]) Content[T] {
	fc := &contentImpl[T]{
		store: &MemoryStore[T]{},
		lazy:  true,
	}

	for _, opt := range opts {
		opt(fc)
	}

	vStore := &versionedStore[T]{
		underlying: fc.store,
		epoch:      1,
	}
	fc.store = vStore

	// Capture execution policy in composable closures rather than fields
	if fc.generate != nil {
		origGen := fc.generate
		origOnGen := fc.onGenerate

		var genMu sync.Mutex
		var generating bool

		fc.generate = func() (*T, error) {
			genMu.Lock()
			if generating {
				val := vStore.Get()
				genMu.Unlock()
				return val, nil
			}
			generating = true
			myEpoch := vStore.CurrentEpoch()
			genMu.Unlock()

			var genVal *T
			var genErr error
			func() {
				defer func() {
					genMu.Lock()
					generating = false
					genMu.Unlock()
					if r := recover(); r != nil {
						panic(r)
					}
				}()
				genVal, genErr = origGen()
			}()

			if genErr == nil && genVal != nil {
				vStore.Commit(genVal, myEpoch)
			}

			if origOnGen != nil {
				origOnGen(genVal, genErr)
			}

			return genVal, genErr
		}
	}

	if fc.isValid != nil {
		origIsVal := fc.isValid
		var valMu sync.Mutex
		var validating bool

		fc.isValid = func() bool {
			valMu.Lock()
			if validating {
				valMu.Unlock()
				return true // Optimistic validity breaks recursion
			}
			validating = true
			valMu.Unlock()

			var valid bool
			func() {
				defer func() {
					valMu.Lock()
					validating = false
					valMu.Unlock()
					if r := recover(); r != nil {
						panic(r)
					}
				}()
				valid = origIsVal()
			}()
			return valid
		}
	}

	if !fc.lazy {
		_, _ = fc.Data()
	}

	return fc
}

func (fc *contentImpl[T]) Data() (*T, error) {
	if fc.isValid != nil && !fc.isValid() {
		// Invalidate() implicitly clears store via vStore which properly coordinates epoch.
		_ = fc.Invalidate()
	}

	fc.mu.Lock()
	val := fc.store.Get()
	fc.mu.Unlock()

	if val != nil {
		return val, nil
	}

	if fc.generate == nil {
		return nil, nil
	}

	return fc.generate()
}

func (fc *contentImpl[T]) Close() error {
	fc.mu.Lock()
	fc.store.Clear()
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
	} else {
		// Even if empty, clear to advance the epoch so in-flight commits are rejected!
		fc.store.Clear()
	}
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
	// 1. Evaluate validity first to preserve precedence (wrapper inherently resolves recursive locks natively)
	if fc.isValid != nil && !fc.isValid() {
		return fmt.Errorf("content is invalid")
	}

	// 2. Evaluate content presence
	fc.mu.Lock()
	val := fc.store.Get()
	fc.mu.Unlock()

	if val == nil {
		return fmt.Errorf("no content available")
	}

	return nil
}
