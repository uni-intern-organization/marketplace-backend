package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type MatchHandler struct {
	vacancyRepo *repository.VacancyRepository
	userRepo    *repository.UserRepository
	aesKey      []byte
}

func NewMatchHandler(vacancyRepo *repository.VacancyRepository, userRepo *repository.UserRepository, aesKey []byte) *MatchHandler {
	return &MatchHandler{vacancyRepo: vacancyRepo, userRepo: userRepo, aesKey: aesKey}
}

// matchScore считает балл совпадения профиля студента с вакансией.
// Только положительные компоненты; итог не меньше 0. Фронт может показывать его как процент (например score*100/maxPoints).
func matchScore(requiredSkills, studentSkills, vacLocation, studentLocation, employmentType, availability string, minExp, studentExp int) int {
	score := 0
	req := splitTrimLower(requiredSkills)
	st := splitTrimLower(studentSkills)
	for _, r := range req {
		for _, s := range st {
			if r == s {
				score += 10
				break
			}
		}
	}
	if vacLocation != "" && studentLocation != "" && strings.EqualFold(strings.TrimSpace(vacLocation), strings.TrimSpace(studentLocation)) {
		score += 5
	}
	if employmentType != "" && availability != "" && strings.EqualFold(strings.TrimSpace(employmentType), strings.TrimSpace(availability)) {
		score += 5
	}
	if minExp > 0 {
		if studentExp >= minExp {
			score += 5
		}
		// при недостаточном опыте не штрафуем — просто 0 за этот критерий
	} else {
		// вакансия без требования опыта — считаем нейтральным совпадением по опыту
		score += 5
	}
	if score < 0 {
		return 0
	}
	return score
}

func splitTrimLower(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		t := strings.ToLower(strings.TrimSpace(p))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

type CandidateResponse struct {
	UserID          string `json:"user_id"`
	Email           string `json:"email"`
	Skills          string `json:"skills"`
	Education       string `json:"education"`
	ExperienceYears int    `json:"experience_years"`
	Location        string `json:"location"`
	Availability    string `json:"availability"`
	MatchScore      int    `json:"match_score"`
}

func (h *MatchHandler) CandidatesForVacancy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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
	vacID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	vac, err := h.vacancyRepo.GetByID(r.Context(), vacID)
	if err != nil {
		http.Error(w, `{"error":"vacancy not found"}`, http.StatusNotFound)
		return
	}
	profiles, err := h.userRepo.ListStudentProfilesForMatching(r.Context())
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	type scored struct {
		cand  CandidateResponse
		score int
	}
	var scoredList []scored
	for _, p := range profiles {
		user, err := h.userRepo.GetByID(r.Context(), p.UserID)
		if err != nil {
			continue
		}
		score := matchScore(vac.RequiredSkills, p.Skills, vac.Location, p.Location, vac.EmploymentType, p.Availability, vac.MinExperienceYears, p.ExperienceYears)
		scoredList = append(scoredList, scored{
			cand: CandidateResponse{
				UserID:          p.UserID.String(),
				Email:           user.Email,
				Skills:          p.Skills,
				Education:       p.Education,
				ExperienceYears: p.ExperienceYears,
				Location:        p.Location,
				Availability:    p.Availability,
				MatchScore:      score,
			},
			score: score,
		})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	resp := make([]CandidateResponse, 0, len(scoredList))
	for _, s := range scoredList {
		resp = append(resp, s.cand)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type VacancyWithScore struct {
	VacancyResponse
	MatchScore int `json:"match_score"`
}

func (h *MatchHandler) RecommendationsForStudent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	profile, err := h.userRepo.GetStudentProfileByUserID(r.Context(), claims.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]VacancyWithScore{})
		return
	}
	list, err := h.vacancyRepo.List(r.Context(), repository.VacancyFilter{}, 100)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	type scored struct {
		v     model.Vacancy
		score int
	}
	var scoredList []scored
	for _, v := range list {
		score := matchScore(v.RequiredSkills, profile.Skills, v.Location, profile.Location, v.EmploymentType, profile.Availability, v.MinExperienceYears, profile.ExperienceYears)
		scoredList = append(scoredList, scored{v: v, score: score})
	}
	sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].score > scoredList[j].score })
	resp := make([]VacancyWithScore, 0, len(scoredList))
	for _, s := range scoredList {
		resp = append(resp, VacancyWithScore{
			VacancyResponse: vacancyToResponse(&s.v, h.aesKey),
			MatchScore:      s.score,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
