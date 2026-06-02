package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type VacancyExtHandler struct {
	vacancyRepo *repository.VacancyRepository
	aesKey      []byte
}

func NewVacancyExtHandler(vacancyRepo *repository.VacancyRepository, aesKey []byte) *VacancyExtHandler {
	return &VacancyExtHandler{vacancyRepo: vacancyRepo, aesKey: aesKey}
}

func (h *VacancyExtHandler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	vacancyID, err := uuid.Parse(r.URL.Query().Get("vacancy_id"))
	if err != nil {
		http.Error(w, `{"error":"vacancy_id required"}`, http.StatusBadRequest)
		return
	}
	if err := h.vacancyRepo.AddFavorite(r.Context(), claims.UserID, vacancyID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *VacancyExtHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	vacancyID, err := uuid.Parse(r.URL.Query().Get("vacancy_id"))
	if err != nil {
		http.Error(w, `{"error":"vacancy_id required"}`, http.StatusBadRequest)
		return
	}
	if err := h.vacancyRepo.RemoveFavorite(r.Context(), claims.UserID, vacancyID); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *VacancyExtHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	list, err := h.vacancyRepo.ListFavorites(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]VacancyResponse, 0, len(list))
	for i := range list {
		resp = append(resp, vacancyToResponse(&list[i], h.aesKey))
	}
	jsonOK(w, map[string]interface{}{"vacancies": resp})
}

func (h *VacancyExtHandler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Title               string  `json:"title"`
		Description         string  `json:"description"`
		CompanyName         string  `json:"company_name"`
		RequiredSkills      string  `json:"required_skills"`
		Location            string  `json:"location"`
		EmploymentType      string  `json:"employment_type"`
		MinExperienceYears  int     `json:"min_experience_years"`
		ListingTier         string  `json:"listing_tier"`
		VacancyType         string  `json:"vacancy_type"`
		Responsibilities    string  `json:"responsibilities"`
		Requirements        string  `json:"requirements"`
		Offers              string  `json:"offers"`
		SalaryType          string  `json:"salary_type"`
		SalaryMin           *int    `json:"salary_min"`
		SalaryMax           *int    `json:"salary_max"`
		DurationMonths      *int    `json:"duration_months"`
		ApplicationDeadline string  `json:"application_deadline"`
		ContactName         string  `json:"contact_name"`
		ContactEmail        string  `json:"contact_email"`
		Specialty           string  `json:"specialty"`
		DesiredStartDate    string  `json:"desired_start_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	titleEnc, _ := crypto.Encrypt([]byte(req.Title), h.aesKey)
	descEnc, _ := crypto.Encrypt([]byte(req.Description), h.aesKey)
	respEnc, _ := crypto.Encrypt([]byte(req.Responsibilities), h.aesKey)
	reqEnc, _ := crypto.Encrypt([]byte(req.Requirements), h.aesKey)
	offEnc, _ := crypto.Encrypt([]byte(req.Offers), h.aesKey)
	contactEnc, _ := crypto.Encrypt([]byte(req.ContactName), h.aesKey)
	tier := model.ListingTierBasic
	switch req.ListingTier {
	case "standard":
		tier = model.ListingTierStandard
	case "premium":
		tier = model.ListingTierPremium
	}
	if req.VacancyType == "" {
		req.VacancyType = "vacancy"
	}
	if req.SalaryType == "" {
		req.SalaryType = "negotiable"
	}
	var appDeadline, startDate *time.Time
	if req.ApplicationDeadline != "" {
		if t, err := time.Parse(time.RFC3339, req.ApplicationDeadline); err == nil {
			appDeadline = &t
		}
	}
	if req.DesiredStartDate != "" {
		if t, err := time.Parse("2006-01-02", req.DesiredStartDate); err == nil {
			startDate = &t
		}
	}
	v, err := h.vacancyRepo.SaveDraft(r.Context(), claims.UserID, repository.VacancyDraftInput{
		TitleEnc: titleEnc, DescriptionEnc: descEnc, CompanyName: req.CompanyName,
		RequiredSkills: req.RequiredSkills, Location: req.Location, EmploymentType: req.EmploymentType,
		MinExperienceYears: req.MinExperienceYears, ListingTier: tier, VacancyType: req.VacancyType,
		ResponsibilitiesEnc: respEnc, RequirementsEnc: reqEnc, OffersEnc: offEnc,
		SalaryType: req.SalaryType, SalaryMin: req.SalaryMin, SalaryMax: req.SalaryMax,
		DurationMonths: req.DurationMonths, ApplicationDeadline: appDeadline,
		ContactNameEnc: contactEnc, ContactEmail: req.ContactEmail, Specialty: req.Specialty,
		DesiredStartDate: startDate,
	})
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "save draft failed", err)
		return
	}
	jsonOK(w, vacancyToResponse(v, h.aesKey))
}
