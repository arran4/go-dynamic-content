package utils

import (
	"errors"
	"fmt"
	"sync"
	"weak"
)

var (
	// ErrNoContent is returned when content cannot be generated or is missing.
	ErrNoContent = errors.New("no content available")

	// ErrInvalidContent is returned when content has failed validation.
	ErrInvalidContent = errors.New("content is invalid")
)

// Content provides thread-safe access to lazily or eagerly generated data.
type Content[T any] interface {
	// Data returns a non-nil pointer to the generated content, or an error.
	// If a generator fails (e.g. during eager loading), the error is retained
	// for inspection via Error(), while later Data() calls will retry generation.
	// A successful generation clears the retained error.
	// A generator that successfully returns (nil, nil) will be coerced to return (nil, ErrNoContent).
	Data() (*T, error)

	// Close clears the currently cached data from the underlying store and triggers the onClose callback.
	Close() error

	// String returns the generated content formatted as a string.
	// It suppresses errors internally; if data generation fails, it returns an empty string.
	String() string

	// Error evaluates the current state of the content cache.
	// It returns ErrInvalidContent if a configured validator fails.
	// If the cache is empty, it returns ErrNoContent (potentially wrapping a retained generator error).
	Error() error

	// HasContent returns true if the underlying store holds a generated value.
	HasContent() bool

	// Invalidate explicitly clears the cached content (and any retained generation error)
	// from the underlying store, and triggers the onInvalidate callback.
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
	lastErr    error
	mu         sync.Mutex
}

func (s *versionedStore[T]) Get() *T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.underlying.Get()
}

func (s *versionedStore[T]) GetError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
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
	s.lastErr = nil
	s.epoch++
}

type commitToken struct {
	epoch uint64
}

func (s *versionedStore[T]) BeginCommit() commitToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	return commitToken{epoch: s.epoch}
}

func (s *versionedStore[T]) CommitIfCurrent(token commitToken, val *T, err error) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.epoch == token.epoch {
		if val != nil {
			s.underlying.Set(val)
		}
		s.lastErr = err
		s.epoch++
		return true
	}
	return false
}

func wrapGenerator[T any](store *versionedStore[T], origGen func() (*T, error), onGen func(val *T, err error)) func() (*T, error) {
	var genMu sync.Mutex
	var generating bool

	return func() (*T, error) {
		genMu.Lock()
		if generating {
			val := store.Get()
			err := store.GetError()
			genMu.Unlock()
			if val == nil {
				if err != nil && !errors.Is(err, ErrNoContent) {
					return nil, fmt.Errorf("%w: %w", ErrNoContent, err)
				}
				return nil, ErrNoContent
			}
			return val, nil
		}
		generating = true
		token := store.BeginCommit()
		genMu.Unlock()

		defer func() {
			genMu.Lock()
			generating = false
			genMu.Unlock()
		}()

		genVal, genErr := origGen()

		if genErr == nil && genVal == nil {
			genErr = ErrNoContent
		}

		if genErr != nil {
			// Do not cache partial values if there is a generation error.
			store.CommitIfCurrent(token, nil, genErr)
		} else {
			store.CommitIfCurrent(token, genVal, nil)
		}

		if onGen != nil {
			onGen(genVal, genErr)
		}

		if genVal == nil && genErr != nil && !errors.Is(genErr, ErrNoContent) {
			return nil, fmt.Errorf("%w: %w", ErrNoContent, genErr)
		}

		return genVal, genErr
	}
}

func wrapValidator[T any](origIsVal func() bool) func() bool {
	var valMu sync.Mutex
	var validating bool

	return func() bool {
		valMu.Lock()
		if validating {
			valMu.Unlock()
			return true // Optimistic validity breaks recursion
		}
		validating = true
		valMu.Unlock()

		defer func() {
			valMu.Lock()
			validating = false
			valMu.Unlock()
		}()

		return origIsVal()
	}
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

	// Capture execution policy in composable private abstractions rather than fields
	if fc.generate != nil {
		fc.generate = wrapGenerator(vStore, fc.generate, fc.onGenerate)
	}

	if fc.isValid != nil {
		fc.isValid = wrapValidator[T](fc.isValid)
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
		if errStore, ok := fc.store.(interface{ GetError() error }); ok {
			if lastErr := errStore.GetError(); lastErr != nil {
				return nil, fmt.Errorf("%w: %w", ErrNoContent, lastErr)
			}
		}
		return nil, ErrNoContent
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
		return ErrInvalidContent
	}

	// 2. Evaluate content presence
	fc.mu.Lock()
	val := fc.store.Get()
	fc.mu.Unlock()

	if val == nil {
		if errStore, ok := fc.store.(interface{ GetError() error }); ok {
			if lastErr := errStore.GetError(); lastErr != nil {
				return fmt.Errorf("%w: %w", ErrNoContent, lastErr)
			}
		}
		return ErrNoContent
	}

	return nil
}
