package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRegisterDBPoolExportsCapacityMetrics(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://localhost/finflow")
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	config.MaxConns = 9
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}
	defer pool.Close()

	metrics := NewMetrics()
	metrics.RegisterDBPool(pool)
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), `finflow_db_pool_connections{state="max"} 9`) {
		t.Fatalf("metrics output missing pool capacity: %s", recorder.Body.String())
	}
}
