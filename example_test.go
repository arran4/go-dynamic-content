package utils_test

import (
	"errors"
	"fmt"

	utils "github.com/arran4/go-weak-content"
	"github.com/arran4/go-weak-content/helpers"
)

// Example demonstrates the simplest sensible use of the library.
//
// - NewContent is the entry point.
// - WithValue is used for content already available.
// - WithGenerator is used for content obtained later.
// - Data() is used to access content when errors matter.
// - String() is used only as a convenience when error suppression is acceptable.
// - Memory storage is the default.
// - Lazy loading is the default.
func Example() {
	// For content already available, use WithValue.
	staticContent := utils.NewContent(utils.WithValue("static data"))
	fmt.Println("Static content:", staticContent.String())

	// For content obtained later, use WithGenerator.
	// By default, it uses memory storage and lazy loading.
	dynamicContent := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "generated data"
			return &val, nil
		}),
	)

	// Use Data() to access the content and handle any errors.
	data, err := dynamicContent.Data()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Dynamic content:", *data)

	// Output:
	// Static content: static data
	// Dynamic content: generated data
}

// Example_decisionGuide teaches users how to choose options.
//
// - Already have the value → WithValue
// - Obtain it later → WithGenerator
// - Value must stay cached → default/memory storage
// - GC may reclaim it opportunistically → weak storage
// - Generation on first access → lazy/default
// - Generation during construction → eager
// - Cached content can become stale → validator
// - Lifecycle observation → callbacks
//
// Note: Stacking contradictory policy options (like weak + memory, or lazy + eager)
// is generally unnecessary. The final enabled option deterministically wins.
func Example_decisionGuide() {
	// A complex configuration demonstrating various options.
	fc := utils.NewContent[string](
		// Obtain it later
		utils.WithGenerator(func() (*string, error) {
			val := "highly configured data"
			return &val, nil
		}),
		// Weak storage: GC may reclaim it opportunistically
		utils.UseWeakStorage[string](true),
		// Eager loading: Generation occurs during construction
		utils.UseEagerLoading[string](true),
		// Validator: Cached content can become stale
		utils.WithValidator[string](func() bool {
			return true // Always valid for this example
		}),
		// Callbacks: Lifecycle observation
		utils.WithOnGenerate[string](func(val *string, err error) {
			fmt.Println("Generation observed")
		}),
	)

	fmt.Println(fc.String())

	// Output:
	// Generation observed
	// highly configured data
}

// Example_lifecycleRecipe demonstrates the full content lifecycle.
//
// 1. construction
// 2. first access/generation
// 3. cache hit
// 4. invalidation
// 5. regeneration
// 6. close/clear
//
// Note that Close() clears state and invokes the callback but does *not*
// permanently disable a configured generator. Later access can generate again.
func Example_lifecycleRecipe() {
	generationCount := 0
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			generationCount++
			val := fmt.Sprintf("data v%d", generationCount)
			fmt.Println("Generating:", val)
			return &val, nil
		}),
		utils.WithOnInvalidate[string](func() {
			fmt.Println("Invalidated")
		}),
		utils.WithOnClose[string](func() {
			fmt.Println("Closed")
		}),
	)

	// 1. Construction (lazy by default, so no generation yet)
	fmt.Println("Constructed")

	// 2. First access (triggers generation)
	fmt.Println("--- First Access ---")
	val1, _ := fc.Data()
	fmt.Println(*val1)

	// 3. Cache hit (no generation)
	fmt.Println("--- Cache Hit ---")
	val2, _ := fc.Data()
	fmt.Println(*val2)

	// 4. Invalidation (clears cache)
	fmt.Println("--- Invalidation ---")
	_ = fc.Invalidate()

	// 5. Regeneration (triggers generation again)
	fmt.Println("--- Regeneration ---")
	val3, _ := fc.Data()
	fmt.Println(*val3)

	// 6. Close/clear (clears state, invokes callback)
	fmt.Println("--- Close ---")
	_ = fc.Close()

	// Close does not permanently disable the generator.
	// Accessing again will regenerate.
	fmt.Println("--- Post-Close Access ---")
	val4, _ := fc.Data()
	fmt.Println(*val4)

	// Output:
	// Constructed
	// --- First Access ---
	// Generating: data v1
	// data v1
	// --- Cache Hit ---
	// data v1
	// --- Invalidation ---
	// Invalidated
	// --- Regeneration ---
	// Generating: data v2
	// data v2
	// --- Close ---
	// Closed
	// --- Post-Close Access ---
	// Generating: data v3
	// data v3
}

