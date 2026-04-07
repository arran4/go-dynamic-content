package helpers

import (
	"os"
	"path/filepath"
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
