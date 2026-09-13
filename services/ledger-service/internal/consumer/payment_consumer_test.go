package consumer

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
)

type fakeLedger struct {
	processed bool
	err       error
	request   model.PaymentMovementRequest
	eventID   string
	eventType string
}

func (l *fakeLedger) RecordPaymentMovementOnce(ctx context.Context, eventID string, eventType string, request model.PaymentMovementRequest) ([]model.Entry, bool, error) {
	l.eventID = eventID
	l.eventType = eventType
	l.request = request
	if l.err != nil {
		return nil, false, l.err
	}
	return []model.Entry{{ID: "entry-1"}, {ID: "entry-2"}}, l.processed, nil
}

func TestHandleMessageRecordsPaymentCreated(t *testing.T) {
	ledger := &fakeLedger{processed: true}
	consumer := NewPaymentConsumer(nil, ledger, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := consumer.HandleMessage(context.Background(), kafka.Message{
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte("5de6b73e-1c90-4597-84a8-2d4bf34be7f8")},
			{Key: "event_type", Value: []byte(PaymentCreatedEvent)},
		},
		Value: []byte(`{"schema_version":1,"payment_id":"payment-1","amount_cents":1299,"currency":"USD"}`),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if ledger.eventID != "5de6b73e-1c90-4597-84a8-2d4bf34be7f8" {
		t.Fatalf("eventID = %q, want header event id", ledger.eventID)
	}
	if ledger.request.PaymentID != "payment-1" {
		t.Fatalf("PaymentID = %q, want payment-1", ledger.request.PaymentID)
	}
	if ledger.request.AmountCents != 1299 {
		t.Fatalf("AmountCents = %d, want 1299", ledger.request.AmountCents)
	}
}

func TestHandleMessageRequiresEventID(t *testing.T) {
	consumer := NewPaymentConsumer(nil, &fakeLedger{processed: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := consumer.HandleMessage(context.Background(), kafka.Message{
		Headers: []kafka.Header{{Key: "event_type", Value: []byte(PaymentCreatedEvent)}},
		Value:   []byte(`{"payment_id":"payment-1","amount_cents":1299,"currency":"USD"}`),
	})
	if err == nil {
		t.Fatalf("HandleMessage() error = nil, want missing event id error")
	}
}

func TestHandleMessageIgnoresStatusChanged(t *testing.T) {
	ledger := &fakeLedger{processed: true}
	consumer := NewPaymentConsumer(nil, ledger, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := consumer.HandleMessage(context.Background(), kafka.Message{
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte("5de6b73e-1c90-4597-84a8-2d4bf34be7f8")},
			{Key: "event_type", Value: []byte(PaymentStatusChangedEvent)},
		},
		Value: []byte(`{"payment_id":"payment-1"}`),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if ledger.eventID != "" {
		t.Fatalf("ledger was called for status changed event")
	}
}

func TestHandleMessageReturnsLedgerError(t *testing.T) {
	consumer := NewPaymentConsumer(nil, &fakeLedger{err: errors.New("ledger unavailable")}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := consumer.HandleMessage(context.Background(), kafka.Message{
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte("5de6b73e-1c90-4597-84a8-2d4bf34be7f8")},
			{Key: "event_type", Value: []byte(PaymentCreatedEvent)},
		},
		Value: []byte(`{"payment_id":"payment-1","amount_cents":1299,"currency":"USD"}`),
	})
	if err == nil {
		t.Fatalf("HandleMessage() error = nil, want ledger error")
	}
}
