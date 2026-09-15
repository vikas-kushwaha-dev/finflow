package proxy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

const internalTokenHeader = "X-Internal-Service-Token"

type errorResponse struct {
	Error string `json:"error"`
}

func New(target string, stripPrefix string, internalToken string, logger *slog.Logger) (http.Handler, error) {
	targetURL, err := url.Parse(strings.TrimRight(target, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse proxy target: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalHost := r.Host
		originalDirector(r)
		r.URL.Path = rewritePath(r.URL.Path, stripPrefix)
		r.Host = targetURL.Host
		r.Header.Set("X-Forwarded-Host", originalHost)
		r.Header.Set("X-Gateway", "finflow-gateway")
		r.Header.Del("X-API-Key")
		r.Header.Del("Authorization")
		r.Header.Set(internalTokenHeader, internalToken)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("proxy request failed", "target", targetURL.String(), "path", r.URL.Path, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(errorResponse{Error: "upstream unavailable"})
	}

	return proxy, nil
}

func rewritePath(path string, stripPrefix string) string {
	stripPrefix = strings.TrimRight(stripPrefix, "/")
	if stripPrefix == "" {
		return path
	}

	rewritten := strings.TrimPrefix(path, stripPrefix)
	if rewritten == "" {
		return "/"
	}
	if !strings.HasPrefix(rewritten, "/") {
		return "/" + rewritten
	}

	return rewritten
}
