package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
)

type fakeLedgerReader struct {
	balances []model.Balance
	entries  []model.Entry
	err      error
}

func (r fakeLedgerReader) ListBalances(ctx context.Context) ([]model.Balance, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.balances, nil
}

func (r fakeLedgerReader) ListEntriesByPaymentID(ctx context.Context, paymentID string) ([]model.Entry, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.entries, nil
}

func TestListBalances(t *testing.T) {
	router := chi.NewRouter()
	handler := NewLedgerHandler(fakeLedgerReader{
		balances: []model.Balance{
			{
				AccountID:   "account-1",
				AccountName: "finflow_cash",
				Currency:    "USD",
				AmountCents: 1299,
				AsOf:        time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC),
			},
		},
	})
	router.Route("/api/v1", handler.RegisterRoutes)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/balances", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, response.Code)
	}

	var body struct {
		Balances []model.Balance `json:"balances"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Balances) != 1 || body.Balances[0].AccountName != "finflow_cash" {
		t.Fatalf("unexpected balances response: %+v", body.Balances)
	}
}

func TestListPaymentEntries(t *testing.T) {
	router := chi.NewRouter()
	handler := NewLedgerHandler(fakeLedgerReader{
		entries: []model.Entry{
			{
				ID:            "entry-1",
				TransactionID: "transaction-1",
				AccountID:     "account-1",
				Direction:     model.DirectionDebit,
				AmountCents:   1299,
				Currency:      "USD",
				ReferenceType: "payment",
				ReferenceID:   "payment-1",
			},
		},
	})
	router.Route("/api/v1", handler.RegisterRoutes)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ledger/payments/payment-1/entries", nil)

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, response.Code)
	}

	var body struct {
		Entries []model.Entry `json:"entries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Entries) != 1 || body.Entries[0].ReferenceID != "payment-1" {
		t.Fatalf("unexpected entries response: %+v", body.Entries)
	}
}
