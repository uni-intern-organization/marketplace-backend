package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type ApplicationHandler struct {
	appRepo  *repository.ApplicationRepository
	invRepo  *repository.InvitationRepository
	userRepo *repository.UserRepository
	aesKey   []byte
}

func NewApplicationHandler(appRepo *repository.ApplicationRepository, invRepo *repository.InvitationRepository, userRepo *repository.UserRepository, aesKey []byte) *ApplicationHandler {
	return &ApplicationHandler{appRepo: appRepo, invRepo: invRepo, userRepo: userRepo, aesKey: aesKey}
}

type CreateApplicationRequest struct {
	RecruiterID string `json:"recruiter_id"`
	InvitationID string `json:"invitation_id,omitempty"`
	CoverLetter string `json:"cover_letter,omitempty"`
}

type ApplicationResponse struct {
	ID          string  `json:"id"`
	StudentID   string  `json:"student_id"`
	RecruiterID string  `json:"recruiter_id"`
	CoverLetter string  `json:"cover_letter,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	recruiterID, err := uuid.Parse(req.RecruiterID)
	if err != nil {
		http.Error(w, `{"error":"invalid recruiter_id"}`, http.StatusBadRequest)
		return
	}
	_, err = h.userRepo.GetByID(r.Context(), recruiterID)
	if err != nil {
		http.Error(w, `{"error":"recruiter not found"}`, http.StatusNotFound)
		return
	}
	var invitationID *uuid.UUID
	if req.InvitationID != "" {
		id, err := uuid.Parse(req.InvitationID)
		if err != nil {
			http.Error(w, `{"error":"invalid invitation_id"}`, http.StatusBadRequest)
			return
		}
		invitationID = &id
	}
	var coverLetterEnc []byte
	if req.CoverLetter != "" {
		coverLetterEnc, err = crypto.Encrypt([]byte(req.CoverLetter), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	app, err := h.appRepo.Create(r.Context(), claims.UserID, recruiterID, invitationID, coverLetterEnc)
	if err != nil {
		http.Error(w, `{"error":"failed to create application"}`, http.StatusInternalServerError)
		return
	}
	resp := ApplicationResponse{
		ID: app.ID.String(), StudentID: app.StudentID.String(), RecruiterID: app.RecruiterID.String(),
		Status: app.Status, CreatedAt: app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if len(app.CoverLetterEnc) > 0 {
		b, _ := crypto.Decrypt(app.CoverLetterEnc, h.aesKey)
		resp.CoverLetter = string(b)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ApplicationHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var list []model.Application
	var err error
	if claims.Role == model.RoleStudent {
		list, err = h.appRepo.ListByStudent(r.Context(), claims.UserID)
	} else if claims.Role == model.RoleRecruiter {
		list, err = h.appRepo.ListByRecruiter(r.Context(), claims.UserID)
	} else {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]ApplicationResponse, 0, len(list))
	for _, app := range list {
		r := ApplicationResponse{
			ID: app.ID.String(), StudentID: app.StudentID.String(), RecruiterID: app.RecruiterID.String(),
			Status: app.Status, CreatedAt: app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if len(app.CoverLetterEnc) > 0 {
			b, _ := crypto.Decrypt(app.CoverLetterEnc, h.aesKey)
			r.CoverLetter = string(b)
		}
		resp = append(resp, r)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type UpdateApplicationStatusRequest struct {
	Status string `json:"status"`
}

func (h *ApplicationHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
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
	app, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"application not found"}`, http.StatusNotFound)
		return
	}
	if app.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req UpdateApplicationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Status != "viewed" && req.Status != "accepted" && req.Status != "rejected" {
		http.Error(w, `{"error":"status must be viewed, accepted or rejected"}`, http.StatusBadRequest)
		return
	}
	if err := h.appRepo.UpdateStatus(r.Context(), id, req.Status); err != nil {
		http.Error(w, `{"error":"failed to update"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": req.Status})
}
