package observability

import (
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type Metrics struct {
	startedAt       time.Time
	totalRequests   atomic.Uint64
	totalServerErrs atomic.Uint64
}

type Snapshot struct {
	UptimeSeconds     int64  `json:"uptime_seconds"`
	StartedAt         string `json:"started_at"`
	TotalRequests     uint64 `json:"total_requests"`
	TotalServerErrors uint64 `json:"total_server_errors"`
}

func NewMetrics(now time.Time) *Metrics {
	return &Metrics{startedAt: now.UTC()}
}

func (m *Metrics) Snapshot(now time.Time) Snapshot {
	return Snapshot{
		UptimeSeconds:     int64(now.UTC().Sub(m.startedAt).Seconds()),
		StartedAt:         m.startedAt.Format(time.RFC3339),
		TotalRequests:     m.totalRequests.Load(),
		TotalServerErrors: m.totalServerErrs.Load(),
	}
}

func RequestLogger(logger *slog.Logger, metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(recorder, r)

			status := recorder.Status()
			if status == 0 {
				status = http.StatusOK
			}

			metrics.totalRequests.Add(1)
			if status >= http.StatusInternalServerError {
				metrics.totalServerErrs.Add(1)
			}

			logger.Info("gateway request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", recorder.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			)
		})
	}
}
