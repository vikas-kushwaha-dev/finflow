package repository

import (
	"context"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
)

type LedgerRepository interface {
	EnsureAccount(ctx context.Context, name string, currency string, normalBalance model.Direction) (model.Account, error)
	CreateTransaction(ctx context.Context, entries []model.Entry) ([]model.Entry, error)
	CreateTransactionOnce(ctx context.Context, eventID string, eventType string, aggregateID string, entries []model.Entry) ([]model.Entry, bool, error)
}
