package service

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/repository"
)

const (
	cashAccountName            = "finflow_cash"
	merchantPayableAccountName = "merchant_payable"
)

var ErrValidation = errors.New("validation failed")
var ErrUnbalancedTransaction = errors.New("ledger transaction is not balanced")

type LedgerService struct {
	repository repository.LedgerRepository
}

func NewLedgerService(repository repository.LedgerRepository) *LedgerService {
	return &LedgerService{repository: repository}
}

func (s *LedgerService) RecordPaymentMovement(ctx context.Context, request model.PaymentMovementRequest) ([]model.Entry, error) {
	entries, err := s.buildPaymentMovement(ctx, request)
	if err != nil {
		return nil, err
	}

	return s.repository.CreateTransaction(ctx, entries)
}

func (s *LedgerService) RecordPaymentMovementOnce(ctx context.Context, eventID string, eventType string, request model.PaymentMovementRequest) ([]model.Entry, bool, error) {
	eventID = strings.TrimSpace(eventID)
	eventType = strings.TrimSpace(eventType)
	if _, err := uuid.Parse(eventID); err != nil {
		return nil, false, ErrValidation
	}
	if eventType == "" {
		return nil, false, ErrValidation
	}

	entries, err := s.buildPaymentMovement(ctx, request)
	if err != nil {
		return nil, false, err
	}

	created, processed, err := s.repository.CreateTransactionOnce(ctx, eventID, eventType, strings.TrimSpace(request.PaymentID), entries)
	if err != nil {
		return nil, false, err
	}

	return created, processed, nil
}

func (s *LedgerService) buildPaymentMovement(ctx context.Context, request model.PaymentMovementRequest) ([]model.Entry, error) {
	request.Currency = strings.ToUpper(strings.TrimSpace(request.Currency))
	request.PaymentID = strings.TrimSpace(request.PaymentID)

	if err := validatePaymentMovement(request); err != nil {
		return nil, err
	}

	cashAccount, err := s.repository.EnsureAccount(ctx, cashAccountName, request.Currency, model.DirectionDebit)
	if err != nil {
		return nil, err
	}

	merchantPayableAccount, err := s.repository.EnsureAccount(ctx, merchantPayableAccountName, request.Currency, model.DirectionCredit)
	if err != nil {
		return nil, err
	}

	transactionID := uuid.NewString()
	entries := []model.Entry{
		{
			ID:            uuid.NewString(),
			TransactionID: transactionID,
			AccountID:     cashAccount.ID,
			Direction:     model.DirectionDebit,
			AmountCents:   request.AmountCents,
			Currency:      request.Currency,
			ReferenceType: "payment",
			ReferenceID:   request.PaymentID,
		},
		{
			ID:            uuid.NewString(),
			TransactionID: transactionID,
			AccountID:     merchantPayableAccount.ID,
			Direction:     model.DirectionCredit,
			AmountCents:   request.AmountCents,
			Currency:      request.Currency,
			ReferenceType: "payment",
			ReferenceID:   request.PaymentID,
		},
	}

	if err := ValidateBalanced(entries); err != nil {
		return nil, err
	}

	return entries, nil
}

func ValidateBalanced(entries []model.Entry) error {
	if len(entries) < 2 {
		return ErrUnbalancedTransaction
	}

	totals := map[string]int64{}
	for _, entry := range entries {
		if entry.AmountCents <= 0 || !isKnownDirection(entry.Direction) || !isCurrency(entry.Currency) {
			return ErrValidation
		}

		switch entry.Direction {
		case model.DirectionDebit:
			totals[entry.Currency] += entry.AmountCents
		case model.DirectionCredit:
			totals[entry.Currency] -= entry.AmountCents
		}
	}

	for _, total := range totals {
		if total != 0 {
			return ErrUnbalancedTransaction
		}
	}

	return nil
}

func validatePaymentMovement(request model.PaymentMovementRequest) error {
	if request.PaymentID == "" {
		return ErrValidation
	}
	if request.AmountCents <= 0 {
		return ErrValidation
	}
	if !isCurrency(request.Currency) {
		return ErrValidation
	}

	return nil
}

func isKnownDirection(direction model.Direction) bool {
	return direction == model.DirectionDebit || direction == model.DirectionCredit
}

func isCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}

	for _, char := range currency {
		if !unicode.IsUpper(char) || !unicode.IsLetter(char) {
			return false
		}
	}

	return true
}
