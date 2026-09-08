package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/repository"
)

var ErrValidation = errors.New("validation failed")
var ErrIdempotencyConflict = repository.ErrIdempotencyConflict
var ErrInvalidStatusTransition = errors.New("invalid payment status transition")
var ErrPaymentNotFound = repository.ErrPaymentNotFound

type PaymentService struct {
	repository repository.PaymentRepository
}

type CreatePaymentResult struct {
	Payment model.Payment
	Created bool
}

func NewPaymentService(repository repository.PaymentRepository) *PaymentService {
	return &PaymentService{repository: repository}
}

func (s *PaymentService) Create(ctx context.Context, request model.CreatePaymentRequest, idempotencyKey string) (CreatePaymentResult, error) {
	request.Currency = strings.ToUpper(strings.TrimSpace(request.Currency))
	request.Description = strings.TrimSpace(request.Description)
	request.ExternalReference = strings.TrimSpace(request.ExternalReference)
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	if err := validateCreatePayment(request, idempotencyKey); err != nil {
		return CreatePaymentResult{}, err
	}

	idempotencyHash := ""
	if idempotencyKey != "" {
		idempotencyHash = createPaymentRequestHash(request)
	}

	payment := model.Payment{
		ID:                uuid.NewString(),
		AmountCents:       request.AmountCents,
		Currency:          request.Currency,
		Status:            model.PaymentStatusPending,
		Description:       request.Description,
		ExternalReference: request.ExternalReference,
		IdempotencyKey:    idempotencyKey,
		IdempotencyHash:   idempotencyHash,
	}

	createdPayment, created, err := s.repository.Create(ctx, payment)
	if err != nil {
		if errors.Is(err, repository.ErrIdempotencyConflict) {
			return CreatePaymentResult{}, ErrIdempotencyConflict
		}
		return CreatePaymentResult{}, err
	}

	return CreatePaymentResult{
		Payment: createdPayment,
		Created: created,
	}, nil
}

func (s *PaymentService) GetByID(ctx context.Context, id string) (model.Payment, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return model.Payment{}, ErrValidation
	}

	payment, err := s.repository.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			return model.Payment{}, ErrPaymentNotFound
		}
		return model.Payment{}, err
	}

	return payment, nil
}

func (s *PaymentService) UpdateStatus(ctx context.Context, id string, request model.UpdatePaymentStatusRequest) (model.Payment, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return model.Payment{}, ErrValidation
	}

	status := model.PaymentStatus(strings.ToLower(strings.TrimSpace(string(request.Status))))
	if !isKnownStatus(status) {
		return model.Payment{}, ErrValidation
	}

	current, err := s.repository.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			return model.Payment{}, ErrPaymentNotFound
		}
		return model.Payment{}, err
	}

	if current.Status == status {
		return current, nil
	}

	if !canTransition(current.Status, status) {
		return model.Payment{}, ErrInvalidStatusTransition
	}

	updated, err := s.repository.UpdateStatus(ctx, id, status)
	if err != nil {
		if errors.Is(err, repository.ErrPaymentNotFound) {
			return model.Payment{}, ErrPaymentNotFound
		}
		return model.Payment{}, err
	}

	return updated, nil
}

func validateCreatePayment(request model.CreatePaymentRequest, idempotencyKey string) error {
	if request.AmountCents <= 0 {
		return ErrValidation
	}

	if len(request.Currency) != 3 {
		return ErrValidation
	}

	for _, char := range request.Currency {
		if !unicode.IsLetter(char) || !unicode.IsUpper(char) {
			return ErrValidation
		}
	}

	if len(request.Description) > 500 {
		return ErrValidation
	}

	if len(request.ExternalReference) > 120 {
		return ErrValidation
	}

	if len(idempotencyKey) > 120 {
		return ErrValidation
	}

	return nil
}

func isKnownStatus(status model.PaymentStatus) bool {
	switch status {
	case model.PaymentStatusPending, model.PaymentStatusSucceeded, model.PaymentStatusFailed:
		return true
	default:
		return false
	}
}

func canTransition(from, to model.PaymentStatus) bool {
	return from == model.PaymentStatusPending &&
		(to == model.PaymentStatusSucceeded || to == model.PaymentStatusFailed)
}

func createPaymentRequestHash(request model.CreatePaymentRequest) string {
	parts := []string{
		strconv.FormatInt(request.AmountCents, 10),
		request.Currency,
		request.Description,
		request.ExternalReference,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))

	return hex.EncodeToString(sum[:])
}
