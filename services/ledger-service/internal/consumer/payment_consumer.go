package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/kafkaclient"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
)

const (
	PaymentCreatedEvent       = "payment.created"
	PaymentStatusChangedEvent = "payment.status_changed"
)

type Reader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, messages ...kafka.Message) error
	Close() error
}

type Ledger interface {
	RecordPaymentMovementOnce(ctx context.Context, eventID string, eventType string, request model.PaymentMovementRequest) ([]model.Entry, bool, error)
}

type PaymentConsumer struct {
	reader Reader
	ledger Ledger
	logger *slog.Logger
}

type paymentCreatedPayload struct {
	SchemaVersion int    `json:"schema_version"`
	PaymentID     string `json:"payment_id"`
	AmountCents   int64  `json:"amount_cents"`
	Currency      string `json:"currency"`
}

func NewPaymentConsumer(reader Reader, ledger Ledger, logger *slog.Logger) *PaymentConsumer {
	return &PaymentConsumer{
		reader: reader,
		ledger: ledger,
		logger: logger,
	}
}

func NewKafkaReader(brokers []string, topic string, groupID string, security config.KafkaSecurityConfig) (*kafka.Reader, error) {
	tlsConfig, saslMechanism, err := kafkaclient.BuildSecurity(security)
	if err != nil {
		return nil, err
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,
		Dialer: &kafka.Dialer{
			ClientID:      "finflow-ledger-consumer",
			Timeout:       10 * time.Second,
			DualStack:     true,
			TLS:           tlsConfig,
			SASLMechanism: saslMechanism,
		},
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0,
	})

	return reader, nil
}

func (c *PaymentConsumer) Run(ctx context.Context) error {
	defer c.reader.Close()

	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			c.logger.Error("fetch payment event failed", "error", err)
			time.Sleep(time.Second)
			continue
		}

		if err := c.HandleMessage(ctx, message); err != nil {
			c.logger.Error("handle payment event failed", "error", err)
			continue
		}

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			c.logger.Error("commit payment event failed", "error", err)
		}
	}
}

func (c *PaymentConsumer) HandleMessage(ctx context.Context, message kafka.Message) error {
	eventID := headerValue(message.Headers, "event_id")
	eventType := headerValue(message.Headers, "event_type")
	if eventID == "" {
		return fmt.Errorf("payment event missing event_id header")
	}

	switch eventType {
	case PaymentCreatedEvent:
		return c.handlePaymentCreated(ctx, eventID, eventType, message.Value)
	case PaymentStatusChangedEvent:
		c.logger.Info("payment status event ignored by ledger", "event_id", eventID)
		return nil
	default:
		c.logger.Info("unsupported payment event ignored", "event_id", eventID, "event_type", eventType)
		return nil
	}
}

func (c *PaymentConsumer) handlePaymentCreated(ctx context.Context, eventID string, eventType string, value []byte) error {
	var payload paymentCreatedPayload
	if err := json.Unmarshal(value, &payload); err != nil {
		return fmt.Errorf("decode payment.created payload: %w", err)
	}

	entries, processed, err := c.ledger.RecordPaymentMovementOnce(ctx, eventID, eventType, model.PaymentMovementRequest{
		PaymentID:   payload.PaymentID,
		AmountCents: payload.AmountCents,
		Currency:    payload.Currency,
	})
	if err != nil {
		return err
	}

	if !processed {
		c.logger.Info("duplicate payment event ignored", "event_id", eventID, "payment_id", payload.PaymentID)
		return nil
	}

	c.logger.Info("payment event consumed", "event_id", eventID, "payment_id", payload.PaymentID, "entries", len(entries))
	return nil
}

func headerValue(headers []kafka.Header, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}

	return ""
}
