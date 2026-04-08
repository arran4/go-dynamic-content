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
	IsValid() bool
	SetValidator(func() bool)
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

type Option[T any] func(*contentConfig[T])

func UseWeakStorage[T any](use bool) Option[T] {
	return func(cfg *contentConfig[T]) {
		if use {
			cfg.store = &WeakStore[T]{}
		}
	}
}

func UseMemoryStorage[T any](use bool) Option[T] {
	return func(cfg *contentConfig[T]) {
		if use {
			cfg.store = &MemoryStore[T]{}
		}
	}
}

func UseLazyLoading[T any](use bool) Option[T] {
	return func(cfg *contentConfig[T]) {
		if use {
			cfg.lazy = true
		}
	}
}

func UseEagerLoading[T any](use bool) Option[T] {
	return func(cfg *contentConfig[T]) {
		if use {
			cfg.lazy = false
		}
	}
}

func WithGenerator[T any](generate func() (*T, error)) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.generate = generate
	}
}

func WithValidator[T any](isValid func() bool) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.isValid = isValid
	}
}

func WithValue[T any](val T) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.store.Set(&val)
	}
}

func WithInvalidator[T any](setup func(invalidate func() error)) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.invalidatorSetup = setup
	}
}

func WithOnGenerate[T any](cb func(val *T, err error)) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.onGenerate = cb
	}
}

func WithOnInvalidate[T any](cb func()) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.onInvalidate = cb
	}
}

func WithOnClose[T any](cb func()) Option[T] {
	return func(cfg *contentConfig[T]) {
		cfg.onClose = cb
	}
}

type contentConfig[T any] struct {
	store            Store[T]
	lazy             bool
	generate         func() (*T, error)
	isValid          func() bool
	invalidatorSetup func(invalidate func() error)
	onGenerate       func(val *T, err error)
	onInvalidate     func()
	onClose          func()
}

type defaultContent[T any] struct {
	mu           sync.Mutex
	store        Store[T]
	lazy         bool
	generate     func() (*T, error)
	isValid      func() bool
	onGenerate   func(val *T, err error)
	onInvalidate func()
	onClose      func()
}

func NewContent[T any](opts ...Option[T]) Content[T] {
	cfg := contentConfig[T]{
		store: &MemoryStore[T]{},
		lazy:  true,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	fc := &defaultContent[T]{
		store:        cfg.store,
		lazy:         cfg.lazy,
		generate:     cfg.generate,
		isValid:      cfg.isValid,
		onGenerate:   cfg.onGenerate,
		onInvalidate: cfg.onInvalidate,
		onClose:      cfg.onClose,
	}

	if cfg.invalidatorSetup != nil {
		cfg.invalidatorSetup(fc.Invalidate)
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

	if fc.onGenerate != nil {
		fc.onGenerate(val, err)
	}

	if err != nil {
		return nil, err
	}

	if val != nil {
		fc.store.Set(val)
	}
	return val, nil
}

func (fc *defaultContent[T]) Data() (*T, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.isValid != nil && !fc.isValid() {
		if fc.store.Get() != nil {
			fc.store.Clear()
			if fc.onInvalidate != nil {
				fc.onInvalidate()
			}
		}
	}

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
	if fc.onClose != nil {
		fc.onClose()
	}
	return nil
}

func (fc *defaultContent[T]) HasContent() bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.store.Get() != nil
}

func (fc *defaultContent[T]) Invalidate() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if fc.store.Get() != nil {
		fc.store.Clear()
		if fc.onInvalidate != nil {
			fc.onInvalidate()
		}
	}
	return nil
}

func (fc *defaultContent[T]) String() string {
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

func (fc *defaultContent[T]) SetGenerator(generate func() (*T, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	fc.store.Clear()
	if !fc.lazy {
		_, _ = fc.load()
	}
}

func (fc *defaultContent[T]) IsValid() bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.isValid != nil && !fc.isValid() {
		return false
	}
	return fc.store.Get() != nil
}

func (fc *defaultContent[T]) SetValidator(isValid func() bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.isValid = isValid
}
