package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterRejectsRequestsOverLimit(t *testing.T) {
	limiter := New(2, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 2; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("request %d expected %d, got %d", i+1, http.StatusNoContent, response.Code)
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected %d, got %d", http.StatusTooManyRequests, response.Code)
	}
}

func TestLimiterResetsAfterWindow(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	limiter := New(1, time.Minute)
	limiter.now = func() time.Time { return now }

	if !limiter.allow("client") {
		t.Fatal("expected first request to be allowed")
	}
	if limiter.allow("client") {
		t.Fatal("expected second request to be rejected")
	}

	now = now.Add(time.Minute)
	if !limiter.allow("client") {
		t.Fatal("expected request after window to be allowed")
	}
}
