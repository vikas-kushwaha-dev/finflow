package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/service"
)

type paymentCreator interface {
	Create(ctx context.Context, request model.CreatePaymentRequest, idempotencyKey string) (service.CreatePaymentResult, error)
}

type PaymentHandler struct {
	service paymentCreator
}

func NewPaymentHandler(service paymentCreator) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) RegisterRoutes(router chi.Router) {
	router.Post("/payments", h.createPayment)
}

func (h *PaymentHandler) createPayment(w http.ResponseWriter, r *http.Request) {
	var request model.CreatePaymentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := h.service.Create(r.Context(), request, r.Header.Get("Idempotency-Key"))
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			writeError(w, http.StatusBadRequest, "invalid payment request")
			return
		}

		writeError(w, http.StatusInternalServerError, "could not create payment")
		return
	}

	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}

	WriteJSON(w, status, result.Payment)
}
