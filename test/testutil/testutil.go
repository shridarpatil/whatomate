// Package testutil provides shared test utilities for the whatomate project.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/zerodha/logf"
)

// TestContext returns a context with a timeout suitable for tests.
// The context is automatically cancelled when the test completes.
func TestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestContextWithTimeout returns a context with a custom timeout.
func TestContextWithTimeout(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// NewTestUUID generates a deterministic UUID for testing based on a seed string.
// This is useful for creating reproducible test data.
func NewTestUUID(seed string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed))
}

// RandomUUID generates a new random UUID for testing.
func RandomUUID() uuid.UUID {
	return uuid.New()
}

// MustParseUUID parses a UUID string or fails the test.
func MustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err, "failed to parse UUID: %s", s)
	return id
}

// NopLogger returns a no-op logger for tests that don't need log output.
func NopLogger() logf.Logger {
	return logf.New(logf.Opts{
		Level:           logf.ErrorLevel, // Only log errors
		EnableCaller:   false,
		EnableColor:    false,
	})
}

// TestLogger returns a logger suitable for test output.
func TestLogger() logf.Logger {
	return logf.New(logf.Opts{
		Level:         logf.DebugLevel,
		EnableCaller:  true,
		EnableColor:   false,
	})
}

// StringPtr returns a pointer to the given string.
func StringPtr(s string) *string {
	return &s
}

// IntPtr returns a pointer to the given int.
func IntPtr(i int) *int {
	return &i
}

// TimePtr returns a pointer to the given time.
func TimePtr(t time.Time) *time.Time {
	return &t
}

// UUIDPtr returns a pointer to the given UUID.
func UUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

// BoolPtr returns a pointer to the given bool.
func BoolPtr(b bool) *bool {
	return &b
}

// AssertEventually retries an assertion function until it passes or times out.
// Useful for testing async operations.
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v: %s", timeout, msg)
}
