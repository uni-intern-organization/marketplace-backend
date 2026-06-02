package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/auth"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/email"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type AuthHandler struct {
	userRepo     *repository.UserRepository
	authSecRepo  *repository.AuthSecurityRepository
	auditRepo    *repository.AuditRepository
	emailSvc     *email.Service
	aesKey       []byte
	jwtSecret    string
	expireHours  int
	refreshDays  int
	rateLimit    config.RateLimitConfig
	frontendURL  string
	appName      string
}

func NewAuthHandler(
	userRepo *repository.UserRepository,
	authSecRepo *repository.AuthSecurityRepository,
	auditRepo *repository.AuditRepository,
	emailSvc *email.Service,
	aesKey []byte,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		userRepo: userRepo, authSecRepo: authSecRepo, auditRepo: auditRepo, emailSvc: emailSvc,
		aesKey: aesKey, jwtSecret: cfg.JWT.Secret, expireHours: cfg.JWT.ExpireHours,
		refreshDays: cfg.JWT.RefreshExpireDays, rateLimit: cfg.RateLimit,
		frontendURL: cfg.App.FrontendURL, appName: cfg.App.AppName,
	}
}

type RegisterRequest struct {
	Email    string         `json:"email"`
	Password string         `json:"password"`
	Role     model.UserRole `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type AuthResponse struct {
	Token        string       `json:"token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	User         UserResponse `json:"user"`
	Requires2FA  bool         `json:"requires_2fa,omitempty"`
}

type UserResponse struct {
	ID    string         `json:"id"`
	Email string         `json:"email"`
	Role  model.UserRole `json:"role"`
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

func (h *AuthHandler) issueTokens(w http.ResponseWriter, r *http.Request, user *model.User) {
	token, err := auth.NewToken(user.ID, user.Email, user.Role, h.jwtSecret, h.expireHours)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	refreshRaw, err := auth.GenerateSecureToken(32)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	expires := time.Now().Add(time.Duration(h.refreshDays) * 24 * time.Hour)
	if err := h.authSecRepo.CreateRefreshToken(r.Context(), user.ID, refreshRaw, expires); err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	actor := user.ID
	_ = h.auditRepo.Log(r.Context(), &actor, "login", "user", &user.ID, nil)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		Token: token, RefreshToken: refreshRaw,
		User: UserResponse{ID: user.ID.String(), Email: user.Email, Role: user.Role},
	})
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
		RespondError(w, http.StatusInternalServerError, "failed to create user", err)
		return
	}
	_ = h.emailSvc.SendWelcome(user.Email)
	actor := user.ID
	_ = h.auditRepo.Log(r.Context(), &actor, "register", "user", &user.ID, map[string]interface{}{"role": req.Role})
	h.issueTokens(w, r, user)
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
	since := time.Now().Add(-time.Duration(h.rateLimit.LockoutMinutes) * time.Minute)
	failed, _ := h.authSecRepo.FailedLoginCount(r.Context(), req.Email, since)
	if failed >= h.rateLimit.MaxFailedLogins {
		http.Error(w, `{"error":"too many failed attempts, try again later"}`, http.StatusTooManyRequests)
		return
	}
	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		_ = h.authSecRepo.RecordLoginAttempt(r.Context(), req.Email, clientIP(r), false)
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		_ = h.authSecRepo.RecordLoginAttempt(r.Context(), req.Email, clientIP(r), false)
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}
	if user.IsBlocked {
		http.Error(w, `{"error":"account blocked"}`, http.StatusForbidden)
		return
	}
	enabled, _ := h.authSecRepo.IsTOTPEnabled(r.Context(), user.ID)
	if enabled {
		if req.TOTPCode == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"requires_2fa": true})
			return
		}
		secretEnc, _, err := h.authSecRepo.GetTOTPSecret(r.Context(), user.ID)
		if err != nil {
			http.Error(w, `{"error":"2fa error"}`, http.StatusInternalServerError)
			return
		}
		secret, err := crypto.Decrypt(secretEnc, h.aesKey)
		if err != nil || !auth.ValidateTOTP(string(secret), req.TOTPCode) {
			http.Error(w, `{"error":"invalid 2fa code"}`, http.StatusUnauthorized)
			return
		}
	}
	_ = h.authSecRepo.RecordLoginAttempt(r.Context(), req.Email, clientIP(r), true)
	h.issueTokens(w, r, user)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		http.Error(w, `{"error":"refresh_token required"}`, http.StatusBadRequest)
		return
	}
	userID, err := h.authSecRepo.GetRefreshTokenUserID(r.Context(), req.RefreshToken)
	if err != nil {
		http.Error(w, `{"error":"invalid refresh token"}`, http.StatusUnauthorized)
		return
	}
	_ = h.authSecRepo.RevokeRefreshToken(r.Context(), req.RefreshToken)
	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
		return
	}
	h.issueTokens(w, r, user)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RefreshToken != "" {
		_ = h.authSecRepo.RevokeRefreshToken(r.Context(), req.RefreshToken)
	}
	if claims := middleware.GetClaims(r.Context()); claims != nil {
		actor := claims.UserID
		_ = h.auditRepo.Log(r.Context(), &actor, "logout", "user", &claims.UserID, nil)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, `{"error":"email required"}`, http.StatusBadRequest)
		return
	}
	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err == nil {
		token, _ := auth.GenerateSecureToken(32)
		_ = h.authSecRepo.CreatePasswordResetToken(r.Context(), user.ID, token, time.Now().Add(time.Hour))
		resetURL := h.frontendURL + "/reset-password?token=" + token
		_ = h.emailSvc.SendPasswordReset(user.Email, resetURL)
	}
	jsonOK(w, map[string]string{"status": "ok", "message": "if account exists, reset email sent"})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.NewPassword == "" {
		http.Error(w, `{"error":"token and new_password required"}`, http.StatusBadRequest)
		return
	}
	userID, err := h.authSecRepo.ConsumePasswordResetToken(r.Context(), req.Token)
	if err != nil {
		http.Error(w, `{"error":"invalid or expired token"}`, http.StatusBadRequest)
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if err := h.authSecRepo.UpdatePassword(r.Context(), userID, hash); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	_ = h.authSecRepo.RevokeAllUserTokens(r.Context(), userID)
	_ = h.auditRepo.Log(r.Context(), &userID, "password_reset", "user", &userID, nil)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	secret, url, err := auth.GenerateTOTPSecret(h.appName, claims.Email)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	enc, err := crypto.Encrypt([]byte(secret), h.aesKey)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	if err := h.authSecRepo.SaveTOTPSecret(r.Context(), claims.UserID, enc); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"secret": secret, "otpauth_url": url})
}

func (h *AuthHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, `{"error":"code required"}`, http.StatusBadRequest)
		return
	}
	secretEnc, _, err := h.authSecRepo.GetTOTPSecret(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"2fa not setup"}`, http.StatusBadRequest)
		return
	}
	secret, err := crypto.Decrypt(secretEnc, h.aesKey)
	if err != nil || !auth.ValidateTOTP(string(secret), req.Code) {
		http.Error(w, `{"error":"invalid code"}`, http.StatusBadRequest)
		return
	}
	if err := h.authSecRepo.EnableTOTP(r.Context(), claims.UserID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	_ = h.auditRepo.Log(r.Context(), &claims.UserID, "2fa_enabled", "user", &claims.UserID, nil)
	jsonOK(w, map[string]string{"status": "ok"})
}
