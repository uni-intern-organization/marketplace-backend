package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type VacancyReviewHandler struct {
	repo *repository.VacancyReviewRepository
}

type vacancyReviewResponse struct {
	ID           string `json:"id"`
	VacancyID    string `json:"vacancy_id"`
	AuthorUserID string `json:"author_user_id"`
	AuthorName   string `json:"author_name"`
	AuthorRole   string `json:"author_role"`
	Text         string `json:"text"`
	CreatedAt    string `json:"created_at"`
}

func NewVacancyReviewHandler(repo *repository.VacancyReviewRepository) *VacancyReviewHandler {
	return &VacancyReviewHandler{repo: repo}
}

func vacancyReviewToResponse(review repository.VacancyReview) vacancyReviewResponse {
	return vacancyReviewResponse{
		ID: review.ID.String(), VacancyID: review.VacancyID.String(), AuthorUserID: review.AuthorUserID.String(),
		AuthorName: review.AuthorName, AuthorRole: review.AuthorRole, Text: review.Text, CreatedAt: review.CreatedAt.Format(time.RFC3339),
	}
}

func (h *VacancyReviewHandler) List(w http.ResponseWriter, r *http.Request) {
	vacancyID, err := uuid.Parse(r.URL.Query().Get("vacancy_id"))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid vacancy_id", err)
		return
	}
	list, err := h.repo.List(r.Context(), vacancyID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed to list reviews", err)
		return
	}
	out := make([]vacancyReviewResponse, 0, len(list))
	for _, review := range list {
		out = append(out, vacancyReviewToResponse(review))
	}
	jsonOK(w, out)
}

func (h *VacancyReviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		RespondError(w, http.StatusForbidden, "forbidden", nil)
		return
	}
	var req struct {
		VacancyID string `json:"vacancy_id"`
		Text      string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid body", err)
		return
	}
	vacancyID, err := uuid.Parse(req.VacancyID)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "invalid vacancy_id", err)
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if len([]rune(req.Text)) < 10 || len([]rune(req.Text)) > 2000 {
		RespondError(w, http.StatusBadRequest, "review must be between 10 and 2000 characters", nil)
		return
	}
	verified, err := h.repo.HasCompletedInternship(r.Context(), claims.UserID, vacancyID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed to verify internship", err)
		return
	}
	if !verified {
		RespondError(w, http.StatusForbidden, "INTERNSHIP_NOT_VERIFIED", nil)
		return
	}
	review, err := h.repo.Create(r.Context(), vacancyID, claims.UserID, req.Text)
	if err != nil {
		RespondError(w, http.StatusConflict, "review already exists", err)
		return
	}
	review.AuthorName = "Студент Steppy"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(vacancyReviewToResponse(*review))
}
