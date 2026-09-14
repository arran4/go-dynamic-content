package helpers_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	utils "github.com/arran4/go-weak-content"
	"github.com/arran4/go-weak-content/helpers"
)

func ExampleDynamicGenerator() {
	// 1. Create a DynamicGenerator with an initial state
	dg := helpers.NewDynamicGenerator[string](func() (*string, error) {
		val := "initial generated content"
		return &val, nil
	})

	// 2. Pass Generate through WithGenerator
	fc := utils.NewContent[string](
		utils.WithGenerator(dg.Generate),
	)

	// 3. Populate and read the cache
	data, err := fc.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("First access:", *data)

	// 4. Call SetGenerator to change the generation logic
	dg.SetGenerator(func() (*string, error) {
		val := "newly generated content"
		return &val, nil
	})

	// 5. Show that changing the generator does not evict already-cached content.
	// DynamicGenerator is the supported way to vary generation behaviour rather than mutating Content.
	data2, err := fc.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("After SetGenerator, before Invalidate:", *data2)

	// 6. Explicitly Invalidate() when the changed generator should take effect
	if err := fc.Invalidate(); err != nil {
		fmt.Println("Invalidate error:", err)
		return
	}

	// 7. Show the next access using the new generator
	data3, err := fc.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("After Invalidate:", *data3)

	// Output:
	// First access: initial generated content
	// After SetGenerator, before Invalidate: initial generated content
	// After Invalidate: newly generated content
}

func ExampleFallbackGenerator() {
	primaryErr := errors.New("primary source failed")

	// A realistic primary/fallback composition.
	// Generators are evaluated in priority order.
	// Fallback ordering is policy, not incidental.
	fallbackGen := helpers.FallbackGenerator[string](
		func() (*string, error) {
			// Primary source fails, so its error is hidden by a later successful source.
			return nil, primaryErr
		},
		func() (*string, error) {
			// Fallback source succeeds.
			val := "fallback content"
			return &val, nil
		},
	)

	val, err := fallbackGen()
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Result:", *val)
	}

	// If all sources fail, it returns the final error.
	failingGen := helpers.FallbackGenerator[string](
		func() (*string, error) { return nil, primaryErr },
		func() (*string, error) { return nil, errors.New("fallback also failed") },
	)
	_, err2 := failingGen()
	fmt.Println("Failing result:", err2)

	// Note: Fallback is not validation of a successful-but-unusable value.
	// Current behavior treats err == nil (even if val is nil) as success.
	nilValGen := helpers.FallbackGenerator[string](
		func() (*string, error) { return nil, nil }, // succeeds with nil
		func() (*string, error) { val := "won't reach here"; return &val, nil },
	)
	val3, err3 := nilValGen()
	fmt.Printf("Nil value result: val=%v, err=%v\n", val3, err3)

	// Output:
	// Result: fallback content
	// Failing result: fallback also failed
	// Nil value result: val=<nil>, err=<nil>
}

func ExampleRetryGenerator() {
	var attempts int

	// Use a deterministic zero-delay example.
	// maxRetries means retries *after* the initial attempt,
	// so total attempts can be maxRetries + 1.
	retryGen := helpers.RetryGenerator[string](
		2, // maxRetries
		0, // delay
		func() (*string, error) {
			attempts++
			fmt.Println("Attempt", attempts)
			return nil, errors.New("transient error")
		},
	)

	// Retrying multiplies side effects and cost. It's appropriate for transient failures,
	// but long blocking delays can be a poor fit for latency-sensitive paths and eager generation.
	fc := utils.NewContent[string](
		utils.WithGenerator(retryGen), // It composes through WithGenerator
	)

	_, err := fc.Data()
	fmt.Println("Final error:", err)

	// Output:
	// Attempt 1
	// Attempt 2
	// Attempt 3
	// Final error: no content available: transient error
}

func ExampleTimeExpiry() {
	// Obtain the validator and reset closure
	validator, reset := helpers.TimeExpiry(50 * time.Millisecond)

	// Pass the validator to WithValidator
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "generated"
			return &val, nil
		}),
		utils.WithValidator[string](validator),
	)

	// First access populates the cache
	data, err := fc.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("First access:", *data)

	// Reset establishes a new validity window.
	// Expiry acts as a cache-validity policy rather than a background scheduler.
	// Demand-driven lifecycle plus current concurrent reset/validation guarantees apply.
	reset()

	// Since we just reset it, it is still valid
	if fc.HasContent() && fc.Error() == nil {
		fmt.Println("Cache is still valid right after reset")
	}

	// Make the example deterministic by not relying on fragile sleeps or wall-clock timing for output.
	// (We don't wait for actual expiry to print, just demonstrate the mechanics).

	// Output:
	// First access: generated
	// Cache is still valid right after reset
}

