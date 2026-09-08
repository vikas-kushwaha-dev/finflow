package repository

import (
	"context"
	"errors"
	"os"
	"sync"
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
		IdempotencyHash:   "same-request-hash",
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

	conflictingInput := input
	conflictingInput.ID = uuid.NewString()
	conflictingInput.AmountCents = 9999
	conflictingInput.IdempotencyHash = "different-request-hash"

	_, _, err = repo.Create(ctx, conflictingInput)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting Create() error = %v, want ErrIdempotencyConflict", err)
	}

	concurrentKey := "integration-concurrent-" + uuid.NewString()
	concurrentReference := "integration-concurrent-" + uuid.NewString()
	concurrentInput := model.Payment{
		ID:                uuid.NewString(),
		AmountCents:       2500,
		Currency:          "USD",
		Status:            model.PaymentStatusPending,
		ExternalReference: concurrentReference,
		IdempotencyKey:    concurrentKey,
		IdempotencyHash:   "concurrent-request-hash",
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM payments WHERE idempotency_key = $1 OR external_reference = $2", concurrentKey, concurrentReference)
	})

	var wg sync.WaitGroup
	results := make(chan model.Payment, 2)
	errorsCh := make(chan error, 2)
	createdCh := make(chan bool, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			payment := concurrentInput
			payment.ID = uuid.NewString()
			result, wasCreated, err := repo.Create(context.Background(), payment)
			if err != nil {
				errorsCh <- err
				return
			}
			results <- result
			createdCh <- wasCreated
		}()
	}
	wg.Wait()
	close(results)
	close(errorsCh)
	close(createdCh)

	for err := range errorsCh {
		t.Fatalf("concurrent Create() error = %v", err)
	}

	createdCount := 0
	var firstID string
	for wasCreated := range createdCh {
		if wasCreated {
			createdCount++
		}
	}
	for result := range results {
		if firstID == "" {
			firstID = result.ID
			continue
		}
		if result.ID != firstID {
			t.Fatalf("concurrent Create() returned different IDs: %q and %q", firstID, result.ID)
		}
	}
	if createdCount != 1 {
		t.Fatalf("concurrent Create() createdCount = %d, want 1", createdCount)
	}

	updated, err := repo.UpdateStatus(ctx, created.ID, model.PaymentStatusSucceeded)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if updated.Status != model.PaymentStatusSucceeded {
		t.Fatalf("UpdateStatus() Status = %q, want %q", updated.Status, model.PaymentStatusSucceeded)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) && !updated.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("UpdateStatus() UpdatedAt = %s, want at or after %s", updated.UpdatedAt, created.UpdatedAt)
	}

	_, err = repo.GetByID(ctx, uuid.NewString())
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrPaymentNotFound", err)
	}

	_, err = repo.UpdateStatus(ctx, uuid.NewString(), model.PaymentStatusSucceeded)
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("UpdateStatus() error = %v, want ErrPaymentNotFound", err)
	}
}
