package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalServiceTokenAllowsExpectedHeader(t *testing.T) {
	called := false
	handler := InternalServiceToken("internal-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ledger/balances", nil)
	req.Header.Set("X-Internal-Service-Token", "internal-secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatalf("handler was not called")
	}
}

func TestInternalServiceTokenRejectsMissingHeader(t *testing.T) {
	handler := InternalServiceToken("internal-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should not be called")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ledger/balances", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
