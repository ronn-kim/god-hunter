package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// BackoffStrategy defines how to back off between retries
type BackoffStrategy int

const (
	// Linear backoff: delay = baseDelay * attempt
	Linear BackoffStrategy = iota
	// Exponential backoff: delay = baseDelay * (2 ^ attempt)
	Exponential
	// Fibonacci backoff: delay follows fibonacci sequence
	Fibonacci
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	Strategy       BackoffStrategy
	JitterFraction float64 // 0-1: add randomness to delay
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		BaseDelay:      100 * time.Millisecond,
		MaxDelay:       30 * time.Second,
		Strategy:       Exponential,
		JitterFraction: 0.1,
	}
}

// Do retries a function with exponential backoff
func Do(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retries cancelled: %w", ctx.Err())
		default:
		}

		// Try the operation
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt < cfg.MaxAttempts-1 {
			delay := calculateBackoff(cfg, attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return fmt.Errorf("retries cancelled: %w", ctx.Err())
			}
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func calculateBackoff(cfg RetryConfig, attempt int) time.Duration {
	var delay time.Duration

	switch cfg.Strategy {
	case Linear:
		delay = cfg.BaseDelay * time.Duration(attempt+1)
	case Exponential:
		delay = cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt)))
	case Fibonacci:
		delay = cfg.BaseDelay * time.Duration(fibonacci(attempt+1))
	default:
		delay = cfg.BaseDelay * time.Duration(math.Pow(2, float64(attempt)))
	}

	// Apply max delay cap
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}

	// Apply jitter
	if cfg.JitterFraction > 0 {
		jitter := time.Duration(float64(delay) * cfg.JitterFraction)
		delay = delay - jitter/2 + time.Duration(timeNow().UnixNano()%int64(jitter))
	}

	return delay
}

func fibonacci(n int) int {
	if n <= 1 {
		return 1
	}
	a, b := 1, 1
	for i := 2; i < n; i++ {
		a, b = b, a+b
	}
	return b
}

// For testing
var timeNow = time.Now

// IsRetryable determines if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Add more sophisticated error classification as needed
	// For now, most network errors are retryable
	return true
}
