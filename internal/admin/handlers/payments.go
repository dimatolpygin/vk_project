package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"vk_neuro_bot/internal/bot/flows"
)

type PaymentsHandler struct {
	deps *flows.Deps
}

func NewPaymentsHandler(deps *flows.Deps) *PaymentsHandler {
	return &PaymentsHandler{deps: deps}
}

func (h *PaymentsHandler) Settle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PaymentID string `json:"payment_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.PaymentID = strings.TrimSpace(req.PaymentID)
	if req.PaymentID == "" {
		http.Error(w, "payment_id is required", http.StatusBadRequest)
		return
	}

	log.Info().Str("payment_id", req.PaymentID).Msg("admin: manual payment settle requested")

	if err := flows.ProcessSuccessfulPaymentWithMetadata(r.Context(), h.deps, req.PaymentID, nil); err != nil {
		log.Error().Err(err).Str("payment_id", req.PaymentID).Msg("admin: manual settle failed")
		http.Error(w, "settle failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info().Str("payment_id", req.PaymentID).Msg("admin: manual settle completed")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":         true,
		"payment_id": req.PaymentID,
	})
}
