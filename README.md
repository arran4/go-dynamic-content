# go-weak-content

A Go library providing an interface to lazily or eagerly cache generated content. It utilizes Go 1.24's new `weak` package to offer a memory-efficient `WeakBytesStore` alongside a standard `MemoryBytesStore`. This is great for managing ephemeral data, reducing garbage collection pressure, and maintaining efficient caching.

## Installation

```bash
go get github.com/arran4/go-weak-content
```

*Note: This library requires Go 1.24 or later due to its use of the standard library's `weak` package.*

## Features

- **Weak Pointers:** Leverage Go 1.24 `weak` pointers to automatically free cached memory when it is no longer referenced elsewhere.
- **Thread-safe Loading:** Implemented safely for concurrent reads/writes using `sync.Mutex`.
- **Flexible Options:** Highly configurable using functional options.
- **Lazy or Eager Loading:** Control when the content generation executes.

## Usage

### Example 1: Lazy Loading with Weak Storage

This is ideal for large datasets where you want the garbage collector to free memory when the content is no longer actively used elsewhere.

```go
package main

import (
	"bytes"
	"fmt"
	"io"

	utils "github.com/arran4/go-weak-content"
)

func main() {
	// Create a new Content instance that lazily loads, and stores via weak references.
	fc := utils.NewContent(
		utils.WithGenerator(func() (io.ReadCloser, error) {
			// This will be called on the first Data() call
			return io.NopCloser(bytes.NewBufferString("Hello from go-weak-content!")), nil
		}),
		utils.UseWeakStorage(true),
		utils.UseLazyLoading(true),
	)

	// Generate and retrieve data
	data, err := fc.Data()
	if err != nil {
		panic(err)
	}

	fmt.Println(string(*data))
}
```

### Example 2: Eager Loading with Memory Storage

If you need the data to be generated immediately and kept firmly in memory, you can use eager loading with memory storage (the default storage is memory storage).

```go
package main

import (
	"bytes"
	"fmt"
	"io"

	utils "github.com/arran4/go-weak-content"
)

func main() {
	fc := utils.NewContent(
		utils.WithGenerator(func() (io.ReadCloser, error) {
			// Executed immediately
			return io.NopCloser(bytes.NewBufferString("Eagerly loaded data!")), nil
		}),
		utils.UseMemoryStorage(true),
		utils.UseEagerLoading(true),
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
	fc := utils.NewContent(utils.WithString("Pre-existing content"))
	fmt.Println(fc.String()) // "Pre-existing content"
}
```

## Interfaces

### `Content`
The `Content` interface represents the core of the library, providing methods to interact with cached content:
- **`Data() (*[]byte, error)`**: Returns a pointer to the byte slice containing the generated content. If the content hasn't been generated yet (lazy loading), it will generate it.
- **`Close() error`**: Clears the currently cached data from the underlying store.
- **`SetGenerator(func() (io.ReadCloser, error))`**: Updates the generator function and clears any currently cached data. If eager loading is enabled, it will immediately generate the content.
- **`String() string`**: A convenience method that returns the generated content as a string. Suppresses errors and returns an empty string if data generation fails.

### Storage Interfaces
- **`BytesStore`**: The interface defining how bytes are stored and retrieved (`Get()`, `Set()`, `Clear()`).
- **`WeakBytesStore`**: An implementation of `BytesStore` utilizing Go 1.24 `weak` pointers.
- **`MemoryBytesStore`**: A standard implementation of `BytesStore` keeping a strong reference in memory.

## Available Options

The `NewContent(opts ...any)` constructor accepts the following options:

- **`UseWeakStorage(bool)`:** Uses a weak pointer (Go 1.24 `weak` package) for storage. The garbage collector may reclaim the cached data if it's not strongly referenced elsewhere.
- **`UseMemoryStorage(bool)`:** Uses a strong reference for storage, keeping the bytes in memory until explicitly cleared (this is the default behavior).
- **`UseLazyLoading(bool)`:** Delays the execution of the generator function until `Data()` or `String()` is first called (this is the default behavior).
- **`UseEagerLoading(bool)`:** Immediately executes the generator function during the `NewContent` call.
- **`WithGenerator(func() (io.ReadCloser, error))`:** The function that supplies the content when needed.
- **`WithBytes([]byte)`:** Directly sets the content cache with the provided byte slice.
- **`WithString(string)`:** Directly sets the content cache with the provided string.

## License

This project is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for more details.
