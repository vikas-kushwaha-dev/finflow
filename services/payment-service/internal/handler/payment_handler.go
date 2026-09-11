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
	GetByID(ctx context.Context, id string) (model.Payment, error)
	UpdateStatus(ctx context.Context, id string, request model.UpdatePaymentStatusRequest) (model.Payment, error)
}

type PaymentHandler struct {
	service paymentCreator
}

func NewPaymentHandler(service paymentCreator) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) RegisterRoutes(router chi.Router) {
	router.Post("/payments", h.createPayment)
	router.Get("/payments/{id}", h.getPayment)
	router.Patch("/payments/{id}/status", h.updatePaymentStatus)
}

func (h *PaymentHandler) createPayment(w http.ResponseWriter, r *http.Request) {
	var request model.CreatePaymentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeRequestError(w, r, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		writeRequestError(w, r, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := h.service.Create(r.Context(), request, r.Header.Get("Idempotency-Key"))
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			writeRequestError(w, r, http.StatusBadRequest, "invalid payment request")
			return
		}

		if errors.Is(err, service.ErrIdempotencyConflict) {
			writeRequestError(w, r, http.StatusConflict, "idempotency key reused with different request")
			return
		}

		writeRequestError(w, r, http.StatusInternalServerError, "could not create payment")
		return
	}

	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
		w.Header().Set("X-Idempotent-Replay", "true")
	}

	WriteJSON(w, status, result.Payment)
}

func (h *PaymentHandler) getPayment(w http.ResponseWriter, r *http.Request) {
	payment, err := h.service.GetByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			writeRequestError(w, r, http.StatusBadRequest, "invalid payment id")
			return
		}

		if errors.Is(err, service.ErrPaymentNotFound) {
			writeRequestError(w, r, http.StatusNotFound, "payment not found")
			return
		}

		writeRequestError(w, r, http.StatusInternalServerError, "could not get payment")
		return
	}

	WriteJSON(w, http.StatusOK, payment)
}

func (h *PaymentHandler) updatePaymentStatus(w http.ResponseWriter, r *http.Request) {
	var request model.UpdatePaymentStatusRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeRequestError(w, r, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		writeRequestError(w, r, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	payment, err := h.service.UpdateStatus(r.Context(), chi.URLParam(r, "id"), request)
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			writeRequestError(w, r, http.StatusBadRequest, "invalid payment status request")
			return
		}

		if errors.Is(err, service.ErrPaymentNotFound) {
			writeRequestError(w, r, http.StatusNotFound, "payment not found")
			return
		}

		if errors.Is(err, service.ErrInvalidStatusTransition) {
			writeRequestError(w, r, http.StatusConflict, "invalid payment status transition")
			return
		}

		writeRequestError(w, r, http.StatusInternalServerError, "could not update payment status")
		return
	}

	WriteJSON(w, http.StatusOK, payment)
}
