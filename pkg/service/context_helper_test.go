package service

import (
	"context"
	"testing"
	"time"
)

func TestWithDefaultTimeout_NoExistingDeadline(t *testing.T) {
	ctx := context.Background()

	newCtx, cancel := WithDefaultTimeout(ctx)
	defer cancel()

	deadline, hasDeadline := newCtx.Deadline()
	if !hasDeadline {
		t.Error("Expected context to have a deadline, but it doesn't")
	}

	// The deadline should be approximately 30 seconds from now
	expectedDeadline := time.Now().Add(30 * time.Second)
	diff := deadline.Sub(expectedDeadline)
	if diff < -1*time.Second || diff > 1*time.Second {
		t.Errorf("Expected deadline to be ~30 seconds from now, got difference of %v", diff)
	}
}

func TestWithDefaultTimeout_ExistingDeadline(t *testing.T) {
	// Create a context with an existing deadline
	existingDeadline := time.Now().Add(10 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), existingDeadline)
	defer cancel()

	// Apply WithDefaultTimeout - should preserve existing deadline
	newCtx, newCancel := WithDefaultTimeout(ctx)
	defer newCancel()

	deadline, hasDeadline := newCtx.Deadline()
	if !hasDeadline {
		t.Error("Expected context to have a deadline, but it doesn't")
	}

	// The deadline should be the same as the original
	diff := deadline.Sub(existingDeadline)
	if diff < -100*time.Millisecond || diff > 100*time.Millisecond {
		t.Errorf("Expected deadline to be preserved, got difference of %v", diff)
	}
}

func TestWithCustomTimeout(t *testing.T) {
	ctx := context.Background()
	customTimeout := 5 * time.Second

	newCtx, cancel := WithCustomTimeout(ctx, customTimeout)
	defer cancel()

	deadline, hasDeadline := newCtx.Deadline()
	if !hasDeadline {
		t.Error("Expected context to have a deadline, but it doesn't")
	}

	// The deadline should be approximately customTimeout from now
	expectedDeadline := time.Now().Add(customTimeout)
	diff := deadline.Sub(expectedDeadline)
	if diff < -100*time.Millisecond || diff > 100*time.Millisecond {
		t.Errorf("Expected deadline to be ~%v from now, got difference of %v", customTimeout, diff)
	}
}

func TestWithDefaultTimeout_Cancellation(t *testing.T) {
	ctx := context.Background()

	newCtx, cancel := WithDefaultTimeout(ctx)

	// Verify context is not canceled initially
	select {
	case <-newCtx.Done():
		t.Error("Context should not be canceled initially")
	default:
		// Good, context is not canceled
	}

	// Cancel the context
	cancel()

	// Verify context is now canceled
	select {
	case <-newCtx.Done():
		// Good, context is canceled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be canceled after calling cancel()")
	}
}

func TestWithCustomTimeout_Cancellation(t *testing.T) {
	ctx := context.Background()

	newCtx, cancel := WithCustomTimeout(ctx, 1*time.Hour)

	// Cancel the context
	cancel()

	// Verify context is now canceled
	select {
	case <-newCtx.Done():
		// Good, context is canceled
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be canceled after calling cancel()")
	}
}

func TestWithCustomTimeout_Expiration(t *testing.T) {
	ctx := context.Background()

	// Set a very short timeout
	newCtx, cancel := WithCustomTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	// Wait for the timeout to expire
	select {
	case <-newCtx.Done():
		// Good, context timed out as expected
		if newCtx.Err() != context.DeadlineExceeded {
			t.Errorf("Expected DeadlineExceeded error, got: %v", newCtx.Err())
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}
