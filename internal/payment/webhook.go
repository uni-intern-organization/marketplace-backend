package payment

import (
	"encoding/json"
	"net/http"

	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type WebhookHandler struct {
	provider PaymentProvider
	repo     *repository.PaymentRepository
}

func NewWebhookHandler(provider PaymentProvider, repo *repository.PaymentRepository) *WebhookHandler {
	return &WebhookHandler{provider: provider, repo: repo}
}

func (h *WebhookHandler) HandleCheckoutConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExternalID string `json:"external_id"`
		SessionID  string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	externalID := req.ExternalID
	if externalID == "" && req.SessionID != "" {
		sess, err := h.repo.GetSession(r.Context(), req.SessionID)
		if err != nil {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}
		if sess.ExternalID != nil {
			externalID = *sess.ExternalID
		}
	}
	if externalID == "" {
		http.Error(w, `{"error":"external_id required"}`, http.StatusBadRequest)
		return
	}
	sessionID, ok, err := h.provider.ConfirmPayment(r.Context(), externalID)
	if err != nil {
		http.Error(w, `{"error":"confirm failed"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "confirmed": ok, "session_id": sessionID.String(),
	})
}
