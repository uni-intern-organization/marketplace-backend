package handler

import (
	"encoding/json"
	"net/http"

	"github.com/uni-intern-organization/marketplace-backend/internal/auth"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type AuthHandler struct {
	userRepo *repository.UserRepository
	jwtSecret string
	expireHours int
}

func NewAuthHandler(userRepo *repository.UserRepository, jwtSecret string, expireHours int) *AuthHandler {
	return &AuthHandler{userRepo: userRepo, jwtSecret: jwtSecret, expireHours: expireHours}
}

type RegisterRequest struct {
	Email    string       `json:"email"`
	Password string       `json:"password"`
	Role     model.UserRole `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type UserResponse struct {
	ID    string       `json:"id"`
	Email string       `json:"email"`
	Role  model.UserRole `json:"role"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}
	if req.Role != model.RoleStudent && req.Role != model.RoleRecruiter && req.Role != model.RoleAdmin {
		http.Error(w, `{"error":"role must be student, recruiter or admin"}`, http.StatusBadRequest)
		return
	}

	exists, err := h.userRepo.ExistsByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, `{"error":"user already registered"}`, http.StatusConflict)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	user, err := h.userRepo.Create(r.Context(), req.Email, hash, req.Role)
	if err != nil {
		http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
		return
	}

	token, err := auth.NewToken(user.ID, user.Email, user.Role, h.jwtSecret, h.expireHours)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User: UserResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Email == "" || req.Password == "" {
		http.Error(w, `{"error":"email and password required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := auth.NewToken(user.ID, user.Email, user.Role, h.jwtSecret, h.expireHours)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User:  UserResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role},
	})
}
