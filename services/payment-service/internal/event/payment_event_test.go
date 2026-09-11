package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
)

func TestNewPaymentCreated(t *testing.T) {
	payment := model.Payment{
		ID:                "payment-1",
		AmountCents:       1299,
		Currency:          "USD",
		Status:            model.PaymentStatusPending,
		ExternalReference: "checkout-1",
		CreatedAt:         time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
	}

	outboxEvent, err := NewPaymentCreated(payment)
	if err != nil {
		t.Fatalf("NewPaymentCreated() error = %v", err)
	}

	if outboxEvent.ID == "" {
		t.Fatalf("ID = empty, want generated id")
	}
	if outboxEvent.EventType != PaymentCreatedEvent {
		t.Fatalf("EventType = %q, want %q", outboxEvent.EventType, PaymentCreatedEvent)
	}
	if outboxEvent.Status != OutboxStatusPending {
		t.Fatalf("Status = %q, want pending", outboxEvent.Status)
	}

	var payload PaymentCreatedPayload
	if err := json.Unmarshal(outboxEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PaymentID != payment.ID {
		t.Fatalf("PaymentID = %q, want %q", payload.PaymentID, payment.ID)
	}
	if payload.SchemaVersion != PaymentEventSchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", payload.SchemaVersion, PaymentEventSchemaVersion)
	}
}

func TestNewPaymentStatusChanged(t *testing.T) {
	payment := model.Payment{
		ID:        "payment-1",
		Status:    model.PaymentStatusSucceeded,
		UpdatedAt: time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC),
	}

	outboxEvent, err := NewPaymentStatusChanged(model.PaymentStatusPending, payment)
	if err != nil {
		t.Fatalf("NewPaymentStatusChanged() error = %v", err)
	}

	if outboxEvent.EventType != PaymentStatusChangedEvent {
		t.Fatalf("EventType = %q, want %q", outboxEvent.EventType, PaymentStatusChangedEvent)
	}

	var payload PaymentStatusChangedPayload
	if err := json.Unmarshal(outboxEvent.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.PreviousStatus != model.PaymentStatusPending {
		t.Fatalf("PreviousStatus = %q, want pending", payload.PreviousStatus)
	}
	if payload.NewStatus != model.PaymentStatusSucceeded {
		t.Fatalf("NewStatus = %q, want succeeded", payload.NewStatus)
	}
}
