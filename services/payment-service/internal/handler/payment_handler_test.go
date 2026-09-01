package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/payment-service/internal/service"
)

type fakePaymentService struct {
	result service.CreatePaymentResult
	err    error
}

func (s fakePaymentService) Create(ctx context.Context, request model.CreatePaymentRequest, idempotencyKey string) (service.CreatePaymentResult, error) {
	if s.err != nil {
		return service.CreatePaymentResult{}, s.err
	}
	return s.result, nil
}

func (s fakePaymentService) GetByID(ctx context.Context, id string) (model.Payment, error) {
	if s.err != nil {
		return model.Payment{}, s.err
	}
	return s.result.Payment, nil
}

func TestCreatePaymentReturnsCreated(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{
		result: service.CreatePaymentResult{
			Created: true,
			Payment: model.Payment{
				ID:          "payment-1",
				AmountCents: 1299,
				Currency:    "USD",
				Status:      model.PaymentStatusPending,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			},
		},
	}).RegisterRoutes(router)

	body := bytes.NewBufferString(`{"amount_cents":1299,"currency":"USD"}`)
	req := httptest.NewRequest(http.MethodPost, "/payments", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "key-1")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var payment model.Payment
	if err := json.NewDecoder(rec.Body).Decode(&payment); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payment.ID != "payment-1" {
		t.Fatalf("Payment.ID = %q, want payment-1", payment.ID)
	}
}

func TestCreatePaymentReturnsOKForIdempotentReplay(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{
		result: service.CreatePaymentResult{
			Created: false,
			Payment: model.Payment{
				ID:          "payment-1",
				AmountCents: 1299,
				Currency:    "USD",
				Status:      model.PaymentStatusPending,
			},
		},
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"amount_cents":1299,"currency":"USD"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCreatePaymentRejectsBadJSON(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"amount_cents":`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreatePaymentReturnsValidationError(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{err: service.ErrValidation}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"amount_cents":0,"currency":"USD"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreatePaymentReturnsServerError(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{err: errors.New("database unavailable")}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBufferString(`{"amount_cents":1299,"currency":"USD"}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestGetPaymentReturnsPayment(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{
		result: service.CreatePaymentResult{
			Payment: model.Payment{
				ID:          "5de6b73e-1c90-4597-84a8-2d4bf34be7f8",
				AmountCents: 1299,
				Currency:    "USD",
				Status:      model.PaymentStatusPending,
			},
		},
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payment model.Payment
	if err := json.NewDecoder(rec.Body).Decode(&payment); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payment.ID != "5de6b73e-1c90-4597-84a8-2d4bf34be7f8" {
		t.Fatalf("Payment.ID = %q, want requested payment", payment.ID)
	}
}

func TestGetPaymentRejectsInvalidID(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{err: service.ErrValidation}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/payments/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetPaymentReturnsNotFound(t *testing.T) {
	router := chi.NewRouter()
	NewPaymentHandler(fakePaymentService{err: service.ErrPaymentNotFound}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/payments/5de6b73e-1c90-4597-84a8-2d4bf34be7f8", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
