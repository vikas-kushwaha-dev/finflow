package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/repository"
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

func (r fakePaymentRepository) GetByID(ctx context.Context, id string) (model.Payment, error) {
	if r.err != nil {
		return model.Payment{}, r.err
	}

	if r.payment.ID == "" {
		return model.Payment{}, repository.ErrPaymentNotFound
	}

	return r.payment, nil
}

func (r fakePaymentRepository) UpdateStatus(ctx context.Context, id string, status model.PaymentStatus) (model.Payment, error) {
	if r.err != nil {
		return model.Payment{}, r.err
	}

	if r.payment.ID == "" {
		return model.Payment{}, repository.ErrPaymentNotFound
	}

	r.payment.Status = status
	r.payment.UpdatedAt = time.Now().UTC()
	return r.payment, nil
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

func TestPaymentServiceGetByIDReturnsPayment(t *testing.T) {
	expected := model.Payment{
		ID:          "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
		AmountCents: 1299,
		Currency:    "USD",
		Status:      model.PaymentStatusPending,
	}
	svc := NewPaymentService(fakePaymentRepository{payment: expected})

	payment, err := svc.GetByID(context.Background(), " 5de6b73e-1c90-4597-84a8-2d4bf34be7f8 ")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if payment.ID != expected.ID {
		t.Fatalf("Payment.ID = %q, want %q", payment.ID, expected.ID)
	}
}

func TestPaymentServiceGetByIDRejectsInvalidID(t *testing.T) {
	svc := NewPaymentService(fakePaymentRepository{})

	_, err := svc.GetByID(context.Background(), "not-a-uuid")
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("GetByID() error = %v, want ErrValidation", err)
	}
}

func TestPaymentServiceGetByIDReturnsNotFound(t *testing.T) {
	svc := NewPaymentService(fakePaymentRepository{
		err: fmt.Errorf("wrapped: %w", repository.ErrPaymentNotFound),
	})

	_, err := svc.GetByID(context.Background(), "5de6b73e-1c90-4597-84a8-2d4bf34be7f8")
	if !errors.Is(err, ErrPaymentNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrPaymentNotFound", err)
	}
}

func TestPaymentServiceUpdateStatusMovesPendingToSucceeded(t *testing.T) {
	existing := model.Payment{
		ID:          "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
		AmountCents: 1299,
		Currency:    "USD",
		Status:      model.PaymentStatusPending,
	}
	svc := NewPaymentService(fakePaymentRepository{payment: existing})

	payment, err := svc.UpdateStatus(context.Background(), existing.ID, model.UpdatePaymentStatusRequest{
		Status: " SUCCEEDED ",
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if payment.Status != model.PaymentStatusSucceeded {
		t.Fatalf("Status = %q, want %q", payment.Status, model.PaymentStatusSucceeded)
	}
}

func TestPaymentServiceUpdateStatusMovesPendingToFailed(t *testing.T) {
	existing := model.Payment{
		ID:          "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
		AmountCents: 1299,
		Currency:    "USD",
		Status:      model.PaymentStatusPending,
	}
	svc := NewPaymentService(fakePaymentRepository{payment: existing})

	payment, err := svc.UpdateStatus(context.Background(), existing.ID, model.UpdatePaymentStatusRequest{
		Status: model.PaymentStatusFailed,
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if payment.Status != model.PaymentStatusFailed {
		t.Fatalf("Status = %q, want %q", payment.Status, model.PaymentStatusFailed)
	}
}

func TestPaymentServiceUpdateStatusAllowsSameStatusReplay(t *testing.T) {
	existing := model.Payment{
		ID:          "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
		AmountCents: 1299,
		Currency:    "USD",
		Status:      model.PaymentStatusSucceeded,
	}
	svc := NewPaymentService(fakePaymentRepository{payment: existing})

	payment, err := svc.UpdateStatus(context.Background(), existing.ID, model.UpdatePaymentStatusRequest{
		Status: model.PaymentStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if payment.Status != model.PaymentStatusSucceeded {
		t.Fatalf("Status = %q, want %q", payment.Status, model.PaymentStatusSucceeded)
	}
}

func TestPaymentServiceUpdateStatusRejectsTerminalChange(t *testing.T) {
	existing := model.Payment{
		ID:          "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
		AmountCents: 1299,
		Currency:    "USD",
		Status:      model.PaymentStatusSucceeded,
	}
	svc := NewPaymentService(fakePaymentRepository{payment: existing})

	_, err := svc.UpdateStatus(context.Background(), existing.ID, model.UpdatePaymentStatusRequest{
		Status: model.PaymentStatusFailed,
	})
	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("UpdateStatus() error = %v, want ErrInvalidStatusTransition", err)
	}
}

func TestPaymentServiceUpdateStatusRejectsInvalidStatus(t *testing.T) {
	svc := NewPaymentService(fakePaymentRepository{})

	_, err := svc.UpdateStatus(context.Background(), "5de6b73e-1c90-4597-84a8-2d4bf34be7f8", model.UpdatePaymentStatusRequest{
		Status: "settled",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("UpdateStatus() error = %v, want ErrValidation", err)
	}
}
