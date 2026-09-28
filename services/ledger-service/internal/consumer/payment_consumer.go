package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/kafkaclient"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/observability"
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
	reader              Reader
	ledger              Ledger
	deadLetters         *DeadLetterPublisher
	maxAttempts         int
	retryInitialBackoff time.Duration
	retryMaxBackoff     time.Duration
	logger              *slog.Logger
	metrics             *observability.Metrics
}

type paymentCreatedPayload struct {
	SchemaVersion int    `json:"schema_version"`
	PaymentID     string `json:"payment_id"`
	AmountCents   int64  `json:"amount_cents"`
	Currency      string `json:"currency"`
}

func NewPaymentConsumer(reader Reader, ledger Ledger, deadLetters *DeadLetterPublisher, maxAttempts int, retryInitialBackoff time.Duration, retryMaxBackoff time.Duration, logger *slog.Logger, metrics *observability.Metrics) *PaymentConsumer {
	return &PaymentConsumer{
		reader:              reader,
		ledger:              ledger,
		deadLetters:         deadLetters,
		maxAttempts:         maxAttempts,
		retryInitialBackoff: retryInitialBackoff,
		retryMaxBackoff:     retryMaxBackoff,
		logger:              logger,
		metrics:             metrics,
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
	defer c.deadLetters.Close()

	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			c.logger.Error("fetch payment event failed", "error", err)
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}

		attempts, err := c.handleWithRetry(ctx, message)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			c.logger.Error("payment event retries exhausted", "event_id", headerValue(message.Headers, "event_id"), "attempts", attempts, "error", err)
			if publishErr := c.deadLetters.Publish(ctx, message, err, attempts); publishErr != nil {
				c.metrics.RecordDeadLetter(headerValue(message.Headers, "event_type"), publishErr)
				return fmt.Errorf("dead-letter payment event: %w", publishErr)
			}
			c.metrics.RecordDeadLetter(headerValue(message.Headers, "event_type"), nil)
		}

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			return fmt.Errorf("commit payment event: %w", err)
		}
	}
}

func (c *PaymentConsumer) handleWithRetry(ctx context.Context, message kafka.Message) (int, error) {
	backoff := c.retryInitialBackoff
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		err := c.HandleMessage(ctx, message)
		if err == nil {
			return attempt, nil
		}
		if isPermanent(err) || attempt == c.maxAttempts {
			return attempt, err
		}
		c.metrics.RecordKafkaRetry(headerValue(message.Headers, "event_type"))
		c.logger.Warn("retrying payment event", "event_id", headerValue(message.Headers, "event_id"), "attempt", attempt, "backoff", backoff, "error", err)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return attempt, ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
		if backoff > c.retryMaxBackoff {
			backoff = c.retryMaxBackoff
		}
	}
	return c.maxAttempts, fmt.Errorf("payment event retries exhausted")
}

func (c *PaymentConsumer) HandleMessage(ctx context.Context, message kafka.Message) error {
	carrier := propagation.MapCarrier{}
	for _, header := range message.Headers {
		carrier.Set(header.Key, string(header.Value))
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	eventID := headerValue(message.Headers, "event_id")
	eventType := headerValue(message.Headers, "event_type")
	ctx, span := otel.Tracer("finflow/ledger-consumer").Start(ctx, "kafka consume "+eventType, trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(attribute.String("messaging.system", "kafka"), attribute.String("messaging.destination.name", message.Topic), attribute.Int("messaging.kafka.partition", message.Partition), attribute.Int64("messaging.kafka.message.offset", message.Offset)))
	defer span.End()
	started := time.Now()
	var err error
	defer func() {
		c.metrics.RecordKafkaConsume(eventType, message.Topic, message.Partition, message.HighWaterMark-message.Offset-1, time.Since(started), err)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "consume failed")
		}
	}()
	if eventID == "" {
		err = permanent(fmt.Errorf("payment event missing event_id header"))
		return err
	}

	switch eventType {
	case PaymentCreatedEvent:
		err = c.handlePaymentCreated(ctx, eventID, eventType, message.Value)
		return err
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
		return permanent(fmt.Errorf("decode payment.created payload: %w", err))
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

type permanentError struct{ error }

func permanent(err error) error { return permanentError{error: err} }

func isPermanent(err error) bool {
	var target permanentError
	return errors.As(err, &target)
}

func headerValue(headers []kafka.Header, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}

	return ""
}
