# go-weak-content

A Go library providing an interface to lazily or eagerly cache generated content. It utilizes Go 1.24's new `weak` package to offer a memory-efficient `WeakStore` alongside a standard `MemoryStore`. This is great for managing ephemeral data, reducing garbage collection pressure, and maintaining efficient caching.

## Installation

```bash
go get github.com/arran4/go-weak-content
```

*Note: This library requires Go 1.24 or later due to its use of the standard library's `weak` package.*

## Features

- **Generics Support:** Caches any type (`Content[T any]`) effectively.
- **Weak Pointers:** Leverage Go 1.24 `weak` pointers to automatically free cached memory when it is no longer referenced elsewhere.
- **Thread-safe Loading:** Implemented safely for concurrent reads/writes using `sync.Mutex`. Generation and validation overlap is explicitly designed for a non-blocking stale-while-revalidate visibility model.
- **Flexible Options:** Highly configurable using functional options.
- **Lazy or Eager Loading:** Control when the content generation executes.

## Usage

### Example 1: Lazy Loading with Weak Storage

This is ideal for large datasets where you want the garbage collector to free memory when the content is no longer actively used elsewhere.

```go
package main

import (
	"fmt"

	utils "github.com/arran4/go-weak-content"
)

func main() {
	// Create a new Content instance that lazily loads, and stores via weak references.
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			// This will be called on the first Data() call
			val := "Hello from go-weak-content!"
			return &val, nil
		}),
		utils.UseWeakStorage[string](true),
		utils.UseLazyLoading[string](true),
	)

	// Generate and retrieve data
	data, err := fc.Data()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println(*data)
}
```

### Example 2: Eager Loading with Memory Storage

If you need the data to be generated immediately and kept firmly in memory, you can use eager loading with memory storage (the default storage is memory storage).

```go
package main

import (
	"fmt"

	utils "github.com/arran4/go-weak-content"
)

func main() {
	// Create a new Content instance that eagerly loads, and stores via memory references.
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			// Executed immediately
			val := "Eagerly loaded data!"
			return &val, nil
		}),
		utils.UseMemoryStorage[string](true),
		utils.UseEagerLoading[string](true),
	)

	fmt.Println(fc.String()) // "Eagerly loaded data!"
}
```

### Example 3: Initializing with Static Strings or Bytes

If the content is already available, you can initialize the cache directly.

```go
package main

import (
	"fmt"

	utils "github.com/arran4/go-weak-content"
)

func main() {
	fc := utils.NewContent[string](utils.WithValue[string]("Pre-existing content"))
	fmt.Println(fc.String()) // "Pre-existing content"
}
```

## Interfaces

### `Content[T any]`
The `Content` interface represents the core of the library, providing methods to interact with cached content:
- **`Data() (*T, error)`**: Returns a non-nil pointer to the value containing the generated content, or an error. If the content hasn't been generated yet (lazy loading), it will generate it. If generation fails (even during eager loading), the error is retained for inspection via `Error()`. Subsequent `Data()` calls will retry generation, and a successful retry will clear the retained error. A generator returning `(nil, nil)` is coerced into returning `(nil, ErrNoContent)`.
- **`Close() error`**: Clears the currently cached data from the underlying store and triggers the `onClose` callback if set.
- **`String() string`**: A convenience method that returns the generated content as a string. Suppresses errors and returns an empty string if data generation fails. If the type is `string`, `[]byte`, or `fmt.Stringer`, it will natively format it.
- **`Error() error`**: Evaluates whether the content state is currently valid. Returns `ErrInvalidContent` if a configured validator fails. If the cache is empty, it returns `ErrNoContent` (potentially wrapping a retained generator error).
- **`HasContent() bool`**: Returns true if the underlying store currently holds a generated value.
- **`Invalidate() error`**: Explicitly clears the cached content (and any retained generation error) from the underlying store and triggers the `onInvalidate` callback if set. This serves as an observation barrier: any generation that has not yet committed when the `Invalidate()` call advances the epoch will be rejected and will not become observable as a successful result to callers.

