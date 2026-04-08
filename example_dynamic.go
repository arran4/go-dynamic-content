//go:build ignore

package main

import (
	"fmt"

	utils "github.com/arran4/go-weak-content"
	"github.com/arran4/go-weak-content/helpers"
)

func main() {
	// 1. Create the dynamic generator helper with an initial state
	dynGen := helpers.NewDynamicGenerator(func() (*string, error) {
		val := "Hello from initial generator state!"
		return &val, nil
	})

	// 2. Pass its Generate method to NewContent
	fc := utils.NewContent(
		utils.WithGenerator(dynGen.Generate),
		utils.UseMemoryStorage[string](true), // Use memory storage so we can invalidate safely
	)

	// Fetch data. It uses the initial generator.
	data, _ := fc.Data()
	fmt.Printf("First call: %s\n", *data)

	// 3. To switch the generator, we change the state inside the helper
	dynGen.SetGenerator(func() (*string, error) {
		val := "Hello from UPDATED generator state!"
		return &val, nil
	})

	// 4. Remember to invalidate the cache so the Content object knows it needs to re-fetch
	fc.Invalidate()

	// Fetch data again. It uses the new generator.
	data, _ = fc.Data()
	fmt.Printf("Second call: %s\n", *data)
}
