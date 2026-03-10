package service

import (
	"context"
	"time"
)

// WithDefaultTimeout wraps a context with a default 30-second timeout if no deadline is set.
func WithDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	// If context already has a deadline, use it
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}

	// Otherwise, add a default 30-second timeout
	return context.WithTimeout(ctx, 30*time.Second)
}

// WithCustomTimeout wraps a context with a custom timeout.
func WithCustomTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}
