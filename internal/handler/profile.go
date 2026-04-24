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

type ProfileHandler struct {
	userRepo      *repository.UserRepository
	recruiterRepo *repository.RecruiterProfileRepository
	aesKey        []byte
}

func NewProfileHandler(userRepo *repository.UserRepository, recruiterRepo *repository.RecruiterProfileRepository, aesKey []byte) *ProfileHandler {
	return &ProfileHandler{userRepo: userRepo, recruiterRepo: recruiterRepo, aesKey: aesKey}
}

type StudentProfileResponse struct {
	FullName        string  `json:"full_name,omitempty"`
	Phone           string  `json:"phone,omitempty"`
	Bio             string  `json:"bio,omitempty"`
	ResumeURL       *string `json:"resume_url,omitempty"`
	Skills          string  `json:"skills,omitempty"`
	Education       string  `json:"education,omitempty"`
	ExperienceYears int     `json:"experience_years,omitempty"`
	Location        string  `json:"location,omitempty"`
	Availability    string  `json:"availability,omitempty"`
}

type RecruiterProfileResponse struct {
	CompanyName string  `json:"company_name,omitempty"`
	FullName    string  `json:"full_name,omitempty"`
	Phone       string  `json:"phone,omitempty"`
	LogoURL     *string `json:"logo_url,omitempty"`
}

// MeResponse — ответ GET /api/me: пользователь + профиль (если есть).
type MeResponse struct {
	UserID  string      `json:"user_id"`
	Email   string      `json:"email"`
	Role    string      `json:"role"`
	Profile interface{} `json:"profile,omitempty"`
}

// UserByIDResponse — ответ GET /api/users?id=: id, email, role и при запросе студента — полный профиль (навыки, образование, опыт и т.д.).
type UserByIDResponse struct {
	ID      string                  `json:"id"`
	Email   string                  `json:"email"`
	Role    string                  `json:"role"`
	Profile *StudentProfileResponse `json:"profile,omitempty"`
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
		p, err := h.userRepo.GetStudentProfileByUserID(r.Context(), claims.UserID)
		if err == nil {
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
		}
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
	case model.RoleAdmin:
		out.Profile = map[string]string{"note": "admin has no profile"}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

type UpdateStudentProfileRequest struct {
	FullName        string `json:"full_name"`
	Phone           string `json:"phone"`
	Bio             string `json:"bio"`
	Skills          string `json:"skills"`
	Education       string `json:"education"`
	ExperienceYears int    `json:"experience_years"`
	Location        string `json:"location"`
	Availability    string `json:"availability"`
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
		p, err := h.userRepo.GetStudentProfileByUserID(r.Context(), id)
		if err == nil {
			resp := StudentProfileResponse{}
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
			out.Profile = &resp
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
