package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type ProfileHandler struct {
	userRepo      *repository.UserRepository
	recruiterRepo *repository.RecruiterProfileRepository
	billingSvc    *billing.Service
	notifRepo     *repository.NotificationRepository
	aesKey        []byte
}

func NewProfileHandler(userRepo *repository.UserRepository, recruiterRepo *repository.RecruiterProfileRepository, billingSvc *billing.Service, notifRepo *repository.NotificationRepository, aesKey []byte) *ProfileHandler {
	return &ProfileHandler{userRepo: userRepo, recruiterRepo: recruiterRepo, billingSvc: billingSvc, notifRepo: notifRepo, aesKey: aesKey}
}

type StudentProfileResponse struct {
	FullName           string  `json:"full_name,omitempty"`
	Phone              string  `json:"phone,omitempty"`
	Bio                string  `json:"bio,omitempty"`
	ResumeURL          *string `json:"resume_url,omitempty"`
	Skills             string  `json:"skills,omitempty"`
	Education          string  `json:"education,omitempty"`
	ExperienceYears    int     `json:"experience_years,omitempty"`
	Location           string  `json:"location,omitempty"`
	Availability       string  `json:"availability,omitempty"`
	AvailabilityStatus string  `json:"availability_status,omitempty"`
	GithubURL          string  `json:"github_url,omitempty"`
	LinkedinURL        string  `json:"linkedin_url,omitempty"`
	BehanceURL         string  `json:"behance_url,omitempty"`
	University         string  `json:"university,omitempty"`
	CourseYear         int     `json:"course_year,omitempty"`
}

type RecruiterProfileResponse struct {
	CompanyName string  `json:"company_name,omitempty"`
	FullName    string  `json:"full_name,omitempty"`
	Phone       string  `json:"phone,omitempty"`
	LogoURL     *string `json:"logo_url,omitempty"`
}

type RecruiterBillingInfo struct {
	Plan          string   `json:"plan"`
	PlanExpiresAt *string  `json:"plan_expires_at,omitempty"`
	IsPro         bool     `json:"is_pro"`
	Features      []string `json:"features"`
}

// MeResponse — ответ GET /api/me: пользователь + профиль (если есть).
type MeResponse struct {
	UserID  string      `json:"user_id"`
	Email   string      `json:"email"`
	Role    string      `json:"role"`
	Profile interface{} `json:"profile,omitempty"`
	Billing *RecruiterBillingInfo `json:"billing,omitempty"`
}

// UserByIDResponse — ответ GET /api/users?id=: id, email, role и при запросе студента — полный профиль (навыки, образование, опыт и т.д.).
type UserByIDResponse struct {
	ID      string                  `json:"id"`
	Email   string                  `json:"email"`
	Role    string                  `json:"role"`
	Profile *StudentProfileResponse `json:"profile,omitempty"`
}

func fillStudentProfileResp(h *ProfileHandler, ctx context.Context, userID uuid.UUID, resp *StudentProfileResponse) {
	p, err := h.userRepo.GetStudentProfileByUserID(ctx, userID)
	if err != nil {
		return
	}
	if len(p.FullNameEnc) > 0 {
		b, _ := crypto.Decrypt(p.FullNameEnc, h.aesKey)
		resp.FullName = string(b)
	}
	if len(p.PhoneEnc) > 0 {
		b, _ := crypto.Decrypt(p.PhoneEnc, h.aesKey)
		resp.Phone = string(b)
	}
	if len(p.BioEnc) > 0 {
		b, _ := crypto.Decrypt(p.BioEnc, h.aesKey)
		resp.Bio = string(b)
	}
	resp.ResumeURL = p.ResumeObjectKey
	resp.Skills = p.Skills
	resp.Education = p.Education
	resp.ExperienceYears = p.ExperienceYears
	resp.Location = p.Location
	resp.Availability = p.Availability
	if ext, err := h.userRepo.GetStudentProfileExtended(ctx, userID); err == nil {
		resp.AvailabilityStatus = ext.AvailabilityStatus
		resp.GithubURL = ext.GithubURL
		resp.LinkedinURL = ext.LinkedinURL
		resp.BehanceURL = ext.BehanceURL
		resp.University = ext.University
		resp.CourseYear = ext.CourseYear
	}
}

