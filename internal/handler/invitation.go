package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type InvitationHandler struct {
	invRepo    *repository.InvitationRepository
	userRepo   *repository.UserRepository
	billingSvc *billing.Service
	aesKey     []byte
}

func NewInvitationHandler(invRepo *repository.InvitationRepository, userRepo *repository.UserRepository, billingSvc *billing.Service, aesKey []byte) *InvitationHandler {
	return &InvitationHandler{invRepo: invRepo, userRepo: userRepo, billingSvc: billingSvc, aesKey: aesKey}
}

type CreateInvitationRequest struct {
	StudentID string `json:"student_id"`
	Message   string `json:"message"`
}

type InvitationResponse struct {
	ID         string `json:"id"`
	RecruiterID string `json:"recruiter_id"`
	StudentID  string `json:"student_id"`
	Message   string `json:"message,omitempty"`
	Status   string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func (h *InvitationHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if !ent.CanInvite {
		billing.WriteError(w, http.StatusForbidden, "subscription_required", "invitations require Pro subscription")
		return
	}
	var req CreateInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	studentID, err := uuid.Parse(req.StudentID)
	if err != nil {
		http.Error(w, `{"error":"invalid student_id"}`, http.StatusBadRequest)
		return
	}
	user, err := h.userRepo.GetByID(r.Context(), studentID)
	if err != nil || user.Role != model.RoleStudent {
		http.Error(w, `{"error":"student not found"}`, http.StatusNotFound)
		return
	}
	exists, err := h.invRepo.Exists(r.Context(), claims.UserID, studentID)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, `{"error":"invitation already sent"}`, http.StatusConflict)
		return
	}
	var messageEnc []byte
	if req.Message != "" {
		messageEnc, err = crypto.Encrypt([]byte(req.Message), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	inv, err := h.invRepo.Create(r.Context(), claims.UserID, studentID, messageEnc)
	if err != nil {
		http.Error(w, `{"error":"failed to create invitation"}`, http.StatusInternalServerError)
		return
	}
	resp := InvitationResponse{
		ID: inv.ID.String(), RecruiterID: inv.RecruiterID.String(), StudentID: inv.StudentID.String(),
		Status: inv.Status, CreatedAt: inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if len(inv.MessageEnc) > 0 {
		b, _ := crypto.Decrypt(inv.MessageEnc, h.aesKey)
		resp.Message = string(b)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *InvitationHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var list []model.Invitation
	var err error
	if claims.Role == model.RoleStudent {
		list, err = h.invRepo.ListByStudent(r.Context(), claims.UserID)
	} else if claims.Role == model.RoleRecruiter {
		list, err = h.invRepo.ListByRecruiter(r.Context(), claims.UserID)
	} else {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]InvitationResponse, 0, len(list))
	for _, inv := range list {
		r := InvitationResponse{
			ID: inv.ID.String(), RecruiterID: inv.RecruiterID.String(), StudentID: inv.StudentID.String(),
			Status: inv.Status, CreatedAt: inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if len(inv.MessageEnc) > 0 {
			b, _ := crypto.Decrypt(inv.MessageEnc, h.aesKey)
			r.Message = string(b)
		}
		resp = append(resp, r)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type UpdateInvitationStatusRequest struct {
	Status string `json:"status"`
}

func (h *InvitationHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	inv, err := h.invRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"invitation not found"}`, http.StatusNotFound)
		return
	}
	if inv.StudentID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req UpdateInvitationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Status != "accepted" && req.Status != "declined" {
		http.Error(w, `{"error":"status must be accepted or declined"}`, http.StatusBadRequest)
		return
	}
	if err := h.invRepo.UpdateStatus(r.Context(), id, req.Status); err != nil {
		http.Error(w, `{"error":"failed to update"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": req.Status})
}
