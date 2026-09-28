package replay

import (
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/consumer"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
)

type fakeAudits struct {
	startErr  error
	finished  bool
	succeeded bool
	failure   string
	source    repository.ReplaySource
}

func (a *fakeAudits) StartReplay(_ context.Context, source repository.ReplaySource) (string, error) {
	a.source = source
	return "audit-1", a.startErr
}

func (a *fakeAudits) FinishReplay(_ context.Context, _ string, succeeded bool, failure string) error {
	a.finished, a.succeeded, a.failure = true, succeeded, failure
	return nil
}

type fakeWriter struct {
	messages []kafka.Message
	err      error
}

func (w *fakeWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.messages = append(w.messages, messages...)
	return w.err
}
func (w *fakeWriter) Close() error { return nil }

func TestReplayPublishesOriginalMessageAndCompletesAudit(t *testing.T) {
	audits := &fakeAudits{}
	writer := &fakeWriter{}
	replayer := New(audits, writer, "finflow.payment.events")
	record := consumer.DeadLetterRecord{
		SchemaVersion: consumer.DeadLetterSchemaVersion,
		EventID:       "event-1", SourceTopic: "finflow.payment.events",
		SourcePartition: 2, SourceOffset: 42, Key: []byte("payment-1"), Value: []byte("{}"),
		Headers: []consumer.DeadLetterHeader{{Key: "event_id", Value: []byte("event-1")}},
	}
	if err := replayer.Replay(context.Background(), record, "operator@example.com", "database incident resolved"); err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if len(writer.messages) != 1 || writer.messages[0].Topic != record.SourceTopic {
		t.Fatalf("published messages = %#v", writer.messages)
	}
	if !audits.finished || !audits.succeeded {
		t.Fatalf("audit completion = finished %v succeeded %v", audits.finished, audits.succeeded)
	}
}

func TestReplayRecordsPublishFailure(t *testing.T) {
	audits := &fakeAudits{}
	writer := &fakeWriter{err: errors.New("Kafka unavailable")}
	replayer := New(audits, writer, "finflow.payment.events")
	err := replayer.Replay(context.Background(), consumer.DeadLetterRecord{
		SchemaVersion: consumer.DeadLetterSchemaVersion,
		EventID:       "event-1", SourceTopic: "finflow.payment.events",
	}, "operator@example.com", "retry")
	if err == nil {
		t.Fatal("Replay() error = nil")
	}
	if !audits.finished || audits.succeeded || audits.failure == "" {
		t.Fatalf("failure audit not recorded: %#v", audits)
	}
}

func TestReplayRejectsUnexpectedSourceTopicBeforeAudit(t *testing.T) {
	audits := &fakeAudits{}
	err := New(audits, &fakeWriter{}, "finflow.payment.events").Replay(context.Background(), consumer.DeadLetterRecord{
		SchemaVersion: consumer.DeadLetterSchemaVersion,
		EventID:       "event-1", SourceTopic: "unexpected.topic",
	}, "operator@example.com", "retry")
	if err == nil {
		t.Fatal("Replay() error = nil")
	}
	if audits.source.EventID != "" {
		t.Fatal("audit started for an unapproved source topic")
	}
}
