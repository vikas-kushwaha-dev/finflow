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

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/handler"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/observability"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/repository"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/service"
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

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	paymentRepository := repository.NewPostgresPaymentRepository(pool)
	paymentService := service.NewPaymentService(paymentRepository)
	paymentHandler := handler.NewPaymentHandler(paymentService)
	metrics := observability.NewMetrics(time.Now())

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
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
		handler.WriteJSON(w, http.StatusOK, metrics.Snapshot(time.Now()))
	})

	router.Route("/api/v1", func(r chi.Router) {
		paymentHandler.RegisterRoutes(r)
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("payment service listening", "addr", cfg.HTTPAddr)
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
