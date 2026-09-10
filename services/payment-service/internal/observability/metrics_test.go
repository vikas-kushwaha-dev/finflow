package observability

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsSnapshot(t *testing.T) {
	startedAt := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	metrics := NewMetrics(startedAt)

	snapshot := metrics.Snapshot(startedAt.Add(90 * time.Second))

	if snapshot.UptimeSeconds != 90 {
		t.Fatalf("UptimeSeconds = %d, want 90", snapshot.UptimeSeconds)
	}
	if snapshot.StartedAt != "2026-09-10T10:00:00Z" {
		t.Fatalf("StartedAt = %q, want RFC3339 start time", snapshot.StartedAt)
	}
}

func TestRequestLoggerRecordsRequestsAndServerErrors(t *testing.T) {
	metrics := NewMetrics(time.Now())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := RequestLogger(logger, metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	snapshot := metrics.Snapshot(time.Now())
	if snapshot.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1", snapshot.TotalRequests)
	}
	if snapshot.TotalServerErrors != 1 {
		t.Fatalf("TotalServerErrors = %d, want 1", snapshot.TotalServerErrors)
	}
}
