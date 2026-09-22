package observability

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRequestLoggerExportsPrometheusMetrics(t *testing.T) {
	metrics := NewMetrics()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := chi.NewRouter()
	router.Use(RequestLogger(logger, metrics))
	router.Get("/test/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test/123", nil))

	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `finflow_http_requests_total{method="GET",route="/test/{id}",status="500"} 1`) {
		t.Fatalf("metrics output missing request counter: %s", body)
	}
	if !strings.Contains(body, "finflow_http_request_duration_seconds_bucket") {
		t.Fatalf("metrics output missing duration histogram")
	}
}

func TestKafkaMetricsUseBoundedLabels(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordKafkaPublish("payment.created", 0, nil)
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), `finflow_kafka_messages_published_total{event_type="payment.created",result="success"} 1`) {
		t.Fatalf("metrics output missing Kafka success counter")
	}
}