func ExampleFileModified() {
	// Setup a temporary file with explicitly controlled timestamps to ensure determinism.
	dir, err := os.MkdirTemp("", "example_file_modified")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	filepath := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(filepath, []byte("v1"), 0644); err != nil {
		panic(err)
	}

	t1 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath, t1, t1); err != nil {
		panic(err)
	}

	// Obtain the validator and reset closure
	validator, reset := helpers.FileModified(filepath)

	// Show/reset the baseline at the correct point so the example does not
	// continually regard freshly generated content as stale.
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			b, err := os.ReadFile(filepath)
			if err != nil {
				return nil, err
			}
			val := string(b)
			// Reset baseline after reading to ensure freshness
			reset()
			return &val, nil
		}),
		utils.WithValidator[string](validator),
	)

	data1, err := fc.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("First read:", *data1)

	// Modify the file externally and update its modification time
	if err := os.WriteFile(filepath, []byte("v2"), 0644); err != nil {
		panic(err)
	}
	t2 := t1.Add(1 * time.Second)
	if err := os.Chtimes(filepath, t2, t2); err != nil {
		panic(err)
	}

	// Subsequent content access observes invalidity and regenerates.
	// Filesystem metadata validation is demand-driven, not background watching.
	// Concurrent reset/validation guarantees are provided by the helper.
	data2, err := fc.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Second read (after external modification):", *data2)

	// Note: missing/unstatable files cause the validator to report invalid.

	// Output:
	// First read: v1
	// Second read (after external modification): v2
}

// Example_lazyFileCache demonstrates a lazy memory-backed file cache using FileModified.
func Example_lazyFileCache() {
	dir, err := os.MkdirTemp("", "example_lazy_file_cache")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "lazy_config.json")
	if err := os.WriteFile(path, []byte(`{"status": "ok"}`), 0644); err != nil {
		panic(err)
	}

	// Storage, generation, validation, and helper responsibility are visibly separate.
	validator, reset := helpers.FileModified(path)

	configCache := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			val := string(b)
			// Reset validity baseline exactly when we generate the new content
			reset()
			return &val, nil
		}),
		utils.WithValidator[string](validator),
		utils.UseMemoryStorage[string](true),
		utils.UseLazyLoading[string](true),
	)

	data, err := configCache.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Loaded config:", *data)

	// Output:
	// Loaded config: {"status": "ok"}
}

// Example_fallbackRetry demonstrates retrying a primary source and falling back to another.
func Example_fallbackRetry() {
	attempts := 0
	primarySource := func() (*string, error) {
		attempts++
		return nil, fmt.Errorf("primary error on attempt %d", attempts)
	}

	fallbackSource := func() (*string, error) {
		val := "fallback data"
		return &val, nil
	}

	// Retry the primary source. If it still fails, fallback to the secondary source.
	combinedGen := helpers.FallbackGenerator[string](
		helpers.RetryGenerator[string](1, 0, primarySource), // 1 retry = 2 attempts total
		fallbackSource,
	)

	cache := utils.NewContent[string](
		utils.WithGenerator(combinedGen),
	)

	data, err := cache.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Loaded data:", *data)
	fmt.Println("Primary attempts:", attempts)

	// Output:
	// Loaded data: fallback data
	// Primary attempts: 2
}

// Example_dynamicInvalidation demonstrates using DynamicGenerator with explicit invalidation.
func Example_dynamicInvalidation() {
	dg := helpers.NewDynamicGenerator[string](func() (*string, error) {
		val := "default user profile"
		return &val, nil
	})

	userCache := utils.NewContent[string](
		utils.WithGenerator(dg.Generate),
	)

	profile, err := userCache.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Profile:", *profile)

	// Switch to a new profile source dynamically
	dg.SetGenerator(func() (*string, error) {
		val := "premium user profile"
		return &val, nil
	})

	// Must invalidate explicitly so the cache fetches from the new generator
	if err := userCache.Invalidate(); err != nil {
		fmt.Println("Invalidate error:", err)
		return
	}

	newProfile, err := userCache.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("New Profile:", *newProfile)

	// Output:
	// Profile: default user profile
	// New Profile: premium user profile
}
