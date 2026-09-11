package publisher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/event"
)

type fakeOutboxStore struct {
	events       []event.OutboxEvent
	publishedIDs []string
	failedIDs    []string
	claimErr     error
}

func (s *fakeOutboxStore) ClaimPending(ctx context.Context, limit int) ([]event.OutboxEvent, error) {
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	return s.events, nil
}

func (s *fakeOutboxStore) MarkPublished(ctx context.Context, id string, publishedAt time.Time) error {
	s.publishedIDs = append(s.publishedIDs, id)
	return nil
}

func (s *fakeOutboxStore) MarkFailed(ctx context.Context, id string, reason string) error {
	s.failedIDs = append(s.failedIDs, id)
	return nil
}

type fakeWriter struct {
	messages []kafka.Message
	err      error
	closed   bool
}

func (w *fakeWriter) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	if w.err != nil {
		return w.err
	}
	w.messages = append(w.messages, messages...)
	return nil
}

func (w *fakeWriter) Close() error {
	w.closed = true
	return nil
}

func TestPublishBatchPublishesAndMarksEvents(t *testing.T) {
	store := &fakeOutboxStore{
		events: []event.OutboxEvent{
			{
				ID:            "event-1",
				AggregateType: event.AggregateTypePayment,
				AggregateID:   "payment-1",
				EventType:     event.PaymentCreatedEvent,
				Payload:       []byte(`{"payment_id":"payment-1"}`),
			},
		},
	}
	writer := &fakeWriter{}
	publisher := NewOutboxPublisher(store, writer, event.PaymentEventsTopic, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := publisher.PublishBatch(context.Background(), 10); err != nil {
		t.Fatalf("PublishBatch() error = %v", err)
	}

	if len(writer.messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(writer.messages))
	}
	if string(writer.messages[0].Key) != "payment-1" {
		t.Fatalf("message key = %q, want payment-1", string(writer.messages[0].Key))
	}
	if len(store.publishedIDs) != 1 || store.publishedIDs[0] != "event-1" {
		t.Fatalf("publishedIDs = %v, want [event-1]", store.publishedIDs)
	}
}

func TestPublishBatchMarksFailedEvent(t *testing.T) {
	store := &fakeOutboxStore{
		events: []event.OutboxEvent{{ID: "event-1", AggregateID: "payment-1"}},
	}
	writer := &fakeWriter{err: errors.New("kafka unavailable")}
	publisher := NewOutboxPublisher(store, writer, event.PaymentEventsTopic, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := publisher.PublishBatch(context.Background(), 10); err != nil {
		t.Fatalf("PublishBatch() error = %v", err)
	}

	if len(store.failedIDs) != 1 || store.failedIDs[0] != "event-1" {
		t.Fatalf("failedIDs = %v, want [event-1]", store.failedIDs)
	}
}
