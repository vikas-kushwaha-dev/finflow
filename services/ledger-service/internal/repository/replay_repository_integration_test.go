package repository_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/database"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
)

func TestReplayAuditRepositoryIntegration(t *testing.T) {
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
	migrationSQL, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "000007_create_dead_letter_replays.up.sql"))
	if err != nil {
		t.Fatalf("read replay migration: %v", err)
	}
	if _, err := pool.Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("apply replay migration: %v", err)
	}

	eventID := uuid.NewString()
	repo := repository.NewReplayAuditRepository(pool)
	source := repository.ReplaySource{
		EventID: eventID, Operator: "integration-test", Reason: "verify replay audit",
		Topic: "finflow.payment.events", Partition: 1, Offset: 42,
	}
	auditID, err := repo.StartReplay(ctx, source)
	if err != nil {
		t.Fatalf("StartReplay() error = %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM dead_letter_replays WHERE event_id = $1", eventID)
	})
	if err := repo.FinishReplay(ctx, auditID, true, ""); err != nil {
		t.Fatalf("FinishReplay() error = %v", err)
	}
	if _, err := repo.StartReplay(ctx, source); !errors.Is(err, repository.ErrReplayAlreadyClaimed) {
		t.Fatalf("second StartReplay() error = %v, want ErrReplayAlreadyClaimed", err)
	}
}
