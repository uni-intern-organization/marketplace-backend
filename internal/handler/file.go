package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	"github.com/uni-intern-organization/marketplace-backend/internal/storage"
)

type FileHandler struct {
	storage       *storage.S3Storage
	userRepo      *repository.UserRepository
	recruiterRepo *repository.RecruiterProfileRepository
}

func NewFileHandler(s *storage.S3Storage, userRepo *repository.UserRepository, recruiterRepo *repository.RecruiterProfileRepository) *FileHandler {
	return &FileHandler{storage: s, userRepo: userRepo, recruiterRepo: recruiterRepo}
}

const maxResumeSize = 5 << 20 // 5 MB
const maxLogoSize = 2 << 20   // 2 MB

func (h *FileHandler) UploadResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(maxResumeSize); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("resume")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		http.Error(w, `{"error":"resume file required (form field: resume or file)"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" {
		http.Error(w, `{"error":"only PDF allowed"}`, http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("resumes/%s/%s.pdf", claims.UserID.String(), uuid.New().String())
	if err := h.storage.Upload(r.Context(), key, file, "application/pdf"); err != nil {
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}
	if err := h.userRepo.SetStudentResumeKey(r.Context(), claims.UserID, key); err != nil {
		http.Error(w, `{"error":"failed to save reference"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"object_key":"%s"}`, key)))
}

func (h *FileHandler) UploadCompanyLogo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(maxLogoSize); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("logo")
	if err != nil {
		http.Error(w, `{"error":"logo file required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp"}
	contentType, ok := allowed[ext]
	if !ok {
		http.Error(w, `{"error":"only PNG, JPG, WEBP allowed"}`, http.StatusBadRequest)
		return
	}
	key := fmt.Sprintf("logos/%s/%s%s", claims.UserID.String(), uuid.New().String(), ext)
	if claims != nil {
		// Retrieve the current logo key from the database
		currentLogoKey, err := h.recruiterRepo.GetLogoKeyByUserID(r.Context(), claims.UserID)
		if err == nil && currentLogoKey != "" {
			// Delete the existing logo from S3
			err = h.storage.Delete(r.Context(), currentLogoKey)
			if err != nil {
				// Log the error but continue with the upload
				fmt.Printf("Failed to delete previous logo: %v\n", err)
			}
		}
	}
	if err := h.storage.Upload(r.Context(), key, file, contentType); err != nil {
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}
	if err := h.recruiterRepo.UpdateLogo(r.Context(), claims.UserID, key); err != nil {
		http.Error(w, `{"error":"failed to save reference"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"object_key":"%s"}`, key)))
}

func (h *FileHandler) GetPresignedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, `{"error":"key required"}`, http.StatusBadRequest)
		return
	}
	url, err := h.storage.GetPresignedURL(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"failed to get url"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"url":"%s"}`, url)))
}

// GetLogo отдаёт логотип компании через прокси (с авторизацией). Рекрутер может получить только свой логотип.
// Принимает GET, HEAD и POST (POST — чтобы «Показать логотип» не давал 405, если фронт шлёт POST).
func (h *FileHandler) GetLogo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost:
		// разрешено
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, `{"error":"key required"}`, http.StatusBadRequest)
		return
	}
	prefix := "logos/" + claims.UserID.String() + "/"
	if !strings.HasPrefix(key, prefix) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	body, contentType, err := h.storage.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"failed to get file"}`, http.StatusInternalServerError)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", contentType)
	io.Copy(w, body)
}
// GetResume отдаёт резюме студента через прокси (с авторизацией). Студент может получить только своё резюме.
// Принимает GET, HEAD и POST.
func (h *FileHandler) GetResume(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost:
		// разрешено
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, `{"error":"key required"}`, http.StatusBadRequest)
		return
	}
	prefix := "resumes/" + claims.UserID.String() + "/"
	if !strings.HasPrefix(key, prefix) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	body, contentType, err := h.storage.GetObject(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"failed to get file"}`, http.StatusInternalServerError)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=resume.pdf")
	io.Copy(w, body)
}

// GetStudentResume отдаёт резюме студента. Может быть использовано рекруитером для просмотра резюме кандидата.
// Принимает GET, HEAD и POST.
func (h *FileHandler) GetStudentResume(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost:
		// разрешено
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, `{"error":"user_id required"}`, http.StatusBadRequest)
		return
	}
	
	// Get resume key for the specified user
	resumeKey, err := h.userRepo.GetStudentResumeKey(r.Context(), userID)
	if err != nil || resumeKey == "" {
		http.Error(w, `{"error":"resume not found"}`, http.StatusNotFound)
		return
	}
	
	body, contentType, err := h.storage.GetObject(r.Context(), resumeKey)
	if err != nil {
		http.Error(w, `{"error":"failed to get file"}`, http.StatusInternalServerError)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=resume.pdf")
	io.Copy(w, body)
}