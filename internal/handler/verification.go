package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type VerificationHandler struct {
	repo      *repository.VerificationRepository
	auditRepo *repository.AuditRepository
	notifier  *notifier.Service
}

func NewVerificationHandler(repo *repository.VerificationRepository, auditRepo *repository.AuditRepository, notifier *notifier.Service) *VerificationHandler {
	return &VerificationHandler{repo: repo, auditRepo: auditRepo, notifier: notifier}
}

func verificationResp(v *model.RecruiterVerification) map[string]interface{} {
	out := map[string]interface{}{
		"id": v.ID.String(), "recruiter_id": v.RecruiterID.String(),
		"bin": v.BIN, "status": v.Status, "comment": v.Comment,
		"created_at": v.CreatedAt.Format(time.RFC3339),
		"updated_at": v.UpdatedAt.Format(time.RFC3339),
	}
	if v.DocumentKey != nil {
		out["document_key"] = *v.DocumentKey
	}
	if v.ReviewedBy != nil {
		out["reviewed_by"] = v.ReviewedBy.String()
	}
	return out
}

func (h *VerificationHandler) Submit(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		BIN         string `json:"bin"`
		DocumentKey string `json:"document_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.BIN == "" {
		http.Error(w, `{"error":"bin required"}`, http.StatusBadRequest)
		return
	}
	var docKey *string
	if req.DocumentKey != "" {
		docKey = &req.DocumentKey
	}
	if err := h.repo.Upsert(r.Context(), claims.UserID, req.BIN, docKey); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	v, _ := h.repo.GetByRecruiter(r.Context(), claims.UserID)
	if v != nil {
		jsonOK(w, verificationResp(v))
		return
	}
	jsonOK(w, map[string]string{"status": "pending"})
}

func (h *VerificationHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	v, err := h.repo.GetByRecruiter(r.Context(), claims.UserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			jsonOK(w, map[string]interface{}{"status": "none"})
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, verificationResp(v))
}

func (h *VerificationHandler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.repo.List(r.Context(), status, 100)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		out = append(out, verificationResp(&list[i]))
	}
	jsonOK(w, map[string]interface{}{"verifications": out})
}

func (h *VerificationHandler) PatchAdmin(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	recruiterID, err := uuid.Parse(r.URL.Query().Get("recruiter_id"))
	if err != nil {
		http.Error(w, `{"error":"recruiter_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Status  string `json:"status"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		http.Error(w, `{"error":"status required"}`, http.StatusBadRequest)
		return
	}
	if req.Status != "approved" && req.Status != "rejected" && req.Status != "pending" {
		http.Error(w, `{"error":"invalid status"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.Review(r.Context(), recruiterID, claims.UserID, req.Status, req.Comment); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if h.notifier != nil {
		switch req.Status {
		case "approved":
			h.notifier.Notify(r.Context(), recruiterID, "verification_approved", "Компания верифицирована",
				"Ваша компания получила статус верифицированного работодателя.", map[string]interface{}{"recruiter_id": recruiterID.String()})
		case "rejected":
			body := "Заявка на верификацию отклонена."
			if req.Comment != "" {
				body += " " + req.Comment
			}
			h.notifier.Notify(r.Context(), recruiterID, "verification_rejected", "Верификация отклонена", body, map[string]interface{}{"recruiter_id": recruiterID.String()})
		}
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "review_verification", "recruiter_verification", &recruiterID, map[string]interface{}{
		"status": req.Status,
	})
	jsonOK(w, map[string]string{"status": "ok"})
}
