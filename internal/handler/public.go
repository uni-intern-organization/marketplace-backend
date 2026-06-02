package handler

import (
	"net/http"

	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type PublicHandler struct {
	paymentRepo *repository.PaymentRepository
}

func NewPublicHandler(paymentRepo *repository.PaymentRepository) *PublicHandler {
	return &PublicHandler{paymentRepo: paymentRepo}
}

func (h *PublicHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.paymentRepo.GetPublicStats(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"stats": stats})
}
