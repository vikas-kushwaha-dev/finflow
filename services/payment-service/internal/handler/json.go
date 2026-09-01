package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

type errorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

func WriteJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	WriteJSON(w, statusCode, errorResponse{Error: message})
}

func writeRequestError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	WriteJSON(w, statusCode, errorResponse{
		Error:     message,
		RequestID: middleware.GetReqID(r.Context()),
	})
}
