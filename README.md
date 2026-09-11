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
- **Thread-safe Loading:** Implemented safely for concurrent reads/writes using `sync.Mutex`.
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
- **`Invalidate() error`**: Explicitly clears the cached content (and any retained generation error) from the underlying store and triggers the `onInvalidate` callback if set.

### Sentinel Errors
- **`ErrNoContent`**: Returned when the content cache is empty (e.g., generator not configured, returned nil, or an error occurred during generation).
- **`ErrInvalidContent`**: Returned when the currently cached content is deemed invalid by the configured validator function.

### Storage Interfaces
- **`Store[T any]`**: The interface defining how objects are stored and retrieved (`Get()`, `Set()`, `Clear()`).
- **`WeakStore[T any]`**: An implementation of `Store[T any]` utilizing Go 1.24 `weak` pointers.
- **`MemoryStore[T any]`**: A standard implementation of `Store[T any]` keeping a strong reference in memory.

## Available Options

The `NewContent[T any](opts ...Option[T])` constructor accepts the following options:

> **Note on Option Ordering:** Storage options (`UseWeakStorage` and `UseMemoryStorage`) preserve existing content. If multiple storage options are provided, the last one provided wins. `WithValue` will have its value retained regardless of whether it appears before or after storage configuration options.

- **`UseWeakStorage[T](bool)`:** Uses a weak pointer (Go 1.24 `weak` package) for storage. The garbage collector may reclaim the cached data if it's not strongly referenced elsewhere.
- **`UseMemoryStorage[T](bool)`:** Uses a strong reference for storage, keeping the object in memory until explicitly cleared (this is the default behavior).
- **`UseLazyLoading[T](bool)`:** Delays the execution of the generator function until `Data()` or `String()` is first called (this is the default behavior).
- **`UseEagerLoading[T](bool)`:** Immediately executes the generator function during the `NewContent` call.
- **`WithGenerator[T](func() (*T, error))`:** The function that supplies the content when needed.
- **`WithValidator[T](func() bool)`:** Sets a function that determines whether the currently cached content is still valid.
- **`WithValue[T](T)`:** Directly sets the content cache with the provided static value. Note that when used in combination with `UseWeakStorage`, the initial value only retains a weak reference internally and will legitimately become eligible for garbage collection unless a strong reference is maintained elsewhere.
- **`WithOnGenerate[T](func(val *T, err error))`:** A callback executed immediately after a generation attempt.
- **`WithOnInvalidate[T](func())`:** A callback executed when content is cleared from the store due to invalidation.
- **`WithOnClose[T](func())`:** A callback executed when `Close()` is called.

## License

This project is licensed under the BSD 3-Clause License. See [LICENSE](LICENSE) for more details.

## Actions Diagnostics

If the CI/CD workflow stops creating runs for PRs or `main` pushes, future maintainers can follow this operational procedure:

1. **Is the workflow registered?**
   ```bash
   gh workflow view ci.yml --repo arran4/go-dynamic-content
   ```
2. **Is its state active? (Detecting disabled_inactivity)**
   GitHub automatically disables scheduled workflows in public repositories after 60 days without repository activity. While this is a plausible historical hypothesis for past incidents where scheduled runs ceased, it has not been definitively proven for PR/push events on this repository.
   Check the exact current workflow state:
   ```bash
   gh api repos/arran4/go-dynamic-content/actions/workflows/257208205 | jq '.state'
   ```
   If it returns `"disabled_inactivity"`, the workflow must be explicitly re-enabled (e.g. `gh workflow enable ci.yml --repo arran4/go-dynamic-content` or via UI/REST endpoint).

3. **Can `workflow_dispatch` create a run?**
   Once the workflow is confirmed active (or explicitly re-enabled), running a manual dispatch tests core functionality independently of event routing:
   ```bash
   gh workflow run ci.yml --repo arran4/go-dynamic-content --ref main -f mode=build
   ```

4. **Can a PR create a run? / Can a push to main create a run?**
   Perform a safe commit (like adding this documentation) to a branch, push it, and create a PR. If it runs, the PR trigger is functional. After merging, verify a main push run is created.

5. **How do I distinguish an absent run from a failed run?**
   Note: The GitHub CLI hides disabled workflows by default. You must use `--all` to see full history when investigating disabled states:
   ```bash
   gh run list --repo arran4/go-dynamic-content --workflow ci.yml --limit 10 --all
   ```
   If a PR or push was made but no new run appears here, the run is **absent** (event dropped or workflow disabled). If it appears with a status of `failed`, it is a **failed run**.

6. **Which GitHub settings should be checked when no run is created?**
   Check for Actions policy restrictions:
   ```bash
   gh api repos/arran4/go-dynamic-content/actions/permissions
   ```
   Check the repository and organizational Actions policies to verify that the specific actions used by this workflow are permitted. Ensure any adjustments preserve the existing release-safe/security posture rather than recommending a blanket policy expansion.
