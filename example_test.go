package utils_test

import (
	"fmt"

	utils "github.com/arran4/go-weak-content"
	"github.com/arran4/go-weak-content/helpers"
)

func ExampleContent_lazyWeakStorage() {
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
	// Output:
	// Hello from go-weak-content!
}

func ExampleContent_eagerLoading() {
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

	fmt.Println(fc.String())
	// Output:
	// Eagerly loaded data!
}

func ExampleContent_eagerLoadingError() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			return nil, fmt.Errorf("initial generation failed")
		}),
		utils.UseEagerLoading[string](true),
	)

	// Eager loading errors are retained and returned on Data()
	data, err := fc.Data()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	if data == nil {
		fmt.Printf("Data is nil\n")
	}

	// Output:
	// Error: no content available: initial generation failed
	// Data is nil
}

func ExampleContent_invalidation() {
	count := 0
	fc := utils.NewContent[int](
		utils.WithGenerator(func() (*int, error) {
			count++
			val := count
			return &val, nil
		}),
		utils.UseMemoryStorage[int](true),
	)

	data1, err := fc.Data()
	if err == nil && data1 != nil {
		fmt.Printf("First call: %d\n", *data1)
	}

	// Explicitly invalidate to clear cache and force regeneration
	_ = fc.Invalidate()

	data2, err := fc.Data()
	if err == nil && data2 != nil {
		fmt.Printf("Second call: %d\n", *data2)
	}
	// Output:
	// First call: 1
	// Second call: 2
}

func ExampleNewContent_dynamicGenerator() {
	// 1. Create the dynamic generator helper with an initial state
	dynGen := helpers.NewDynamicGenerator(func() (*string, error) {
		val := "Hello from initial generator state!"
		return &val, nil
	})

	// 2. Pass its Generate method to NewContent
	fc := utils.NewContent[string](
		utils.WithGenerator(dynGen.Generate),
		utils.UseMemoryStorage[string](true), // Use memory storage so we can invalidate safely
	)

	// Fetch data. It uses the initial generator.
	data, err := fc.Data()
	if err == nil && data != nil {
		fmt.Printf("First call: %s\n", *data)
	}

	// 3. To switch the generator, we change the state inside the helper
	dynGen.SetGenerator(func() (*string, error) {
		val := "Hello from UPDATED generator state!"
		return &val, nil
	})

	// 4. Remember to invalidate the cache so the Content object knows it needs to re-fetch
	_ = fc.Invalidate()

	// Fetch data again. It uses the new generator.
	data, err = fc.Data()
	if err == nil && data != nil {
		fmt.Printf("Second call: %s\n", *data)
	}

	// Output:
	// First call: Hello from initial generator state!
	// Second call: Hello from UPDATED generator state!
}
