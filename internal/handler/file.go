package handler

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	"github.com/uni-intern-organization/marketplace-backend/internal/storage"
	"golang.org/x/image/webp"
)

type FileHandler struct {
	storage       *storage.S3Storage
	userRepo      *repository.UserRepository
	recruiterRepo *repository.RecruiterProfileRepository
	bannerRepo    *repository.BannerRepository
}

func NewFileHandler(s *storage.S3Storage, userRepo *repository.UserRepository, recruiterRepo *repository.RecruiterProfileRepository, bannerRepo *repository.BannerRepository) *FileHandler {
	return &FileHandler{storage: s, userRepo: userRepo, recruiterRepo: recruiterRepo, bannerRepo: bannerRepo}
}

const maxBannerSize = model.MaxBannerFileBytes

const maxResumeSize = 5 << 20 // 5 MB
const maxLogoSize = 2 << 20   // 2 MB
const maxAvatarSize = 2 << 20 // 2 MB

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

func (h *FileHandler) UploadStudentAvatar(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		file, header, err = r.FormFile("file")
	}
	if err != nil {
		http.Error(w, `{"error":"avatar file required"}`, http.StatusBadRequest)
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
	key := fmt.Sprintf("avatars/%s/%s%s", claims.UserID, uuid.New(), ext)
	if err := h.storage.Upload(r.Context(), key, file, contentType); err != nil {
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}
	if err := h.userRepo.SetStudentAvatarKey(r.Context(), claims.UserID, key); err != nil {
		http.Error(w, `{"error":"failed to save reference"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"object_key": key})
}

func (h *FileHandler) GetCompanyLogo(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, `{"error":"key required"}`, http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(key, "logos/") || path.Clean(key) != key || strings.Contains(key, `\`) {
		http.Error(w, `{"error":"invalid logo key"}`, http.StatusBadRequest)
		return
	}

	body, contentType, err := h.storage.Get(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"failed to get logo"}`, http.StatusNotFound)
		return
	}
	defer body.Close()

	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	if _, err := io.Copy(w, body); err != nil {
		return
	}
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

func (h *FileHandler) UploadBanner(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || (claims.Role != model.RoleRecruiter && claims.Role != model.RoleAdmin) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(maxBannerSize); err != nil {
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}
	placement := r.FormValue("placement")
	if placement == "" {
		http.Error(w, `{"error":"placement required"}`, http.StatusBadRequest)
		return
	}
	if h.bannerRepo != nil {
		if _, err := h.bannerRepo.GetPlacement(r.Context(), placement); err != nil {
			http.Error(w, `{"error":"unknown placement"}`, http.StatusBadRequest)
			return
		}
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		file, header, err = r.FormFile("banner")
	}
	if err != nil {
		http.Error(w, `{"error":"banner file required"}`, http.StatusBadRequest)
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
	data, err := io.ReadAll(io.LimitReader(file, maxBannerSize+1))
	if err != nil {
		http.Error(w, `{"error":"failed to read file"}`, http.StatusBadRequest)
		return
	}
	if len(data) > maxBannerSize {
		http.Error(w, `{"error":"file too large (max 10MB)"}`, http.StatusBadRequest)
		return
	}
	if h.bannerRepo != nil {
		pl, err := h.bannerRepo.GetPlacement(r.Context(), placement)
		if err == nil {
			wImg, hImg, err := decodeBannerDimensions(data, contentType)
			if err != nil {
				http.Error(w, `{"error":"invalid image"}`, http.StatusBadRequest)
				return
			}
			allowed := model.BannerAllowedSizes(pl.Code, pl.Width, pl.Height)
			if !model.BannerDimensionsMatch(wImg, hImg, allowed) {
				msg := model.BannerDimensionError(wImg, hImg, allowed)
				http.Error(w, fmt.Sprintf(`{"error":%q,"actual_size":"%dx%d","expected_sizes":%q}`,
					msg, wImg, hImg, model.BannerAllowedSizesLabel(allowed)), http.StatusBadRequest)
				return
			}
		}
	}
	key := fmt.Sprintf("banners/%s/%s%s", claims.UserID.String(), uuid.New().String(), ext)
	if err := h.storage.Upload(r.Context(), key, bytes.NewReader(data), contentType); err != nil {
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"object_key":"%s","placement":"%s"}`, key, placement)))
}

func decodeBannerDimensions(data []byte, contentType string) (int, int, error) {
	r := bytes.NewReader(data)
	if contentType == "image/webp" {
		cfg, err := webp.DecodeConfig(r)
		if err != nil {
			return 0, 0, err
		}
		return cfg.Width, cfg.Height, nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}
