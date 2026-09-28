package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/config"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/kafkaclient"
)

const DeadLetterSchemaVersion = 1

type MessageWriter interface {
	WriteMessages(ctx context.Context, messages ...kafka.Message) error
	Close() error
}

type DeadLetterHeader struct {
	Key   string `json:"key"`
	Value []byte `json:"value"`
}

type DeadLetterRecord struct {
	SchemaVersion   int                `json:"schema_version"`
	EventID         string             `json:"event_id"`
	EventType       string             `json:"event_type"`
	SourceTopic     string             `json:"source_topic"`
	SourcePartition int                `json:"source_partition"`
	SourceOffset    int64              `json:"source_offset"`
	Key             []byte             `json:"key"`
	Value           []byte             `json:"value"`
	Headers         []DeadLetterHeader `json:"headers"`
	Failure         string             `json:"failure"`
	Attempts        int                `json:"attempts"`
	FailedAt        time.Time          `json:"failed_at"`
}

type DeadLetterPublisher struct {
	writer MessageWriter
	topic  string
}

func NewDeadLetterPublisher(writer MessageWriter, topic string) *DeadLetterPublisher {
	return &DeadLetterPublisher{writer: writer, topic: topic}
}

func NewKafkaWriter(brokers []string, topic string, clientID string, security config.KafkaSecurityConfig) (MessageWriter, error) {
	tlsConfig, saslMechanism, err := kafkaclient.BuildSecurity(security)
	if err != nil {
		return nil, err
	}
	transport := &kafka.Transport{ClientID: clientID, TLS: tlsConfig, SASL: saslMechanism}
	return &kafkaMessageWriter{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			Transport:    transport,
		},
		transport: transport,
	}, nil
}

func NewPartitionReader(brokers []string, topic string, partition int, offset int64, clientID string, security config.KafkaSecurityConfig) (*kafka.Reader, error) {
	tlsConfig, saslMechanism, err := kafkaclient.BuildSecurity(security)
	if err != nil {
		return nil, err
	}
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers, Topic: topic, Partition: partition, StartOffset: offset,
		Dialer: &kafka.Dialer{
			ClientID: clientID, Timeout: 10 * time.Second, DualStack: true,
			TLS: tlsConfig, SASLMechanism: saslMechanism,
		},
		MinBytes: 1, MaxBytes: 10e6,
	}), nil
}

func (p *DeadLetterPublisher) Publish(ctx context.Context, message kafka.Message, cause error, attempts int) error {
	headers := make([]DeadLetterHeader, 0, len(message.Headers))
	for _, header := range message.Headers {
		headers = append(headers, DeadLetterHeader{Key: header.Key, Value: header.Value})
	}
	record := DeadLetterRecord{
		SchemaVersion:   DeadLetterSchemaVersion,
		EventID:         headerValue(message.Headers, "event_id"),
		EventType:       headerValue(message.Headers, "event_type"),
		SourceTopic:     message.Topic,
		SourcePartition: message.Partition,
		SourceOffset:    message.Offset,
		Key:             message.Key,
		Value:           message.Value,
		Headers:         headers,
		Failure:         boundedFailure(cause),
		Attempts:        attempts,
		FailedAt:        time.Now().UTC(),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal dead-letter record: %w", err)
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic: p.topic,
		Key:   message.Key,
		Value: payload,
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(record.EventID)},
			{Key: "event_type", Value: []byte(record.EventType)},
			{Key: "source_topic", Value: []byte(record.SourceTopic)},
		},
	}); err != nil {
		return fmt.Errorf("publish dead-letter record: %w", err)
	}
	return nil
}

func (p *DeadLetterPublisher) Close() error { return p.writer.Close() }

func boundedFailure(err error) string {
	const maxLength = 2048
	value := err.Error()
	if len(value) > maxLength {
		return value[:maxLength]
	}
	return value
}

type kafkaMessageWriter struct {
	writer    *kafka.Writer
	transport *kafka.Transport
}

func (w *kafkaMessageWriter) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	return w.writer.WriteMessages(ctx, messages...)
}

func (w *kafkaMessageWriter) Close() error {
	err := w.writer.Close()
	w.transport.CloseIdleConnections()
	return err
}
