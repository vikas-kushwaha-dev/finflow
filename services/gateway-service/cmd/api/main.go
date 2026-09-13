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

	"github.com/vikas-kushwaha-dev/finflow/services/gateway-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/gateway-service/internal/handler"
	"github.com/vikas-kushwaha-dev/finflow/services/gateway-service/internal/observability"
	"github.com/vikas-kushwaha-dev/finflow/services/gateway-service/internal/proxy"
	"github.com/vikas-kushwaha-dev/finflow/services/gateway-service/internal/ratelimit"
	"github.com/vikas-kushwaha-dev/finflow/services/gateway-service/internal/security"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	paymentProxy, err := proxy.New(cfg.PaymentServiceURL, "/api/v1", logger)
	if err != nil {
		logger.Error("payment proxy invalid", "error", err)
		os.Exit(1)
	}

	ledgerProxy, err := proxy.New(cfg.LedgerServiceURL, "/api/v1/ledger", logger)
	if err != nil {
		logger.Error("ledger proxy invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metrics := observability.NewMetrics(time.Now())
	limiter := ratelimit.New(cfg.RateLimitRequests, cfg.RateLimitWindow)

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

	router.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		handler.WriteJSON(w, http.StatusOK, metrics.Snapshot(time.Now()))
	})

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(security.APIKey(cfg.APIKey))
		r.Use(limiter.Middleware)
		r.Use(security.MaxBodyBytes(cfg.MaxBodyBytes))
		r.Handle("/payments", paymentProxy)
		r.Handle("/payments/*", paymentProxy)
		r.Handle("/ledger", ledgerProxy)
		r.Handle("/ledger/*", ledgerProxy)
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("gateway listening", "addr", cfg.HTTPAddr)
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
