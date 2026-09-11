package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
)

const (
	AggregateTypePayment      = "payment"
	PaymentCreatedEvent       = "payment.created"
	PaymentStatusChangedEvent = "payment.status_changed"
	OutboxStatusPending       = "pending"
	OutboxStatusPublishing    = "publishing"
	OutboxStatusPublished     = "published"
	OutboxStatusFailed        = "failed"
	PaymentEventsTopic        = "finflow.payment.events"
	PaymentEventSchemaVersion = 1
)

type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Status        string
	Attempts      int
	LastError     string
	CreatedAt     time.Time
	PublishedAt   *time.Time
}

type PaymentCreatedPayload struct {
	SchemaVersion     int                 `json:"schema_version"`
	PaymentID         string              `json:"payment_id"`
	AmountCents       int64               `json:"amount_cents"`
	Currency          string              `json:"currency"`
	Status            model.PaymentStatus `json:"status"`
	ExternalReference string              `json:"external_reference,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
}

type PaymentStatusChangedPayload struct {
	SchemaVersion  int                 `json:"schema_version"`
	PaymentID      string              `json:"payment_id"`
	PreviousStatus model.PaymentStatus `json:"previous_status"`
	NewStatus      model.PaymentStatus `json:"new_status"`
	ChangedAt      time.Time           `json:"changed_at"`
}

func NewPaymentCreated(payment model.Payment) (OutboxEvent, error) {
	payload := PaymentCreatedPayload{
		SchemaVersion:     PaymentEventSchemaVersion,
		PaymentID:         payment.ID,
		AmountCents:       payment.AmountCents,
		Currency:          payment.Currency,
		Status:            payment.Status,
		ExternalReference: payment.ExternalReference,
		CreatedAt:         payment.CreatedAt,
	}

	return newPaymentEvent(payment.ID, PaymentCreatedEvent, payload)
}

func NewPaymentStatusChanged(previousStatus model.PaymentStatus, payment model.Payment) (OutboxEvent, error) {
	payload := PaymentStatusChangedPayload{
		SchemaVersion:  PaymentEventSchemaVersion,
		PaymentID:      payment.ID,
		PreviousStatus: previousStatus,
		NewStatus:      payment.Status,
		ChangedAt:      payment.UpdatedAt,
	}

	return newPaymentEvent(payment.ID, PaymentStatusChangedEvent, payload)
}

func newPaymentEvent(paymentID string, eventType string, payload any) (OutboxEvent, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("marshal payment event payload: %w", err)
	}

	return OutboxEvent{
		ID:            uuid.NewString(),
		AggregateType: AggregateTypePayment,
		AggregateID:   paymentID,
		EventType:     eventType,
		Payload:       body,
		Status:        OutboxStatusPending,
	}, nil
}
