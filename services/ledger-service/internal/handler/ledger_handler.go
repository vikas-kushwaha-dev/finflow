package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/model"
	"github.com/vikas-kushwaha-dev/finflow/services/ledger-service/internal/service"
)

type ledgerReader interface {
	ListBalances(ctx context.Context) ([]model.Balance, error)
	ListEntriesByPaymentID(ctx context.Context, paymentID string) ([]model.Entry, error)
}

type LedgerHandler struct {
	service ledgerReader
}

func NewLedgerHandler(service ledgerReader) *LedgerHandler {
	return &LedgerHandler{service: service}
}

func (h *LedgerHandler) RegisterRoutes(router chi.Router) {
	router.Get("/ledger/balances", h.listBalances)
	router.Get("/ledger/payments/{payment_id}/entries", h.listPaymentEntries)
}

func (h *LedgerHandler) listBalances(w http.ResponseWriter, r *http.Request) {
	balances, err := h.service.ListBalances(r.Context())
	if err != nil {
		writeRequestError(w, r, http.StatusInternalServerError, "could not list ledger balances")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"balances": balances})
}

func (h *LedgerHandler) listPaymentEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := h.service.ListEntriesByPaymentID(r.Context(), chi.URLParam(r, "payment_id"))
	if err != nil {
		if errors.Is(err, service.ErrValidation) {
			writeRequestError(w, r, http.StatusBadRequest, "invalid payment id")
			return
		}

		writeRequestError(w, r, http.StatusInternalServerError, "could not list ledger entries")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