// Example_errorHandlingRecipe teaches the distinction between different errors
// and how to handle them.
func Example_errorHandlingRecipe() {
	genErr := errors.New("underlying network error")

	// 1. ErrNoContent and generator errors
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			// A generator failure is wrapped with ErrNoContent.
			return nil, genErr
		}),
	)

	_, err := fc.Data()
	if err != nil {
		fmt.Println("Data() error:", err)
		if errors.Is(err, utils.ErrNoContent) {
			fmt.Println("It is an ErrNoContent wrapping the original error.")
		}
	}

	// 2. Error() vs String()
	fmt.Println("Error() returns:", fc.Error())
	// String() suppresses errors and returns an empty string when generation fails.
	fmt.Printf("String() returns: %q\n", fc.String())

	// 3. ErrInvalidContent
	fcInvalid := utils.NewContent[string](
		utils.WithValue("stale data"),
		utils.WithValidator[string](func() bool {
			return false // Explicitly invalid
		}),
	)

	// Error() reports ErrInvalidContent before checking cache emptiness
	fmt.Println("Invalid content Error():", fcInvalid.Error())

	// Output:
	// Data() error: no content available: underlying network error
	// It is an ErrNoContent wrapping the original error.
	// Error() returns: no content available: underlying network error
	// String() returns: ""
	// Invalid content Error(): content is invalid
}

// ExampleUseMemoryStorage demonstrates memory storage (the default).
// It strongly retains cached content, which is normally what users want
// when predictable cache lifetime matters. Explicitly specifying it is often
// redundant, but can be useful for clarity or when overriding another storage option.
func ExampleUseMemoryStorage() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "predictable data"
			return &val, nil
		}),
		utils.UseMemoryStorage[string](true),
	)

	data, _ := fc.Data()
	fmt.Println(*data)

	// Output:
	// predictable data
}

// ExampleUseWeakStorage demonstrates weak storage.
// Cached content is only weakly retained; GC may reclaim it once nothing else
// strongly references it. Cache presence must therefore not be required for correctness.
// It is appropriate for opportunistic caching. Callers should be prepared for
// regeneration/absence according to configuration. Do not use it when predictable
// retention is required.
func ExampleUseWeakStorage() {
	generationCount := 0
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			generationCount++
			val := fmt.Sprintf("weak data v%d", generationCount)
			return &val, nil
		}),
		utils.UseWeakStorage[string](true),
	)

	// Fetch data (strongly referenced locally)
	data, _ := fc.Data()
	fmt.Println("First access:", *data)

	// If data were to become unreferenced and GC ran, the cache could be emptied.
	// When accessed again, it would regenerate.

	// Output:
	// First access: weak data v1
}

// ExampleWithValue demonstrates seeding initial content.
// Memory storage retains it strongly. Final weak storage does not promise
// strong retention. Storage option ordering no longer silently discards it;
// storage selection is policy, not value mutation.
func ExampleWithValue() {
	fc := utils.NewContent(
		utils.WithValue("initial seeded data"),
		utils.UseMemoryStorage[string](true),
	)

	fmt.Println(fc.String())

	// Output:
	// initial seeded data
}

// ExampleWithGenerator demonstrates the real contract: func() (*T, error).
// Results are cached. Generation may happen again after invalidation,
// weak collection, errors, or close according to configuration.
// Generator errors can be retried on later access. (nil, nil) means no content.
// Generators with side effects must tolerate possible repeated execution.
// Use WithValue instead when the content is already available.
func ExampleWithGenerator() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "generated dynamically"
			return &val, nil
		}),
	)

	fmt.Println(fc.String())

	// Output:
	// generated dynamically
}

// ExampleUseLazyLoading demonstrates lazy loading (the default).
// Construction does not generate. First access pays the generation cost/error.
// Appropriate for optional or expensive content. Less appropriate when startup
// should validate/fail early.
func ExampleUseLazyLoading() {
	generated := false
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			generated = true
			val := "lazy data"
			return &val, nil
		}),
		utils.UseLazyLoading[string](true),
	)

	fmt.Println("After construction, generated:", generated)

	_, _ = fc.Data() // First access pays the cost

	fmt.Println("After access, generated:", generated)

	// Output:
	// After construction, generated: false
	// After access, generated: true
}

// ExampleUseEagerLoading demonstrates eager loading.
// Generation occurs during construction. Useful for pre-warming or early validation.
// Callers still need to understand how errors are represented on the returned Content.
// It should not be used merely to avoid explicitly calling Data() at an
// intentional initialization point.
func ExampleUseEagerLoading() {
	generated := false
	_ = utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			generated = true
			val := "eager data"
			return &val, nil
		}),
		utils.UseEagerLoading[string](true),
	)

	fmt.Println("After construction, generated:", generated)

	// Output:
	// After construction, generated: true
}

