package ratelimit

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type Limiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	now      func() time.Time
	requests map[string]bucket
}

type bucket struct {
	count      int
	windowEnd  time.Time
	lastSeenAt time.Time
}

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:    limit,
		window:   window,
		now:      time.Now,
		requests: make(map[string]bucket),
	}
}

func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientKey(r)) {
			writeError(w, r, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	current := l.requests[key]
	if current.windowEnd.IsZero() || !now.Before(current.windowEnd) {
		current = bucket{
			count:     0,
			windowEnd: now.Add(l.window),
		}
	}

	current.count++
	current.lastSeenAt = now
	l.requests[key] = current

	return current.count <= l.limit
}

func clientKey(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return r.RemoteAddr
	}

	return host
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error:     message,
		RequestID: middleware.GetReqID(r.Context()),
	})
}
