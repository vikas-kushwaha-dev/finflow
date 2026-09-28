package replay

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/consumer"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
)

type AuditStore interface {
	StartReplay(ctx context.Context, source repository.ReplaySource) (string, error)
	FinishReplay(ctx context.Context, id string, succeeded bool, failure string) error
}

type Replayer struct {
	audits      AuditStore
	writer      consumer.MessageWriter
	sourceTopic string
}

func New(audits AuditStore, writer consumer.MessageWriter, sourceTopic string) *Replayer {
	return &Replayer{audits: audits, writer: writer, sourceTopic: sourceTopic}
}

func (r *Replayer) Replay(ctx context.Context, record consumer.DeadLetterRecord, operator string, reason string) error {
	if record.SchemaVersion != consumer.DeadLetterSchemaVersion {
		return fmt.Errorf("unsupported dead-letter schema version %d", record.SchemaVersion)
	}
	if record.EventID == "" {
		return errors.New("dead-letter record has no event_id")
	}
	if record.SourceTopic != r.sourceTopic {
		return fmt.Errorf("dead-letter source topic %q is not allowed", record.SourceTopic)
	}
	if operator == "" || reason == "" {
		return errors.New("replay operator and reason are required")
	}

	auditID, err := r.audits.StartReplay(ctx, repository.ReplaySource{
		EventID: record.EventID, Operator: operator, Reason: reason,
		Topic: record.SourceTopic, Partition: record.SourcePartition, Offset: record.SourceOffset,
	})
	if err != nil {
		return err
	}

	headers := make([]kafka.Header, 0, len(record.Headers)+2)
	for _, header := range record.Headers {
		headers = append(headers, kafka.Header{Key: header.Key, Value: header.Value})
	}
	headers = append(headers,
		kafka.Header{Key: "replayed_at", Value: []byte(time.Now().UTC().Format(time.RFC3339Nano))},
		kafka.Header{Key: "replay_audit_id", Value: []byte(auditID)},
	)
	if err := r.writer.WriteMessages(ctx, kafka.Message{
		Topic: r.sourceTopic, Key: record.Key, Value: record.Value, Headers: headers,
	}); err != nil {
		_ = r.audits.FinishReplay(ctx, auditID, false, err.Error())
		return fmt.Errorf("republish dead-letter event: %w", err)
	}
	if err := r.audits.FinishReplay(ctx, auditID, true, ""); err != nil {
		return fmt.Errorf("record successful dead-letter replay: %w", err)
	}
	return nil
}
