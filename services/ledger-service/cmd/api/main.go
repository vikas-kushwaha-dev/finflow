package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/handler"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/observability"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/security"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := observability.InitTracing(ctx, "ledger-service", logger)
	if err != nil {
		logger.Error("trace configuration failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	metrics := observability.NewMetrics()
	pool, err := database.Connect(ctx, cfg.DatabaseURL, metrics)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	ledgerRepository := repository.NewPostgresLedgerRepository(pool)
	ledgerService := service.NewLedgerService(ledgerRepository)
	ledgerHandler := handler.NewLedgerHandler(ledgerService)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(security.Headers)
	router.Use(observability.RequestLogger(logger, metrics))
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(15 * time.Second))

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"env":    cfg.AppEnv,
		})
	})

	router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		readyCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(readyCtx); err != nil {
			handler.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "not_ready",
				"error":  "database unavailable",
			})
			return
		}

		handler.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ready",
		})
	})

	router.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.Handler().ServeHTTP(w, r)
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(security.InternalServiceToken(cfg.InternalToken))
		ledgerHandler.RegisterRoutes(r)
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           otelhttp.NewHandler(router, "ledger-service.http"),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("ledger service listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
