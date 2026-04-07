//go:build ignore

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
