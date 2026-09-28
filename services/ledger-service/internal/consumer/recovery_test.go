package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/observability"
)

type recoveryReader struct {
	message   kafka.Message
	fetched   bool
	committed int
}

func (r *recoveryReader) FetchMessage(context.Context) (kafka.Message, error) {
	if r.fetched {
		return kafka.Message{}, context.Canceled
	}
	r.fetched = true
	return r.message, nil
}
func (r *recoveryReader) CommitMessages(context.Context, ...kafka.Message) error {
	r.committed++
	return nil
}
func (r *recoveryReader) Close() error { return nil }

type recoveryWriter struct {
	messages []kafka.Message
	err      error
}

func (w *recoveryWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.messages = append(w.messages, messages...)
	return w.err
}
func (w *recoveryWriter) Close() error { return nil }

type failingLedger struct{ calls int }

func (l *failingLedger) RecordPaymentMovementOnce(context.Context, string, string, model.PaymentMovementRequest) ([]model.Entry, bool, error) {
	l.calls++
	return nil, false, errors.New("database unavailable")
}

func TestRunRetriesThenDeadLettersAndCommits(t *testing.T) {
	message := kafka.Message{
		Topic: "finflow.payment.events", Partition: 1, Offset: 7,
		Headers: []kafka.Header{{Key: "event_id", Value: []byte("event-1")}, {Key: "event_type", Value: []byte(PaymentCreatedEvent)}},
		Value:   []byte(`{"payment_id":"payment-1","amount_cents":1299,"currency":"USD"}`),
	}
	reader := &recoveryReader{message: message}
	writer := &recoveryWriter{}
	ledger := &failingLedger{}
	consumer := NewPaymentConsumer(reader, ledger, NewDeadLetterPublisher(writer, "finflow.payment.events.dead-letter"), 3, time.Millisecond, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)), observability.NewMetrics())

	if err := consumer.Run(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if ledger.calls != 3 || reader.committed != 1 || len(writer.messages) != 1 {
		t.Fatalf("calls=%d committed=%d dead_letters=%d", ledger.calls, reader.committed, len(writer.messages))
	}
	var record DeadLetterRecord
	if err := json.Unmarshal(writer.messages[0].Value, &record); err != nil {
		t.Fatalf("decode dead-letter record: %v", err)
	}
	if record.Attempts != 3 || record.SourceOffset != 7 {
		t.Fatalf("dead-letter record = %#v", record)
	}
}

func TestRunDoesNotCommitWhenDeadLetterPublishFails(t *testing.T) {
	reader := &recoveryReader{message: kafka.Message{
		Topic: "finflow.payment.events", Headers: []kafka.Header{{Key: "event_id", Value: []byte("event-1")}, {Key: "event_type", Value: []byte(PaymentCreatedEvent)}},
		Value: []byte(`{"payment_id":"payment-1","amount_cents":1299,"currency":"USD"}`),
	}}
	writer := &recoveryWriter{err: errors.New("Kafka unavailable")}
	consumer := NewPaymentConsumer(reader, &failingLedger{}, NewDeadLetterPublisher(writer, "dead-letter"), 1, time.Millisecond, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)), observability.NewMetrics())
	if err := consumer.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil")
	}
	if reader.committed != 0 {
		t.Fatalf("committed = %d, want 0", reader.committed)
	}
}

func TestRunDeadLettersMalformedPayloadWithoutRetry(t *testing.T) {
	reader := &recoveryReader{message: kafka.Message{
		Topic: "finflow.payment.events", Offset: 3,
		Headers: []kafka.Header{{Key: "event_id", Value: []byte("event-1")}, {Key: "event_type", Value: []byte(PaymentCreatedEvent)}},
		Value:   []byte("{"),
	}}
	writer := &recoveryWriter{}
	ledger := &failingLedger{}
	consumer := NewPaymentConsumer(reader, ledger, NewDeadLetterPublisher(writer, "dead-letter"), 5, time.Millisecond, time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)), observability.NewMetrics())
	if err := consumer.Run(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
	var record DeadLetterRecord
	if err := json.Unmarshal(writer.messages[0].Value, &record); err != nil {
		t.Fatalf("decode dead-letter record: %v", err)
	}
	if record.Attempts != 1 || ledger.calls != 0 {
		t.Fatalf("attempts=%d ledger_calls=%d, want 1 and 0", record.Attempts, ledger.calls)
	}
}
