package model

import "time"

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
)

type Payment struct {
	ID                string        `json:"id"`
	AmountCents       int64         `json:"amount_cents"`
	Currency          string        `json:"currency"`
	Status            PaymentStatus `json:"status"`
	Description       string        `json:"description,omitempty"`
	ExternalReference string        `json:"external_reference,omitempty"`
	IdempotencyKey    string        `json:"idempotency_key,omitempty"`
	IdempotencyHash   string        `json:"-"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type CreatePaymentRequest struct {
	AmountCents       int64  `json:"amount_cents"`
	Currency          string `json:"currency"`
	Description       string `json:"description"`
	ExternalReference string `json:"external_reference"`
}

type UpdatePaymentStatusRequest struct {
	Status PaymentStatus `json:"status"`
}
