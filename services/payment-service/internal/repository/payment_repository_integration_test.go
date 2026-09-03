package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/migration"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
)

func TestPostgresPaymentRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set INTEGRATION_DATABASE_URL to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := migration.Run(ctx, pool, ""); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	repo := NewPostgresPaymentRepository(pool)
	key := "integration-" + uuid.NewString()
	reference := "integration-test-" + uuid.NewString()

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM payments WHERE idempotency_key = $1 OR external_reference = $2", key, reference)
	})

	input := model.Payment{
		ID:                uuid.NewString(),
		AmountCents:       1299,
		Currency:          "USD",
		Status:            model.PaymentStatusPending,
		Description:       "Integration test payment",
		ExternalReference: reference,
		IdempotencyKey:    key,
	}

	created, wasCreated, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !wasCreated {
		t.Fatalf("Create() wasCreated = false, want true")
	}

	fetched, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if fetched.ID != created.ID {
		t.Fatalf("GetByID() ID = %q, want %q", fetched.ID, created.ID)
	}

	replayed, wasCreated, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("idempotent Create() error = %v", err)
	}
	if wasCreated {
		t.Fatalf("idempotent Create() wasCreated = true, want false")
	}
	if replayed.ID != created.ID {
		t.Fatalf("idempotent Create() ID = %q, want %q", replayed.ID, created.ID)
	}

	_, err = repo.GetByID(ctx, uuid.NewString())
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrPaymentNotFound", err)
	}
}
