package utils

import (
	"io"
	"sync"
	"weak"
)

type Content interface {
	Data() (*[]byte, error)
	Close() error
	SetGenerator(func() (io.ReadCloser, error))
	String() string
	IsValid() bool
	SetValidator(func() bool)
	Invalidate()
	SetOnInvalidated(func())
	SetOnRefetched(func())
	SetOnClosed(func())
}

type BytesStore interface {
	Get() *[]byte
	Set(*[]byte)
	Clear()
}

type WeakBytesStore struct {
	ptr weak.Pointer[[]byte]
}

func (s *WeakBytesStore) Get() *[]byte {
	return s.ptr.Value()
}

func (s *WeakBytesStore) Set(val *[]byte) {
	if val == nil {
		s.ptr = weak.Pointer[[]byte]{}
	} else {
		s.ptr = weak.Make(val)
	}
}

func (s *WeakBytesStore) Clear() {
	s.ptr = weak.Pointer[[]byte]{}
}

type MemoryBytesStore struct {
	val *[]byte
}

func (s *MemoryBytesStore) Get() *[]byte {
	return s.val
}

func (s *MemoryBytesStore) Set(val *[]byte) {
	s.val = val
}

func (s *MemoryBytesStore) Clear() {
	s.val = nil
}

type UseWeakStorage bool
type UseMemoryStorage bool
type UseLazyLoading bool
type UseEagerLoading bool

type WithGenerator func() (io.ReadCloser, error)
type WithValidator func() bool
type WithBytes []byte
type WithString string

type WithOnInvalidated func()
type WithOnRefetched func()
type WithOnClosed func()

type contentConfig struct {
	store         BytesStore
	lazy          bool
	generate      func() (io.ReadCloser, error)
	isValid       func() bool
	onInvalidated func()
	onRefetched   func()
	onClosed      func()
}

type defaultContent struct {
	mu            sync.Mutex
	store         BytesStore
	lazy          bool
	generate      func() (io.ReadCloser, error)
	isValid       func() bool
	onInvalidated func()
	onRefetched   func()
	onClosed      func()
}

func NewContent(opts ...any) Content {
	cfg := contentConfig{
		store: &MemoryBytesStore{},
		lazy:  true,
	}

	for _, opt := range opts {
		switch o := opt.(type) {
		case UseWeakStorage:
			if o {
				cfg.store = &WeakBytesStore{}
			}
		case UseMemoryStorage:
			if o {
				cfg.store = &MemoryBytesStore{}
			}
		case UseLazyLoading:
			if o {
				cfg.lazy = true
			}
		case UseEagerLoading:
			if o {
				cfg.lazy = false
			}
		case WithGenerator:
			cfg.generate = o
		case WithValidator:
			cfg.isValid = o
		case WithOnInvalidated:
			cfg.onInvalidated = o
		case WithOnRefetched:
			cfg.onRefetched = o
		case WithOnClosed:
			cfg.onClosed = o
		case WithBytes:
			b := []byte(o)
			cfg.store.Set(&b)
		case WithString:
			b := []byte(o)
			cfg.store.Set(&b)
		}
	}

	fc := &defaultContent{
		store:         cfg.store,
		lazy:          cfg.lazy,
		generate:      cfg.generate,
		isValid:       cfg.isValid,
		onInvalidated: cfg.onInvalidated,
		onRefetched:   cfg.onRefetched,
		onClosed:      cfg.onClosed,
	}

	if !fc.lazy {
		_, _ = fc.load()
	}

	return fc
}

func (fc *defaultContent) load() (*[]byte, error) {
	if fc.generate == nil {
		return nil, nil // No generator provided, return nil or handle gracefully
	}
	rc, err := fc.generate()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()

	b, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	fc.store.Set(&b)
	if fc.onRefetched != nil {
		fc.onRefetched()
	}
	return &b, nil
}

func (fc *defaultContent) Data() (*[]byte, error) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.isValid != nil && !fc.isValid() {
		wasCached := fc.store.Get() != nil
		fc.store.Clear()
		if wasCached && fc.onInvalidated != nil {
			fc.onInvalidated()
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

func (fc *defaultContent) Close() error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.store.Clear()
	if fc.onClosed != nil {
		fc.onClosed()
	}
	return nil
}

func (fc *defaultContent) String() string {
	b, err := fc.Data()
	if err != nil {
		return "" // Suppress error for templates
	}
	if b == nil {
		return ""
	}
	return string(*b)
}

func (fc *defaultContent) SetGenerator(generate func() (io.ReadCloser, error)) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.generate = generate
	wasCached := fc.store.Get() != nil
	fc.store.Clear()
	if wasCached && fc.onInvalidated != nil {
		fc.onInvalidated()
	}
	if !fc.lazy {
		_, _ = fc.load()
	}
}

func (fc *defaultContent) IsValid() bool {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if fc.isValid != nil && !fc.isValid() {
		return false
	}
	return fc.store.Get() != nil
}

func (fc *defaultContent) SetValidator(isValid func() bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.isValid = isValid
}

func (fc *defaultContent) Invalidate() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	wasCached := fc.store.Get() != nil
	fc.store.Clear()
	if wasCached && fc.onInvalidated != nil {
		fc.onInvalidated()
	}
}

func (fc *defaultContent) SetOnInvalidated(onInvalidated func()) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.onInvalidated = onInvalidated
}

func (fc *defaultContent) SetOnRefetched(onRefetched func()) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.onRefetched = onRefetched
}

func (fc *defaultContent) SetOnClosed(onClosed func()) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.onClosed = onClosed
}
