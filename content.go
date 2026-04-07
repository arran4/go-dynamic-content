package utils

import (
	"fmt"
	"sync"
	"weak"
)

type Content[T any] interface {
	Data() (*T, error)
	Close() error
	SetGenerator(func() (*T, error))
	String() string
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

type UseWeakStorage bool
type UseMemoryStorage bool
type UseLazyLoading bool
type UseEagerLoading bool

type WithGenerator[T any] func() (*T, error)
type WithValue[T any] struct { val T }

func NewWithValue[T any](val T) WithValue[T] {
	return WithValue[T]{val: val}
}

type contentConfig[T any] struct {
	store    Store[T]
	lazy     bool
	generate func() (*T, error)
}

type defaultContent[T any] struct {
	mu       sync.Mutex
	store    Store[T]
	lazy     bool
	generate func() (*T, error)
}

func NewContent[T any](opts ...any) Content[T] {
	cfg := contentConfig[T]{
		store: &MemoryStore[T]{},
		lazy:  true,
	}

	for _, opt := range opts {
		switch o := opt.(type) {
		case UseWeakStorage:
			if o {
				cfg.store = &WeakStore[T]{}
			}
		case UseMemoryStorage:
			if o {
				cfg.store = &MemoryStore[T]{}
			}
		case UseLazyLoading:
			if o {
				cfg.lazy = true
			}
		case UseEagerLoading:
			if o {
				cfg.lazy = false
			}
		case WithGenerator[T]:
			cfg.generate = o
		case WithValue[T]:
			val := o.val
			cfg.store.Set(&val)
		}
	}

	fc := &defaultContent[T]{
		store:    cfg.store,
		lazy:     cfg.lazy,
		generate: cfg.generate,
	}

	if !fc.lazy {
		_, _ = fc.load()
	}

	return fc
}

func (fc *defaultContent[T]) load() (*T, error) {
	if fc.generate == nil {
		return nil, nil // No generator provided, return nil or handle gracefully
	}
	val, err := fc.generate()
	if err != nil {
		return nil, err
	}

	fc.store.Set(val)
	return val, nil
}

func (fc *defaultContent[T]) Data() (*T, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if val := fc.store.Get(); val != nil {
		return val, nil
	}

	val, err := fc.load()
	if err != nil {
		return nil, err
	}

	return val, nil
}

func (fc *defaultContent[T]) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.store.Clear()
	return nil
}

func (fc *defaultContent[T]) String() string {
	val, err := fc.Data()
	if err != nil || val == nil {
		return "" // Suppress error for templates
	}

	if s, ok := any(*val).(string); ok {
		return s
	}
	if b, ok := any(*val).([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", *val)
}

func (fc *defaultContent[T]) SetGenerator(generate func() (*T, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.store.Clear()
	if !fc.lazy {
		_, _ = fc.load()
	}
}
