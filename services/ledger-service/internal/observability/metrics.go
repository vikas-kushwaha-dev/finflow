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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry      *prometheus.Registry
	httpRequests  *prometheus.CounterVec
	httpDuration  *prometheus.HistogramVec
	dbOperations  *prometheus.CounterVec
	dbDuration    *prometheus.HistogramVec
	kafkaConsumed *prometheus.CounterVec
	kafkaDuration *prometheus.HistogramVec
	kafkaLag      *prometheus.GaugeVec
	kafkaRetries  *prometheus.CounterVec
	deadLetters   *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		registry:      registry,
		httpRequests:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_http_requests_total", Help: "HTTP requests handled."}, []string{"method", "route", "status"}),
		httpDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "finflow_http_request_duration_seconds", Help: "HTTP request duration.", Buckets: prometheus.DefBuckets}, []string{"method", "route"}),
		dbOperations:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_db_operations_total", Help: "Database operations completed."}, []string{"operation", "result"}),
		dbDuration:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "finflow_db_operation_duration_seconds", Help: "Database operation duration.", Buckets: prometheus.DefBuckets}, []string{"operation"}),
		kafkaConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_kafka_messages_consumed_total", Help: "Kafka messages processed."}, []string{"event_type", "result"}),
		kafkaDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "finflow_kafka_consume_duration_seconds", Help: "Kafka processing duration.", Buckets: prometheus.DefBuckets}, []string{"event_type"}),
		kafkaLag:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "finflow_kafka_consumer_lag_messages", Help: "Approximate message lag from the last fetched high-water mark."}, []string{"topic", "partition"}),
		kafkaRetries:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_kafka_consumer_retries_total", Help: "Kafka message processing retries."}, []string{"event_type"}),
		deadLetters:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "finflow_kafka_dead_letters_total", Help: "Kafka dead-letter publishing outcomes."}, []string{"event_type", "result"}),
	}
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), m.httpRequests, m.httpDuration, m.dbOperations, m.dbDuration, m.kafkaConsumed, m.kafkaDuration, m.kafkaLag, m.kafkaRetries, m.deadLetters)
	return m
}

func (m *Metrics) RecordKafkaRetry(eventType string) {
	if eventType == "" {
		eventType = "unknown"
	}
	m.kafkaRetries.WithLabelValues(eventType).Inc()
}

func (m *Metrics) RecordDeadLetter(eventType string, err error) {
	if eventType == "" {
		eventType = "unknown"
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	m.deadLetters.WithLabelValues(eventType, result).Inc()
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (m *Metrics) RegisterDBPool(pool *pgxpool.Pool) {
	states := map[string]func() float64{
		"acquired": func() float64 { return float64(pool.Stat().AcquiredConns()) },
		"idle":     func() float64 { return float64(pool.Stat().IdleConns()) },
		"total":    func() float64 { return float64(pool.Stat().TotalConns()) },
		"max":      func() float64 { return float64(pool.Stat().MaxConns()) },
	}
	for state, value := range states {
		m.registry.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name:        "finflow_db_pool_connections",
				Help:        "PostgreSQL connection pool size by state.",
				ConstLabels: prometheus.Labels{"state": state},
			},
			value,
		))
	}
}

func (m *Metrics) RecordKafkaConsume(eventType string, topic string, partition int, lag int64, duration time.Duration, err error) {
	if eventType == "" {
		eventType = "unknown"
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	m.kafkaConsumed.WithLabelValues(eventType, result).Inc()
	m.kafkaDuration.WithLabelValues(eventType).Observe(duration.Seconds())
	if lag < 0 {
		lag = 0
	}
	m.kafkaLag.WithLabelValues(topic, strconv.Itoa(partition)).Set(float64(lag))
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
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			metrics.httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
			metrics.httpDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
			logger.Info("ledger http request", "method", r.Method, "route", route, "status", status, "bytes", recorder.BytesWritten(), "duration_ms", time.Since(start).Milliseconds(), "request_id", middleware.GetReqID(r.Context()))
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
