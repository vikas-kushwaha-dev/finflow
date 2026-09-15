package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRewritePathRemovesGatewayPrefix(t *testing.T) {
	got := rewritePath("/api/v1/payments/123", "/api/v1")
	if got != "/payments/123" {
		t.Fatalf("expected /payments/123, got %s", got)
	}
}

func TestRewritePathKeepsRootWhenPrefixIsFullPath(t *testing.T) {
	got := rewritePath("/api/v1", "/api/v1")
	if got != "/" {
		t.Fatalf("expected /, got %s", got)
	}
}

func TestProxyAddsInternalTokenAndRemovesClientCredentials(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(internalTokenHeader); got != "internal-secret" {
			t.Fatalf("internal token = %q, want internal-secret", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("X-API-Key forwarded = %q, want empty", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization forwarded = %q, want empty", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	handler, err := New(upstream.URL, "/api/v1", "internal-secret", slog.Default())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/payments", nil)
	request.Header.Set("X-API-Key", "client-secret")
	request.Header.Set("Authorization", "Bearer client-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
