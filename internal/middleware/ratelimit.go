package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/leoarkiteto/stratum-api/internal/apierr"
	"github.com/leoarkiteto/stratum-api/internal/httpx"
)

// RateLimiter is a simple in-memory fixed-window rate limiter keyed by an
// arbitrary string (typically the client IP). It is single-instance only:
// a shared store (e.g. Redis) is required for multi-instance deployments.
// Fixed-window allows up to 2x the limit at window boundaries — acceptable
// for abuse throttling of credential endpoints.
type RateLimiter struct {
	mu         sync.Mutex
	limit      int
	window     time.Duration
	maxEntries int
	counts     map[string]int
	started    map[string]time.Time
}

// maxEntriesDefault caps the tracked keys per process to bound memory.
const maxEntriesDefault = 10_000

// NewRateLimiter builds a limiter allowing `limit` calls per `window`.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:      limit,
		window:     window,
		maxEntries: maxEntriesDefault,
		counts:     map[string]int{},
		started:    map[string]time.Time{},
	}
}

// Allow reports whether a request from key fits within the current window.
func (rl *RateLimiter) Allow(key string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, tracked := rl.started[key]; !tracked && len(rl.started) >= rl.maxEntries {
		// At capacity with an unknown key: reset all windows to bound
		// memory under a distributed-IP attack. Clients already tracked
		// (distinct keys < maxEntries per window) are never affected, and
		// each key is still limited to `limit` calls per window between
		// resets. This keeps Allow O(1) with no locked full-map sweep.
		rl.counts = make(map[string]int, 16)
		rl.started = make(map[string]time.Time, 16)
	}

	start, ok := rl.started[key]
	if !ok || now.Sub(start) >= rl.window {
		rl.started[key] = now
		rl.counts[key] = 1
		return true
	}
	rl.counts[key]++
	return rl.counts[key] <= rl.limit
}

// RateLimit throttles requests per client IP with the given limiter.
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(clientIP(r)) {
				httpx.WriteError(w, apierr.TooManyRequests("rate_limited", "too many requests, try again later"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP returns the client address from RemoteAddr. Proxy headers are
// intentionally not honored — the API is assumed to be reached directly.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