func (h *ProfileHandler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	out := MeResponse{
		UserID: claims.UserID.String(),
		Email:  claims.Email,
		Role:   string(claims.Role),
	}

	switch claims.Role {
	case model.RoleStudent:
		resp := StudentProfileResponse{}
		fillStudentProfileResp(h, r.Context(), claims.UserID, &resp)
		out.Profile = resp
	case model.RoleRecruiter:
		resp := RecruiterProfileResponse{}
		p, err := h.recruiterRepo.GetByUserID(r.Context(), claims.UserID)
		if err == nil {
			if len(p.CompanyNameEnc) > 0 {
				b, _ := crypto.Decrypt(p.CompanyNameEnc, h.aesKey)
				resp.CompanyName = string(b)
			}
			if len(p.FullNameEnc) > 0 {
				b, _ := crypto.Decrypt(p.FullNameEnc, h.aesKey)
				resp.FullName = string(b)
			}
			if len(p.PhoneEnc) > 0 {
				b, _ := crypto.Decrypt(p.PhoneEnc, h.aesKey)
				resp.Phone = string(b)
			}
			resp.LogoURL = p.CompanyLogoObjectKey
		}
		out.Profile = resp
		ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
		if err == nil {
			b := RecruiterBillingInfo{
				Plan:     string(ent.Plan),
				IsPro:    ent.IsPro,
				Features: ent.Features,
			}
			if ent.PlanExpiresAt != nil {
				s := ent.PlanExpiresAt.Format(time.RFC3339)
				b.PlanExpiresAt = &s
			}
			out.Billing = &b
		}
	case model.RoleAdmin:
		out.Profile = map[string]string{"note": "admin has no profile"}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

type UpdateStudentProfileRequest struct {
	FullName           string `json:"full_name"`
	Phone              string `json:"phone"`
	Bio                string `json:"bio"`
	Skills             string `json:"skills"`
	Education          string `json:"education"`
	ExperienceYears    int    `json:"experience_years"`
	Location           string `json:"location"`
	Availability       string `json:"availability"`
	AvailabilityStatus string `json:"availability_status"`
	GithubURL          string `json:"github_url"`
	LinkedinURL        string `json:"linkedin_url"`
	BehanceURL         string `json:"behance_url"`
	University         string `json:"university"`
	CourseYear         int    `json:"course_year"`
}

func (h *ProfileHandler) UpdateStudentProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req UpdateStudentProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	var fullNameEnc, phoneEnc, bioEnc []byte
	var err error
	if req.FullName != "" {
		fullNameEnc, err = crypto.Encrypt([]byte(req.FullName), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	if req.Phone != "" {
		phoneEnc, err = crypto.Encrypt([]byte(req.Phone), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	if req.Bio != "" {
		bioEnc, err = crypto.Encrypt([]byte(req.Bio), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	var skillsPtr, educationPtr, locationPtr, availabilityPtr *string
	var expPtr *int
	if req.Skills != "" {
		skillsPtr = &req.Skills
	}
	if req.Education != "" {
		educationPtr = &req.Education
	}
	if req.Location != "" {
		locationPtr = &req.Location
	}
	if req.Availability != "" {
		availabilityPtr = &req.Availability
	}
	if req.ExperienceYears > 0 {
		expPtr = &req.ExperienceYears
	}
	_, err = h.userRepo.GetStudentProfileByUserID(r.Context(), claims.UserID)
	if err != nil {
		_, err = h.userRepo.CreateStudentProfile(r.Context(), claims.UserID, fullNameEnc, phoneEnc, bioEnc, req.Skills, req.Education, req.Location, req.Availability, req.ExperienceYears)
	} else {
		err = h.userRepo.UpdateStudentProfile(r.Context(), claims.UserID, fullNameEnc, phoneEnc, bioEnc, nil, skillsPtr, educationPtr, locationPtr, availabilityPtr, expPtr)
	}
	if err != nil {
		http.Error(w, `{"error":"failed to save profile"}`, http.StatusInternalServerError)
		return
	}
	_ = h.userRepo.UpdateStudentProfileExtended(r.Context(), claims.UserID, repository.StudentProfileExtended{
		AvailabilityStatus: req.AvailabilityStatus, GithubURL: req.GithubURL, LinkedinURL: req.LinkedinURL,
		BehanceURL: req.BehanceURL, University: req.University, CourseYear: req.CourseYear,
	})
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type UpdateRecruiterProfileRequest struct {
	CompanyName string `json:"company_name"`
	FullName    string `json:"full_name"`
	Phone       string `json:"phone"`
}

func (h *ProfileHandler) UpdateRecruiterProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req UpdateRecruiterProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	var companyEnc, fullNameEnc, phoneEnc []byte
	var err error
	if req.CompanyName != "" {
		companyEnc, err = crypto.Encrypt([]byte(req.CompanyName), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	if req.FullName != "" {
		fullNameEnc, err = crypto.Encrypt([]byte(req.FullName), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	if req.Phone != "" {
		phoneEnc, err = crypto.Encrypt([]byte(req.Phone), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	existing, err := h.recruiterRepo.GetByUserID(r.Context(), claims.UserID)
	if err != nil {
		_, err = h.recruiterRepo.Create(r.Context(), claims.UserID, companyEnc, fullNameEnc, phoneEnc)
	} else {
		_ = existing
		err = h.recruiterRepo.Update(r.Context(), claims.UserID, companyEnc, fullNameEnc, phoneEnc)
	}
	if err != nil {
		http.Error(w, `{"error":"failed to save profile"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GetUserByID for admin or self (placeholder)
func (h *ProfileHandler) GetUserByID(w http.ResponseWriter, r *http.Request) {
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
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	// Разрешаем:
	// - администратору — любого пользователя,
	// - рекрутеру — любого пользователя (в т.ч. студента из заявки),
	// - обычному пользователю — только самого себя.
	if claims.Role != model.RoleAdmin && claims.Role != model.RoleRecruiter && claims.UserID != id {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	out := UserByIDResponse{
		ID:    user.ID.String(),
		Email: user.Email,
		Role:  string(user.Role),
	}
	if user.Role == model.RoleStudent {
		resp := StudentProfileResponse{}
		fillStudentProfileResp(h, r.Context(), id, &resp)
		out.Profile = &resp
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *ProfileHandler) GetProfileCompletion(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	percent := 0
	missing := []string{}
	p, err := h.userRepo.GetStudentProfileByUserID(r.Context(), claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(model.ProfileCompletion{Percent: 0, Missing: []string{"profile"}})
		return
	}
	if len(p.FullNameEnc) > 0 {
		percent += 10
	} else {
		missing = append(missing, "photo_or_name")
	}
	if len(p.PhoneEnc) > 0 {
		percent += 5
	} else {
		missing = append(missing, "phone")
	}
	if len(p.BioEnc) > 0 {
		percent += 10
	} else {
		missing = append(missing, "bio")
	}
	if len(strings.Split(p.Skills, ",")) >= 5 {
		percent += 10
	} else {
		missing = append(missing, "skills")
	}
	if p.Education != "" {
		percent += 10
	} else {
		missing = append(missing, "education")
	}
	if p.Location != "" {
		percent += 5
	} else {
		missing = append(missing, "location")
	}
	if p.ResumeObjectKey != nil && *p.ResumeObjectKey != "" {
		percent += 15
	} else {
		missing = append(missing, "resume")
	}
	ext, _ := h.userRepo.GetStudentProfileExtended(r.Context(), claims.UserID)
	if ext != nil {
		if ext.University != "" {
			percent += 10
		} else {
			missing = append(missing, "university")
		}
		if ext.CourseYear > 0 {
			percent += 5
		} else {
			missing = append(missing, "course_year")
		}
		if ext.GithubURL != "" || ext.LinkedinURL != "" || ext.BehanceURL != "" {
			percent += 10
		} else {
			missing = append(missing, "social_links")
		}
		if ext.AvailabilityStatus != "" {
			percent += 5
		}
	}
	if percent > 100 {
		percent = 100
	}
	badge := ""
	if percent >= 80 {
		badge = "active_candidate"
	}
	if percent >= 100 {
		badge = "complete_profile"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.ProfileCompletion{Percent: percent, Badge: badge, Missing: missing})
}

func (h *ProfileHandler) GetProfileVisibility(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	v, err := h.userRepo.GetProfileVisibility(r.Context(), claims.UserID)
	if err != nil {
		jsonOK(w, map[string]interface{}{
			"skills_visibility": "public", "education_visibility": "public",
			"experience_visibility": "public", "portfolio_visibility": "public",
			"hackathons_visibility": "public", "reviews_visibility": "public",
		})
		return
	}
	jsonOK(w, v)
}

func (h *ProfileHandler) UpdateProfileVisibility(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req model.ProfileSectionVisibility
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	req.UserID = claims.UserID
	if err := h.userRepo.UpsertProfileVisibility(r.Context(), req); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

type ActivityItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func (h *ProfileHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	limit := 10
	if h.notifRepo == nil {
		jsonOK(w, []ActivityItem{})
		return
	}
	notifs, err := h.notifRepo.ListByUser(r.Context(), claims.UserID, limit)
	if err != nil {
		jsonOK(w, []ActivityItem{})
		return
	}
	items := make([]ActivityItem, 0, len(notifs))
	for _, n := range notifs {
		items = append(items, ActivityItem{
			ID: n.ID.String(), Type: n.Type, Title: n.Title, Body: n.Body,
			CreatedAt: n.CreatedAt.Format(time.RFC3339),
		})
	}
	jsonOK(w, items)
}