// ExampleWithValidator demonstrates validation.
// Validity is checked when content state is accessed.
// Invalid content is cleared/regenerated on Data(). Error() can report ErrInvalidContent.
// Validation differs from explicit invalidation. Validators should be cheap,
// deterministic, non-destructive, and safe to call repeatedly.
func ExampleWithValidator() {
	isValid := true
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "data"
			return &val, nil
		}),
		utils.WithValidator[string](func() bool {
			return isValid
		}),
	)

	_, _ = fc.Data() // Generate
	fmt.Println("HasContent when valid:", fc.HasContent())

	isValid = false // Content is now stale
	fmt.Println("Error when invalid:", fc.Error())

	// Output:
	// HasContent when valid: true
	// Error when invalid: content is invalid
}

// ExampleWithOnGenerate demonstrates observing generation outcomes.
// Recommended for things like metrics, logging, notification/integration.
// Discouraged as the sole basis of correctness.
func ExampleWithOnGenerate() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "observed data"
			return &val, nil
		}),
		utils.WithOnGenerate[string](func(val *string, err error) {
			if err != nil {
				fmt.Println("Generated error:", err)
			} else {
				fmt.Println("Generated value:", *val)
			}
		}),
	)

	_, _ = fc.Data()

	// Output:
	// Generated value: observed data
}

// ExampleWithOnInvalidate demonstrates observing actual content invalidation.
// Repeated invalidation of an already-empty cache should not imply repeated
// content-removal events. It should not be treated as guaranteed cleanup
// for every imaginable lifecycle transition.
func ExampleWithOnInvalidate() {
	fc := utils.NewContent[string](
		utils.WithValue("data to invalidate"),
		utils.WithOnInvalidate[string](func() {
			fmt.Println("Invalidation occurred")
		}),
	)

	// First invalidation triggers the callback because cache was non-empty
	_ = fc.Invalidate()

	// Subsequent invalidations on an empty cache do not trigger the callback
	_ = fc.Invalidate()

	// Output:
	// Invalidation occurred
}

// ExampleWithOnClose demonstrates closing behavior.
// Close() clears cached state and the callback runs.
// The object is not currently terminally disabled; future access may generate again.
func ExampleWithOnClose() {
	fc := utils.NewContent[string](
		utils.WithValue("data to close"),
		utils.WithOnClose[string](func() {
			fmt.Println("Close occurred")
		}),
	)

	_ = fc.Close()
	fmt.Println("HasContent after close:", fc.HasContent())

	// Output:
	// Close occurred
	// HasContent after close: false
}

// ExampleContent_Data demonstrates the Data method.
func ExampleContent_Data() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "data from Data()"
			return &val, nil
		}),
	)

	val, err := fc.Data()
	if err == nil {
		fmt.Println(*val)
	}

	// Output:
	// data from Data()
}

// ExampleContent_Error demonstrates the Error method.
func ExampleContent_Error() {
	fc := utils.NewContent[string]() // No generator, no value

	err := fc.Error()
	fmt.Println("Error:", err)

	// Output:
	// Error: no content available
}

// ExampleContent_HasContent demonstrates the HasContent method meaningfully.
func ExampleContent_HasContent() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			val := "delayed data"
			return &val, nil
		}),
	)

	fmt.Println("HasContent before Data():", fc.HasContent())
	_, _ = fc.Data()
	fmt.Println("HasContent after Data():", fc.HasContent())

	// Output:
	// HasContent before Data(): false
	// HasContent after Data(): true
}

// ExampleContent_Invalidate demonstrates invalidation followed by regeneration.
func ExampleContent_Invalidate() {
	counter := 0
	fc := utils.NewContent[int](
		utils.WithGenerator(func() (*int, error) {
			counter++
			val := counter
			return &val, nil
		}),
	)

	v1, _ := fc.Data()
	fmt.Println("First access:", *v1)

	_ = fc.Invalidate()

	v2, _ := fc.Data()
	fmt.Println("Second access:", *v2)

	// Output:
	// First access: 1
	// Second access: 2
}

// ExampleContent_Close demonstrates the Close method correctly.
func ExampleContent_Close() {
	fc := utils.NewContent(
		utils.WithValue("some data"),
	)

	fmt.Println("HasContent before Close:", fc.HasContent())
	_ = fc.Close()
	fmt.Println("HasContent after Close:", fc.HasContent())

	// Output:
	// HasContent before Close: true
	// HasContent after Close: false
}

// ExampleContent_String contrasts Data/Error with String.
func ExampleContent_String() {
	fc := utils.NewContent[string](
		utils.WithGenerator(func() (*string, error) {
			return nil, errors.New("failed to generate string")
		}),
	)

	// String() suppresses the error and returns an empty string
	fmt.Printf("String representation: %q\n", fc.String())

	// Error() correctly surfaces the wrapped error
	fmt.Println("Actual error:", fc.Error())

	// Output:
	// String representation: ""
	// Actual error: no content available: failed to generate string
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
