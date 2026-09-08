package utils_test

import (
	"fmt"
	"sync/atomic"
	"testing"

	utils "github.com/arran4/go-weak-content"
	"github.com/arran4/go-weak-content/helpers"
)

func TestContent_CompositionRetryGenerator(t *testing.T) {
	var generateCalls int32

	// A generator that fails once then succeeds, testing composition with helpers.RetryGenerator
	gen := func() (*[]byte, error) {
		count := atomic.AddInt32(&generateCalls, 1)
		if count == 1 {
			return nil, fmt.Errorf("temporary error")
		}
		b := []byte("success")
		return &b, nil
	}

	retryGen := helpers.RetryGenerator[[]byte](3, 0, gen)

	fc := utils.NewContent[[]byte](
		utils.WithGenerator[[]byte](retryGen),
	)

	b, err := fc.Data()
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}

	// Safe nil check before casting
	if b == nil {
		t.Errorf("expected success but cache returned nil")
	} else if string(*b) != "success" {
		t.Errorf("expected success, got %s", string(*b))
	}

	if atomic.LoadInt32(&generateCalls) != 2 {
		t.Errorf("expected 2 calls, got %d", atomic.LoadInt32(&generateCalls))
	}
}

func TestContent_CompositionTimeExpiry(t *testing.T) {
	// Construct the helper explicitly to verify it composes state through WithValidator
	validator, _ := helpers.TimeExpiry(0)

	fc := utils.NewContent[[]byte](
		utils.WithGenerator[[]byte](func() (*[]byte, error) {
			b := []byte("expiring content")
			return &b, nil
		}),
		// Instant expiry composition validating state execution natively correctly without sleeps
		utils.WithValidator[[]byte](validator),
	)

	// Load data natively tracking valid scopes
	_, _ = fc.Data()

	err := fc.Error()
	if err == nil || err.Error() != "content is invalid" {
		t.Errorf("expected invalid content error after expiry, got %v", err)
	}
}

func TestContent_CompositionDynamicGenerator(t *testing.T) {
	dg := helpers.NewDynamicGenerator[[]byte](nil)

	// Create Content wrapping dg.Generate
	fc := utils.NewContent[[]byte](
		utils.WithGenerator[[]byte](dg.Generate),
	)

	// Set first generator
	dg.SetGenerator(func() (*[]byte, error) {
		b := []byte("first content")
		return &b, nil
	})

	b, err := fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if b == nil {
		t.Errorf("expected first content but cache returned nil")
	} else if string(*b) != "first content" {
		t.Errorf("expected 'first content', got %s", string(*b))
	}

	// Switch the underlying dynamic generator
	dg.SetGenerator(func() (*[]byte, error) {
		b := []byte("second content")
		return &b, nil
	})

	// Must explicitly invalidate to get new generation
	_ = fc.Invalidate()

	b, err = fc.Data()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if b == nil {
		t.Errorf("expected second content but cache returned nil")
	} else if string(*b) != "second content" {
		t.Errorf("expected 'second content', got %s", string(*b))
	}
}
