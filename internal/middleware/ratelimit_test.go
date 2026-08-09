package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum-backend/internal/middleware"
)

func TestRateLimiterAllowsWithinLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(2, time.Hour)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if !rl.Allow("1.2.3.4") {
		t.Fatal("second request should be allowed")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("third request should be rejected")
	}
	// A different key is unaffected.
	if !rl.Allow("5.6.7.8") {
		t.Fatal("a different client should not be limited")
	}
}

func TestRateLimiterResetsAfterWindow(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 30*time.Millisecond)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("second request within the window should be rejected")
	}
	time.Sleep(40 * time.Millisecond)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("request after the window elapsed should be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := middleware.NewRateLimiter(2, time.Hour)
	h := middleware.RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i, want := range []int{http.StatusNoContent, http.StatusNoContent, http.StatusTooManyRequests} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != want {
			t.Errorf("request %d: status = %d, want %d", i+1, rr.Code, want)
		}
	}

	// Envelope shape check on the 429.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if got := rr.Body.String(); got == "" {
		t.Fatal("expected an error envelope on 429")
	}
}
