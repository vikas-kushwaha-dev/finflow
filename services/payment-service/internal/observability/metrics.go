package observability

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry       *prometheus.Registry
	httpRequests   *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
	dbOperations   *prometheus.CounterVec
	dbDuration     *prometheus.HistogramVec
	kafkaPublished *prometheus.CounterVec
	kafkaDuration  *prometheus.HistogramVec
	outboxClaimed  prometheus.Counter
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry:       registry,
		httpRequests:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_http_requests_total", Help: "HTTP requests handled."}, []string{"method", "route", "status"}),
		httpDuration:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "finflow_http_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
		dbOperations:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_db_operations_total", Help: "Database operations completed."}, []string{"operation", "result"}),
		dbDuration:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "finflow_db_operation_duration_seconds", Help: "Database operation duration.", Buckets: prometheus.DefBuckets}, []string{"operation"}),
		kafkaPublished: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_kafka_messages_published_total", Help: "Kafka messages published."}, []string{"event_type", "result"}),
		kafkaDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "finflow_kafka_publish_duration_seconds", Help: "Kafka publish duration.", Buckets: prometheus.DefBuckets}, []string{"event_type"}),
		outboxClaimed:  prometheus.NewCounter(prometheus.CounterOpts{Name: "finflow_outbox_events_claimed_total", Help: "Outbox events claimed for publishing."}),
	}
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), m.httpRequests, m.httpDuration, m.dbOperations, m.dbDuration, m.kafkaPublished, m.kafkaDuration, m.outboxClaimed)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (m *Metrics) RecordKafkaPublish(eventType string, duration time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	m.kafkaPublished.WithLabelValues(eventType, result).Inc()
	m.kafkaDuration.WithLabelValues(eventType).Observe(duration.Seconds())
}

func (m *Metrics) AddOutboxClaimed(count int) { m.outboxClaimed.Add(float64(count)) }

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
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			metrics.httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
			metrics.httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
			logger.Info("http request", "method", r.Method, "route", route, "status", status, "bytes", recorder.BytesWritten(), "duration_ms", time.Since(start).Milliseconds(), "request_id", middleware.GetReqID(r.Context()))
		})
	}
}

type queryStart struct {
	started   time.Time
	operation string
}
type queryStartKey struct{}

func (m *Metrics) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	fields := strings.Fields(data.SQL)
	operation := "unknown"
	if len(fields) > 0 {
		operation = strings.ToLower(fields[0])
	}
	return context.WithValue(ctx, queryStartKey{}, queryStart{started: time.Now(), operation: operation})
}

func (m *Metrics) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(queryStartKey{}).(queryStart)
	if !ok {
		return
	}
	result := "success"
	if data.Err != nil {
		result = "error"
	}
	m.dbOperations.WithLabelValues(start.operation, result).Inc()
	m.dbDuration.WithLabelValues(start.operation).Observe(time.Since(start.started).Seconds())
}
