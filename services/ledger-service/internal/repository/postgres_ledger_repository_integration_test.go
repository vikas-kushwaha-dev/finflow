package repository_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/service"
)

func TestPostgresLedgerRepositoryIntegration(t *testing.T) {
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

	migrationSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "000003_create_ledger.up.sql"))
	if err != nil {
		t.Fatalf("read ledger migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("apply ledger migration: %v", err)
	}

	repo := repository.NewPostgresLedgerRepository(pool)
	ledgerService := service.NewLedgerService(repo)

	entries, err := ledgerService.RecordPaymentMovement(ctx, model.PaymentMovementRequest{
		PaymentID:   "integration-payment-ledger",
		AmountCents: 1299,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatalf("RecordPaymentMovement() error = %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM ledger_entries WHERE transaction_id = $1", entries[0].TransactionID)
	})

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].TransactionID != entries[1].TransactionID {
		t.Fatalf("entries do not share transaction id")
	}
	if err := service.ValidateBalanced(entries); err != nil {
		t.Fatalf("ValidateBalanced() error = %v", err)
	}
}
