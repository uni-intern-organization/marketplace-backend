package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type SearchHandler struct {
	pool *pgxpool.Pool
}

func NewSearchHandler(pool *pgxpool.Pool) *SearchHandler {
	return &SearchHandler{pool: pool}
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
		ID    string       `json:"id"`
		Email string       `json:"email"`
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
