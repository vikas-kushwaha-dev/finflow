package model

import "time"

type Direction string

const (
	DirectionDebit  Direction = "debit"
	DirectionCredit Direction = "credit"
)

type Account struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Currency      string    `json:"currency"`
	NormalBalance Direction `json:"normal_balance"`
	CreatedAt     time.Time `json:"created_at"`
}

type Entry struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	AccountID     string    `json:"account_id"`
	Direction     Direction `json:"direction"`
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   string    `json:"reference_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type Balance struct {
	AccountID   string    `json:"account_id"`
	AccountName string    `json:"account_name"`
	Currency    string    `json:"currency"`
	AmountCents int64     `json:"amount_cents"`
	AsOf        time.Time `json:"as_of"`
}

type PaymentMovementRequest struct {
	PaymentID   string
	AmountCents int64
	Currency    string
}
