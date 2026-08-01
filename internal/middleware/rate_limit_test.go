package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// Set RPS to 2 and burst to 2
	rl := NewRateLimiter(2, 2)

	// Mock next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := rl.Middleware(nextHandler)

	// Make 3 requests. The first 2 should pass (burst=2), the 3rd should fail (429).
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if i < 2 {
			if rr.Code != http.StatusOK {
				t.Errorf("Request %d should have succeeded, got %d", i, rr.Code)
			}
		} else {
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf("Request %d should have been rate limited, got %d", i, rr.Code)
			}
		}
	}

	// Wait 1 second (allows 2 more tokens to be generated at 2 RPS)
	time.Sleep(1 * time.Second)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Request after waiting should have succeeded, got %d", rr.Code)
	}
}
