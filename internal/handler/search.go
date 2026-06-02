package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type SearchHandler struct {
	pool          *pgxpool.Pool
	userRepo      *repository.UserRepository
	billingSvc    *billing.Service
	vacancyRepo   *repository.VacancyRepository
	freelanceRepo *repository.FreelanceRepository
	hackathonRepo *repository.HackathonRepository
	aesKey        []byte
}

func NewSearchHandler(
	pool *pgxpool.Pool,
	userRepo *repository.UserRepository,
	billingSvc *billing.Service,
	vacancyRepo *repository.VacancyRepository,
	freelanceRepo *repository.FreelanceRepository,
	hackathonRepo *repository.HackathonRepository,
	aesKey []byte,
) *SearchHandler {
	return &SearchHandler{
		pool: pool, userRepo: userRepo, billingSvc: billingSvc,
		vacancyRepo: vacancyRepo, freelanceRepo: freelanceRepo, hackathonRepo: hackathonRepo,
		aesKey: aesKey,
	}
}

// SearchUsers returns users by role and optional email prefix (admin only or recruiter for students).
func (h *SearchHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	role := r.URL.Query().Get("role")
	emailPrefix := r.URL.Query().Get("email")
	if role != string(model.RoleStudent) && role != string(model.RoleRecruiter) {
		role = ""
	}
	// Only admin can search all; recruiter can search students
	if claims.Role != model.RoleAdmin && claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if claims.Role == model.RoleRecruiter && role != string(model.RoleStudent) {
		role = string(model.RoleStudent)
	}
	query := `SELECT id, email, role FROM users WHERE 1=1`
	args := []interface{}{}
	argNum := 1
	if role != "" {
		query += ` AND role = ` + fmt.Sprintf("$%d", argNum)
		args = append(args, role)
		argNum++
	}
	if emailPrefix != "" {
		query += ` AND email LIKE ` + fmt.Sprintf("$%d", argNum)
		args = append(args, emailPrefix+"%")
		argNum++
	}
	query += ` ORDER BY email LIMIT 50`
	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		http.Error(w, `{"error":"search failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type row struct {
		ID    string         `json:"id"`
		Email string         `json:"email"`
		Role  model.UserRole `json:"role"`
	}
	var results []row
	for rows.Next() {
		var id uuid.UUID
		var email string
		var roleVal model.UserRole
		if err := rows.Scan(&id, &email, &roleVal); err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		results = append(results, row{ID: id.String(), Email: email, Role: roleVal})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// SearchStudents returns students with profiles, filtered by skills, experience_min, location, education (recruiter/admin).
func (h *SearchHandler) SearchStudents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if claims.Role != model.RoleRecruiter && claims.Role != model.RoleAdmin {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if !ent.CanSearch {
		billing.WriteError(w, http.StatusForbidden, "subscription_required", "student search requires Pro subscription")
		return
	}
	profiles, err := h.userRepo.ListStudentProfilesForMatching(r.Context())
	if err != nil {
		http.Error(w, `{"error":"search failed"}`, http.StatusInternalServerError)
		return
	}
	skillsFilter := r.URL.Query().Get("skills")       // comma-separated
	locationFilter := r.URL.Query().Get("location")   // substring
	educationFilter := r.URL.Query().Get("education") // substring
	expMin := -1
	if s := r.URL.Query().Get("experience_min"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			expMin = n
		}
	}
	type studentRow struct {
		ID              string `json:"id"`
		Email           string `json:"email"`
		Skills          string `json:"skills"`
		Education       string `json:"education"`
		ExperienceYears int    `json:"experience_years"`
		Location        string `json:"location"`
		Availability    string `json:"availability"`
	}
	var results []studentRow
	skillSet := splitTrimLower(skillsFilter)
	for _, p := range profiles {
		if expMin >= 0 && p.ExperienceYears < expMin {
			continue
		}
		if locationFilter != "" && !strings.Contains(strings.ToLower(p.Location), strings.ToLower(locationFilter)) {
			continue
		}
		if educationFilter != "" && !strings.Contains(strings.ToLower(p.Education), strings.ToLower(educationFilter)) {
			continue
		}
		if len(skillSet) > 0 {
			studentSkills := splitTrimLower(p.Skills)
			matched := false
			for _, sk := range skillSet {
				for _, st := range studentSkills {
					if sk == st {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		user, err := h.userRepo.GetByID(r.Context(), p.UserID)
		if err != nil {
			continue
		}
		results = append(results, studentRow{
			ID:              p.UserID.String(),
			Email:           user.Email,
			Skills:          p.Skills,
			Education:       p.Education,
			ExperienceYears: p.ExperienceYears,
			Location:        p.Location,
			Availability:    p.Availability,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// GetStudentByID returns one student's profile by user ID (recruiter/admin).
// GET /api/students/{id} — для страницы «Профиль кандидата» из заявок.
func (h *SearchHandler) GetStudentByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if claims.Role != model.RoleRecruiter && claims.Role != model.RoleAdmin {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if !ent.CanSearch {
		billing.WriteError(w, http.StatusForbidden, "subscription_required", "student profiles require Pro subscription")
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	user, err := h.userRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}
	if user.Role != model.RoleStudent {
		http.Error(w, `{"error":"not a student"}`, http.StatusNotFound)
		return
	}
	type studentRow struct {
		ID              string `json:"id"`
		Email           string `json:"email"`
		Skills          string `json:"skills"`
		Education       string `json:"education"`
		ExperienceYears int    `json:"experience_years"`
		Location        string `json:"location"`
		Availability    string `json:"availability"`
	}
	out := studentRow{
		ID:              user.ID.String(),
		Email:           user.Email,
		Skills:          "",
		Education:       "",
		ExperienceYears: 0,
		Location:        "",
		Availability:    "",
	}
	profile, err := h.userRepo.GetStudentProfileByUserID(r.Context(), id)
	if err == nil {
		out.Skills = profile.Skills
		out.Education = profile.Education
		out.ExperienceYears = profile.ExperienceYears
		out.Location = profile.Location
		out.Availability = profile.Availability
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

type GlobalSearchItem struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Subtitle    string `json:"subtitle,omitempty"`
}

type GlobalSearchResponse struct {
	Vacancies  []GlobalSearchItem `json:"vacancies"`
	Freelance  []GlobalSearchItem `json:"freelance"`
	Hackathons []GlobalSearchItem `json:"hackathons"`
	Companies  []GlobalSearchItem `json:"companies"`
}

// GlobalSearch unified catalog search for header overlay (public).
func (h *SearchHandler) GlobalSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 3
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 10 {
			limit = n
		}
	}
	resp := GlobalSearchResponse{
		Vacancies:  []GlobalSearchItem{},
		Freelance:  []GlobalSearchItem{},
		Hackathons: []GlobalSearchItem{},
		Companies:  []GlobalSearchItem{},
	}
	if q == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}
	if h.vacancyRepo != nil {
		vacList, _ := h.vacancyRepo.List(r.Context(), repository.VacancyFilter{Query: q}, limit)
		seenCompanies := map[string]bool{}
		for _, v := range vacList {
			vr := vacancyToResponse(&v, h.aesKey)
			resp.Vacancies = append(resp.Vacancies, GlobalSearchItem{
				Type: "vacancy", ID: vr.ID, Title: vr.Title,
				Description: vr.CompanyName, Subtitle: vr.Location,
			})
			if v.CompanyName != "" && !seenCompanies[v.CompanyName] {
				seenCompanies[v.CompanyName] = true
				resp.Companies = append(resp.Companies, GlobalSearchItem{
					Type: "company", ID: v.RecruiterID.String(), Title: v.CompanyName,
				})
			}
		}
	}
	if h.freelanceRepo != nil {
		tasks, _ := h.freelanceRepo.ListOpen(r.Context(), "", 50)
		count := 0
		for _, t := range tasks {
			if count >= limit {
				break
			}
			title := ""
			if len(t.TitleEnc) > 0 {
				if b, err := crypto.Decrypt(t.TitleEnc, h.aesKey); err == nil {
					title = string(b)
				}
			}
			if title == "" || !strings.Contains(strings.ToLower(title), strings.ToLower(q)) {
				if !strings.Contains(strings.ToLower(t.Category), strings.ToLower(q)) {
					continue
				}
			}
			resp.Freelance = append(resp.Freelance, GlobalSearchItem{
				Type: "freelance", ID: t.ID.String(), Title: title, Subtitle: t.Category,
			})
			count++
		}
	}
	if h.hackathonRepo != nil {
		hacks, _ := h.hackathonRepo.ListPublished(r.Context(), 50)
		count := 0
		for _, hc := range hacks {
			if count >= limit {
				break
			}
			title := ""
			if len(hc.TitleEnc) > 0 {
				if b, err := crypto.Decrypt(hc.TitleEnc, h.aesKey); err == nil {
					title = string(b)
				}
			}
			if title == "" || !strings.Contains(strings.ToLower(title+" "+hc.Theme), strings.ToLower(q)) {
				continue
			}
			resp.Hackathons = append(resp.Hackathons, GlobalSearchItem{
				Type: "hackathon", ID: hc.ID.String(), Title: title, Subtitle: hc.Theme,
			})
			count++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