### Concurrency Visibility Model

The library guarantees thread-safety and defines specific behaviors for concurrent generation, validation, and invalidation:

- **Non-Blocking Execution:** Generation and validation use a single-flight model but do not block concurrent overlapping callers.
- **Stale-While-Validation:** If `isValid()` is actively evaluating, overlapping concurrent calls to `Data()` or `Error()` are permitted to see the pre-validation cached state and will optimistically treat it as valid without blocking.
- **Stale-While-Generation:** If a generation is in flight, overlapping concurrent calls to `Data()` will immediately return the existing stale value (if available), or `ErrNoContent` if empty, without waiting for the new generation to complete.
- **Callbacks Observe Accepted State:** Callbacks like `onGenerate` and `onInvalidate` represent accepted, committed cache state transitions. A stale generation result rejected due to a concurrent `Invalidate()` call will not invoke `onGenerate`.

### Sentinel Errors
- **`ErrNoContent`**: Returned when the content cache is empty (e.g., generator not configured, returned nil, or an error occurred during generation).
- **`ErrInvalidContent`**: Returned when the currently cached content is deemed invalid by the configured validator function.

### Storage Interfaces
- **`Store[T any]`**: The interface defining how objects are stored and retrieved (`Get()`, `Set()`, `Clear()`).
- **`WeakStore[T any]`**: An implementation of `Store[T any]` utilizing Go 1.24 `weak` pointers.
- **`MemoryStore[T any]`**: A standard implementation of `Store[T any]` keeping a strong reference in memory.

## Available Options

The `NewContent[T any](opts ...Option[T])` constructor accepts the following options:

> **Note on Boolean Options and Ordering:** For storage options (`UseWeakStorage`, `UseMemoryStorage`) and loading options (`UseLazyLoading`, `UseEagerLoading`), the boolean argument is a strict policy selector, not just an enable flag. Passing `false` selects the opposite policy (e.g. `UseWeakStorage(false)` selects memory storage). Note: Before v1.0, passing `false` acted as a no-op; existing callers using `false` as 'leave defaults/previous choice unchanged' should omit the option instead, or deliberately choose their desired policy. Unchanged function signatures do not mean unchanged behaviour. Later options in the list completely overwrite earlier ones. Storage options preserve existing content (like `WithValue`).

- **`UseWeakStorage[T](use bool)`:** If `true`, uses a weak pointer (Go 1.24 `weak` package) for storage. If `false`, selects memory storage.
- **`UseMemoryStorage[T](use bool)`:** If `true`, uses a strong reference for storage, keeping the object in memory until explicitly cleared (the default). If `false`, selects weak storage.
- **`UseLazyLoading[T](use bool)`:** If `true`, delays the execution of the generator function until `Data()` or `String()` is first called (the default). If `false`, selects eager loading.
- **`UseEagerLoading[T](use bool)`:** If `true`, immediately executes the generator function during the `NewContent` call. If `false`, selects lazy loading.
- **`WithGenerator[T](func() (*T, error))`:** The function that supplies the content when needed.
- **`WithValidator[T](func() bool)`:** Sets a function that determines whether the currently cached content is still valid.
- **`WithValue[T](T)`:** Directly sets the content cache with the provided static value. Note that when used in combination with `UseWeakStorage`, the initial value only retains a weak reference internally and will legitimately become eligible for garbage collection unless a strong reference is maintained elsewhere.
- **`WithOnGenerate[T](func(val *T, err error))`:** A callback executed immediately after a generation attempt.
- **`WithOnInvalidate[T](func())`:** A callback executed when content is cleared from the store due to invalidation.
- **`WithOnClose[T](func())`:** A callback executed when `Close()` is called.

## License

This project is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for more details.
