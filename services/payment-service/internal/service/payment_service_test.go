package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
)

type fakePaymentRepository struct {
	payment model.Payment
	created bool
	err     error
}

func (r fakePaymentRepository) Create(ctx context.Context, payment model.Payment) (model.Payment, bool, error) {
	if r.err != nil {
		return model.Payment{}, false, r.err
	}

	if r.payment.ID != "" {
		return r.payment, r.created, nil
	}

	payment.CreatedAt = time.Now().UTC()
	payment.UpdatedAt = payment.CreatedAt
	return payment, r.created, nil
}

func TestPaymentServiceCreateValidPayment(t *testing.T) {
	svc := NewPaymentService(fakePaymentRepository{created: true})

	result, err := svc.Create(context.Background(), model.CreatePaymentRequest{
		AmountCents: 1299,
		Currency:    "usd",
		Description: "  Test payment  ",
	}, " key-1 ")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if !result.Created {
		t.Fatalf("Create() Created = false, want true")
	}
	if result.Payment.AmountCents != 1299 {
		t.Fatalf("AmountCents = %d, want 1299", result.Payment.AmountCents)
	}
	if result.Payment.Currency != "USD" {
		t.Fatalf("Currency = %q, want USD", result.Payment.Currency)
	}
	if result.Payment.Description != "Test payment" {
		t.Fatalf("Description = %q, want trimmed value", result.Payment.Description)
	}
	if result.Payment.IdempotencyKey != "key-1" {
		t.Fatalf("IdempotencyKey = %q, want key-1", result.Payment.IdempotencyKey)
	}
}

func TestPaymentServiceCreateRejectsInvalidPayment(t *testing.T) {
	svc := NewPaymentService(fakePaymentRepository{created: true})

	_, err := svc.Create(context.Background(), model.CreatePaymentRequest{
		AmountCents: 0,
		Currency:    "USD",
	}, "")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create() error = %v, want ErrValidation", err)
	}
}

func TestPaymentServiceCreateReturnsExistingIdempotentPayment(t *testing.T) {
	existing := model.Payment{
		ID:             "existing-payment",
		AmountCents:    1299,
		Currency:       "USD",
		Status:         model.PaymentStatusPending,
		IdempotencyKey: "key-1",
	}
	svc := NewPaymentService(fakePaymentRepository{payment: existing, created: false})

	result, err := svc.Create(context.Background(), model.CreatePaymentRequest{
		AmountCents: 1299,
		Currency:    "USD",
	}, "key-1")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if result.Created {
		t.Fatalf("Create() Created = true, want false")
	}
	if result.Payment.ID != existing.ID {
		t.Fatalf("Payment.ID = %q, want existing payment", result.Payment.ID)
	}
}
