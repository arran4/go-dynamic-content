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

```go
package main

import (
	"bytes"
	"fmt"
	"io"

	utils "github.com/arran4/go-weak-content"
)

func main() {
	// Create a new FileContent instance that lazily loads, and stores via weak references.
	fc := utils.NewFileContent(
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

## Available Options

The `NewFileContent(opts ...any)` constructor accepts the following options:

- **`UseWeakStorage(bool)`:** Uses a weak pointer (Go 1.24 `weak` package) for storage. The garbage collector may reclaim the cached data if it's not strongly referenced elsewhere.
- **`UseMemoryStorage(bool)`:** Uses a strong reference for storage, keeping the bytes in memory until explicitly cleared (this is the default behavior).
- **`UseLazyLoading(bool)`:** Delays the execution of the generator function until `Data()` or `String()` is first called (this is the default behavior).
- **`UseEagerLoading(bool)`:** Immediately executes the generator function during the `NewFileContent` call.
- **`WithGenerator(func() (io.ReadCloser, error))`:** The function that supplies the content when needed.
- **`WithBytes([]byte)`:** Directly sets the content cache with the provided byte slice.
- **`WithString(string)`:** Directly sets the content cache with the provided string.

## License

This project is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for more details.
