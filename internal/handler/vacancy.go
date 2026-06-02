package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type VacancyHandler struct {
	vacancyRepo *repository.VacancyRepository
	userRepo    *repository.UserRepository
	billingSvc  *billing.Service
	aesKey      []byte
}

func NewVacancyHandler(vacancyRepo *repository.VacancyRepository, userRepo *repository.UserRepository, billingSvc *billing.Service, aesKey []byte) *VacancyHandler {
	return &VacancyHandler{vacancyRepo: vacancyRepo, userRepo: userRepo, billingSvc: billingSvc, aesKey: aesKey}
}

type CreateVacancyRequest struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	CompanyName        string `json:"company_name"`
	RequiredSkills     string `json:"required_skills"`
	Location           string `json:"location"`
	EmploymentType     string `json:"employment_type"`
	MinExperienceYears int    `json:"min_experience_years"`
	ListingTier        string `json:"listing_tier"`
}

type VacancyResponse struct {
	ID                 string  `json:"id"`
	RecruiterID        string  `json:"recruiter_id"`
	Title              string  `json:"title"`
	Description        string  `json:"description"`
	CompanyName        string  `json:"company_name"`
	RequiredSkills     string  `json:"required_skills"`
	Location           string  `json:"location"`
	EmploymentType     string  `json:"employment_type"`
	MinExperienceYears int     `json:"min_experience_years"`
	ListingTier        string  `json:"listing_tier"`
	Status             string  `json:"status"`
	ExpiresAt          *string `json:"expires_at,omitempty"`
	IsFeatured         bool    `json:"is_featured"`
	FeaturedUntil      *string `json:"featured_until,omitempty"`
	CreatedAt          string  `json:"created_at"`
}

func vacancyToResponse(v *model.Vacancy, aesKey []byte) VacancyResponse {
	resp := VacancyResponse{
		ID:                 v.ID.String(),
		RecruiterID:        v.RecruiterID.String(),
		CompanyName:        v.CompanyName,
		RequiredSkills:     v.RequiredSkills,
		Location:           v.Location,
		EmploymentType:     v.EmploymentType,
		MinExperienceYears: v.MinExperienceYears,
		ListingTier:        string(v.ListingTier),
		Status:             string(v.Status),
		CreatedAt:          v.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if v.ExpiresAt != nil {
		s := v.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &s
	}
	if len(v.TitleEnc) > 0 {
		b, _ := crypto.Decrypt(v.TitleEnc, aesKey)
		resp.Title = string(b)
	}
	if len(v.DescriptionEnc) > 0 {
		b, _ := crypto.Decrypt(v.DescriptionEnc, aesKey)
		resp.Description = string(b)
	}
	// Вручную вставлённые в БД записи с пустым title_enc: показываем компанию, чтобы список не был «пустым»
	if resp.Title == "" && v.CompanyName != "" {
		resp.Title = v.CompanyName
	}
	now := time.Now()
	resp.IsFeatured = billing.IsFeaturedActive(v, now)
	if v.FeaturedUntil != nil && resp.IsFeatured {
		s := v.FeaturedUntil.Format(time.RFC3339)
		resp.FeaturedUntil = &s
	}
	return resp
}

func (h *VacancyHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req CreateVacancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
		return
	}
	if req.CompanyName == "" {
		http.Error(w, `{"error":"company_name required"}`, http.StatusBadRequest)
		return
	}
	titleEnc, err := crypto.Encrypt([]byte(req.Title), h.aesKey)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	descEnc, _ := crypto.Encrypt([]byte(req.Description), h.aesKey)
	tier := model.ListingTierBasic
	switch req.ListingTier {
	case "standard":
		tier = model.ListingTierStandard
	case "premium":
		tier = model.ListingTierPremium
	}
	v, err := h.vacancyRepo.Create(r.Context(), claims.UserID, titleEnc, descEnc, req.CompanyName, req.RequiredSkills, req.Location, req.EmploymentType, req.MinExperienceYears, tier)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed to create vacancy", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vacancyToResponse(v, h.aesKey))
}

func (h *VacancyHandler) GetOrList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Query().Get("id") != "" {
		h.Get(w, r)
		return
	}
	h.List(w, r)
}

func (h *VacancyHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	_ = middleware.GetClaims(r.Context())
	filter := repository.VacancyFilter{
		Query:          r.URL.Query().Get("q"),
		Skills:         r.URL.Query().Get("skills"),
		Location:       r.URL.Query().Get("location"),
		EmploymentType: r.URL.Query().Get("employment_type"),
		Tier:           r.URL.Query().Get("tier"),
		PremiumOnly:    r.URL.Query().Get("premium") == "true",
	}
	if s := r.URL.Query().Get("min_experience_years"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			filter.MinExperienceYears = &n
		}
	}
	if s := r.URL.Query().Get("created_after"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			filter.CreatedAfter = &t
		} else if t, err := time.Parse(time.RFC3339, s); err == nil {
			filter.CreatedAfter = &t
		}
	}
	if s := r.URL.Query().Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	list, err := h.vacancyRepo.List(r.Context(), filter, limit)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	resp := make([]VacancyResponse, 0, len(list))
	for i := range list {
		resp = append(resp, vacancyToResponse(&list[i], h.aesKey))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *VacancyHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
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
	v, err := h.vacancyRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"vacancy not found"}`, http.StatusNotFound)
		return
	}
	claims := middleware.GetClaims(r.Context())
	var viewerID *uuid.UUID
	if claims != nil {
		viewerID = &claims.UserID
	}
	_ = h.vacancyRepo.RecordView(r.Context(), id, viewerID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vacancyToResponse(v, h.aesKey))
}

func (h *VacancyHandler) Renew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	var req struct{ ListingTier string `json:"listing_tier"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	tier := model.ListingTierBasic
	switch req.ListingTier {
	case "standard":
		tier = model.ListingTierStandard
	case "premium":
		tier = model.ListingTierPremium
	}
	if err := h.vacancyRepo.Renew(r.Context(), id, claims.UserID, tier); err != nil {
		http.Error(w, `{"error":"renew failed"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *VacancyHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	list, err := h.vacancyRepo.ListByRecruiter(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"list failed"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]VacancyResponse, 0, len(list))
	for i := range list {
		resp = append(resp, vacancyToResponse(&list[i], h.aesKey))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type UpdateVacancyRequest struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	CompanyName        string `json:"company_name"`
	RequiredSkills     string `json:"required_skills"`
	Location           string `json:"location"`
	EmploymentType     string `json:"employment_type"`
	MinExperienceYears int    `json:"min_experience_years"`
}

func (h *VacancyHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
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
	var req UpdateVacancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	titleEnc, _ := crypto.Encrypt([]byte(req.Title), h.aesKey)
	descEnc, _ := crypto.Encrypt([]byte(req.Description), h.aesKey)
	if err := h.vacancyRepo.Update(r.Context(), id, claims.UserID, titleEnc, descEnc, req.CompanyName, req.RequiredSkills, req.Location, req.EmploymentType, req.MinExperienceYears); err != nil {
		http.Error(w, `{"error":"vacancy not found or forbidden"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *VacancyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
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
	if err := h.vacancyRepo.Delete(r.Context(), id, claims.UserID); err != nil {
		http.Error(w, `{"error":"vacancy not found or forbidden"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
