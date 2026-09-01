package service

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/repository"
)

var ErrValidation = errors.New("validation failed")
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

	payment := model.Payment{
		ID:                uuid.NewString(),
		AmountCents:       request.AmountCents,
		Currency:          request.Currency,
		Status:            model.PaymentStatusPending,
		Description:       request.Description,
		ExternalReference: request.ExternalReference,
		IdempotencyKey:    idempotencyKey,
	}

	createdPayment, created, err := s.repository.Create(ctx, payment)
	if err != nil {
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
