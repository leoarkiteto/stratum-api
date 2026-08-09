package middleware

import (
	"testing"
	"time"
)

// Internal test: reaches the unexported maxEntries field.
func TestRateLimiterCapacityResetBoundsMemory(t *testing.T) {
	rl := NewRateLimiter(2, time.Hour)
	rl.maxEntries = 2

	// Fill the map to capacity.
	if !rl.Allow("a") || !rl.Allow("b") {
		t.Fatal("first two keys should be allowed")
	}
	// Unknown key at capacity: windows reset, bounded memory.
	if !rl.Allow("c") {
		t.Fatal("unknown key at capacity should trigger a reset and be allowed")
	}
	if len(rl.started) != 1 {
		t.Fatalf("after reset the map must hold exactly the new key, got %d entries", len(rl.started))
	}
	// The reset gives tracked keys a fresh window: "a" is allowed again.
	if !rl.Allow("a") {
		t.Fatal("key tracked after reset should be allowed")
	}
}

func TestRateLimiterStillLimitsAfterReset(t *testing.T) {
	rl := NewRateLimiter(1, time.Hour)
	rl.maxEntries = 1

	if !rl.Allow("a") {
		t.Fatal("first call should be allowed")
	}
	// Unknown key at capacity resets; then the limit still applies per key.
	if !rl.Allow("b") {
		t.Fatal("reset should allow the new key")
	}
	if rl.Allow("b") {
		t.Fatal("second call for the same key within the window must still be rejected")
	}
}
