package helpers

import (
	"sync/atomic"
)

// DynamicGenerator is a helper that wraps multiple possible generation states
// and allows swapping between them dynamically. This demonstrates how to do
// state management and generator switching without relying on a SetGenerator method
// on the Content interface.
type DynamicGenerator[T any] struct {
	// We use atomic.Value to hold the current generator function thread-safely
	currentGen atomic.Value
}

// NewDynamicGenerator initializes a DynamicGenerator with a default generator.
func NewDynamicGenerator[T any](initial func() (*T, error)) *DynamicGenerator[T] {
	dg := &DynamicGenerator[T]{}
	if initial != nil {
		dg.currentGen.Store(initial)
	}
	return dg
}

// SetGenerator allows swapping out the generation logic dynamically at runtime.
func (dg *DynamicGenerator[T]) SetGenerator(next func() (*T, error)) {
	if next != nil {
		dg.currentGen.Store(next)
	}
}

// Generate is the method you pass to utils.WithGenerator. It executes whichever
// function is currently stored.
func (dg *DynamicGenerator[T]) Generate() (*T, error) {
	val := dg.currentGen.Load()
	if val == nil {
		return nil, nil
	}
	genFn := val.(func() (*T, error))
	return genFn()
}
