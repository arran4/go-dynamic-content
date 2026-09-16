package helpers

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileModified(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")

	// 1. Create initial file
	err := os.WriteFile(tempFile, []byte("initial"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Explicitly set an initial modification time to avoid arbitrary system timestamps
	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	err = os.Chtimes(tempFile, baseTime, baseTime)
	if err != nil {
		t.Fatalf("failed to set initial mod time: %v", err)
	}

	isValid, reset := FileModified(tempFile)

	// 1. Unchanged file remains valid
	if !isValid() {
		t.Error("expected validator to return true for unchanged file")
	}

	// 2. Newer mtime is invalid
	newerTime := baseTime.Add(1 * time.Hour)
	err = os.Chtimes(tempFile, newerTime, newerTime)
	if err != nil {
		t.Fatalf("failed to update mod time: %v", err)
	}
	if isValid() {
		t.Error("expected validator to return false after file mtime was made newer")
	}

	// 3. Older mtime is invalid
	olderTime := baseTime.Add(-1 * time.Hour)
	err = os.Chtimes(tempFile, olderTime, olderTime)
	if err != nil {
		t.Fatalf("failed to update mod time: %v", err)
	}
	if isValid() {
		t.Error("expected validator to return false after file mtime was made older")
	}

	// Reset to new baseline
	err = os.Chtimes(tempFile, baseTime, baseTime)
	if err != nil {
		t.Fatalf("failed to reset mod time: %v", err)
	}
	reset()
	if !isValid() {
		t.Error("expected validator to return true after reset")
	}

	// 4. Size change is invalid even when mtime is restored to the baseline
	err = os.WriteFile(tempFile, []byte("initial_changed_size"), 0644)
	if err != nil {
		t.Fatalf("failed to change file size: %v", err)
	}
	err = os.Chtimes(tempFile, baseTime, baseTime)
	if err != nil {
		t.Fatalf("failed to restore mod time: %v", err)
	}
	if isValid() {
		t.Error("expected validator to return false after file size changed")
	}

	// Reset to new baseline with changed size
	reset()
	if !isValid() {
		t.Error("expected validator to return true after reset")
	}

	// 5. Replacing the path with a different file is invalid even when replacement has same size and mtime
	// Create another file with the exact same size ("initial_changed_size" is 20 bytes) and mtime
	replacementFile := filepath.Join(tempDir, "replacement.txt")
	err = os.WriteFile(replacementFile, []byte("replacement_file_123"), 0644)
	if err != nil {
		t.Fatalf("failed to create replacement file: %v", err)
	}
	err = os.Chtimes(replacementFile, baseTime, baseTime)
	if err != nil {
		t.Fatalf("failed to set replacement file mod time: %v", err)
	}

	// Replace tempFile with replacementFile using os.Rename
	err = os.Rename(replacementFile, tempFile)
	if err != nil {
		t.Fatalf("failed to replace file: %v", err)
	}

	if isValid() {
		t.Error("expected validator to return false after file identity changed (replacement)")
	}

	// 6. reset() after a detected change establishes the current state as valid again
	reset()
	if !isValid() {
		t.Error("expected validator to return true after reset on replacement file")
	}

	// 7. A missing/unstatable file is invalid
	_ = os.Remove(tempFile)
	if isValid() {
		t.Error("expected validator to return false after file is deleted")
	}

	// Resetting on a missing file makes the missing state the baseline...
	// but according to the contract, missing files remain invalid.
	reset()
	if isValid() {
		t.Error("expected validator to return false for a missing file even after reset")
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
