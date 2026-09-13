package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
)

type fakeLedgerRepository struct {
	err error
}

func (r fakeLedgerRepository) EnsureAccount(ctx context.Context, name string, currency string, normalBalance model.Direction) (model.Account, error) {
	if r.err != nil {
		return model.Account{}, r.err
	}

	return model.Account{
		ID:            name + "-" + currency,
		Name:          name,
		Currency:      currency,
		NormalBalance: normalBalance,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (r fakeLedgerRepository) CreateTransaction(ctx context.Context, entries []model.Entry) ([]model.Entry, error) {
	if r.err != nil {
		return nil, r.err
	}
	return entries, nil
}

func (r fakeLedgerRepository) CreateTransactionOnce(ctx context.Context, eventID string, eventType string, aggregateID string, entries []model.Entry) ([]model.Entry, bool, error) {
	if r.err != nil {
		return nil, false, r.err
	}
	return entries, true, nil
}

func TestRecordPaymentMovementCreatesBalancedEntries(t *testing.T) {
	svc := NewLedgerService(fakeLedgerRepository{})

	entries, err := svc.RecordPaymentMovement(context.Background(), model.PaymentMovementRequest{
		PaymentID:   "payment-1",
		AmountCents: 1299,
		Currency:    "usd",
	})
	if err != nil {
		t.Fatalf("RecordPaymentMovement() error = %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Direction != model.DirectionDebit {
		t.Fatalf("first direction = %q, want debit", entries[0].Direction)
	}
	if entries[1].Direction != model.DirectionCredit {
		t.Fatalf("second direction = %q, want credit", entries[1].Direction)
	}
	if entries[0].AmountCents != entries[1].AmountCents {
		t.Fatalf("entries are not equal amount")
	}
	if entries[0].TransactionID != entries[1].TransactionID {
		t.Fatalf("entries are not in the same transaction")
	}
}

func TestRecordPaymentMovementRejectsInvalidRequest(t *testing.T) {
	svc := NewLedgerService(fakeLedgerRepository{})

	_, err := svc.RecordPaymentMovement(context.Background(), model.PaymentMovementRequest{
		PaymentID:   "",
		AmountCents: 1299,
		Currency:    "USD",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("RecordPaymentMovement() error = %v, want ErrValidation", err)
	}
}

func TestRecordPaymentMovementOnceCreatesBalancedEntries(t *testing.T) {
	svc := NewLedgerService(fakeLedgerRepository{})

	entries, processed, err := svc.RecordPaymentMovementOnce(context.Background(), "5de6b73e-1c90-4597-84a8-2d4bf34be7f8", "payment.created", model.PaymentMovementRequest{
		PaymentID:   "payment-1",
		AmountCents: 1299,
		Currency:    "USD",
	})
	if err != nil {
		t.Fatalf("RecordPaymentMovementOnce() error = %v", err)
	}
	if !processed {
		t.Fatalf("processed = false, want true")
	}
	if err := ValidateBalanced(entries); err != nil {
		t.Fatalf("ValidateBalanced() error = %v", err)
	}
}

func TestRecordPaymentMovementOnceRejectsInvalidEventID(t *testing.T) {
	svc := NewLedgerService(fakeLedgerRepository{})

	_, _, err := svc.RecordPaymentMovementOnce(context.Background(), "not-a-uuid", "payment.created", model.PaymentMovementRequest{
		PaymentID:   "payment-1",
		AmountCents: 1299,
		Currency:    "USD",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("RecordPaymentMovementOnce() error = %v, want ErrValidation", err)
	}
}

func TestValidateBalancedAcceptsBalancedEntries(t *testing.T) {
	err := ValidateBalanced([]model.Entry{
		{Direction: model.DirectionDebit, AmountCents: 500, Currency: "USD"},
		{Direction: model.DirectionCredit, AmountCents: 500, Currency: "USD"},
	})
	if err != nil {
		t.Fatalf("ValidateBalanced() error = %v", err)
	}
}

func TestValidateBalancedRejectsUnbalancedEntries(t *testing.T) {
	err := ValidateBalanced([]model.Entry{
		{Direction: model.DirectionDebit, AmountCents: 500, Currency: "USD"},
		{Direction: model.DirectionCredit, AmountCents: 499, Currency: "USD"},
	})
	if !errors.Is(err, ErrUnbalancedTransaction) {
		t.Fatalf("ValidateBalanced() error = %v, want ErrUnbalancedTransaction", err)
	}
}

func TestValidateBalancedRejectsCrossCurrencyEntries(t *testing.T) {
	err := ValidateBalanced([]model.Entry{
		{Direction: model.DirectionDebit, AmountCents: 500, Currency: "USD"},
		{Direction: model.DirectionCredit, AmountCents: 500, Currency: "GBP"},
	})
	if !errors.Is(err, ErrUnbalancedTransaction) {
		t.Fatalf("ValidateBalanced() error = %v, want ErrUnbalancedTransaction", err)
	}
}
