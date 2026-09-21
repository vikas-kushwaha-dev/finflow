package publisher

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/event"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/kafkaclient"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/repository"
)

type Writer interface {
	WriteMessages(ctx context.Context, messages ...kafka.Message) error
	Close() error
}

type OutboxPublisher struct {
	store  repository.OutboxRepository
	writer Writer
	topic  string
	logger *slog.Logger
}

func NewOutboxPublisher(store repository.OutboxRepository, writer Writer, topic string, logger *slog.Logger) *OutboxPublisher {
	return &OutboxPublisher{
		store:  store,
		writer: writer,
		topic:  topic,
		logger: logger,
	}
}

type KafkaWriter struct {
	writer    *kafka.Writer
	transport *kafka.Transport
}

func NewKafkaWriter(brokers []string, topic string, security config.KafkaSecurityConfig) (*KafkaWriter, error) {
	tlsConfig, saslMechanism, err := kafkaclient.BuildSecurity(security)
	if err != nil {
		return nil, err
	}

	transport := &kafka.Transport{
		ClientID: "finflow-outbox-publisher",
		TLS:      tlsConfig,
		SASL:     saslMechanism,
	}

	return &KafkaWriter{
		transport: transport,
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			Transport:    transport,
		},
	}, nil
}

func (w *KafkaWriter) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	return w.writer.WriteMessages(ctx, messages...)
}

func (w *KafkaWriter) Close() error {
	err := w.writer.Close()
	w.transport.CloseIdleConnections()
	return err
}

func (p *OutboxPublisher) Run(ctx context.Context, interval time.Duration, batchSize int) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer p.writer.Close()

	for {
		if err := p.PublishBatch(ctx, batchSize); err != nil {
			p.logger.Error("publish outbox batch failed", "error", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *OutboxPublisher) PublishBatch(ctx context.Context, batchSize int) error {
	events, err := p.store.ClaimPending(ctx, batchSize)
	if err != nil {
		return err
	}

	for _, outboxEvent := range events {
		if err := p.publishOne(ctx, outboxEvent); err != nil {
			if markErr := p.store.MarkFailed(ctx, outboxEvent.ID, err.Error()); markErr != nil {
				p.logger.Error("mark outbox event failed failed", "event_id", outboxEvent.ID, "error", markErr)
			}
			continue
		}

		if err := p.store.MarkPublished(ctx, outboxEvent.ID, time.Now().UTC()); err != nil {
			p.logger.Error("mark outbox event published failed", "event_id", outboxEvent.ID, "error", err)
		}
	}

	return nil
}

func (p *OutboxPublisher) publishOne(ctx context.Context, outboxEvent event.OutboxEvent) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic: p.topic,
		Key:   []byte(outboxEvent.AggregateID),
		Value: outboxEvent.Payload,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(outboxEvent.ID)},
			{Key: "event_type", Value: []byte(outboxEvent.EventType)},
			{Key: "aggregate_type", Value: []byte(outboxEvent.AggregateType)},
		},
	})
}
