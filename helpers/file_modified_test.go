package helpers

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileModified(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")

	// Create a file
	err := os.WriteFile(tempFile, []byte("initial"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	isValid, reset := FileModified(tempFile)

	if !isValid() {
		t.Error("expected validator to return true right after creation")
	}

	// Update the file's modification time
	// We need to ensure the modified time is actually in the future, so let's sleep a tiny bit
	time.Sleep(10 * time.Millisecond)

	now := time.Now()
	err = os.Chtimes(tempFile, now, now)
	if err != nil {
		t.Fatalf("failed to update mod time: %v", err)
	}

	if isValid() {
		t.Error("expected validator to return false after file was modified")
	}

	// Reset
	reset()

	if !isValid() {
		t.Error("expected validator to return true after reset")
	}

	// Delete file
	_ = os.Remove(tempFile)
	if isValid() {
		t.Error("expected validator to return false after file is deleted")
	}
}

func TestFileModified_Concurrent(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_concurrent.txt")

	err := os.WriteFile(tempFile, []byte("initial"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	isValid, reset := FileModified(tempFile)

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	numWorkers := 10
	iterations := 100

	// Start goroutines validating constantly
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			for j := 0; j < iterations; j++ {
				isValid()
			}
		}()
	}

	// Start goroutines resetting constantly
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			for j := 0; j < iterations; j++ {
				reset()
			}
		}()
	}

	// Start goroutines modifying file constantly
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh
			for j := 0; j < iterations; j++ {
				now := time.Now()
				_ = os.Chtimes(tempFile, now, now)
			}
		}()
	}

	close(startCh)
	wg.Wait()
}
