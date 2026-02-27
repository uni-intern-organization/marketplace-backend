package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type SearchHandler struct {
	pool     *pgxpool.Pool
	userRepo *repository.UserRepository
}

func NewSearchHandler(pool *pgxpool.Pool, userRepo *repository.UserRepository) *SearchHandler {
	return &SearchHandler{pool: pool, userRepo: userRepo}
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
