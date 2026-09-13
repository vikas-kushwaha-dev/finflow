package proxy

import "testing"

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
