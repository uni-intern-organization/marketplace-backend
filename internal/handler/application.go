package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type ApplicationHandler struct {
	appRepo     *repository.ApplicationRepository
	invRepo     *repository.InvitationRepository
	userRepo    *repository.UserRepository
	vacancyRepo *repository.VacancyRepository
	notifier    *notifier.Service
	aesKey      []byte
}

func NewApplicationHandler(appRepo *repository.ApplicationRepository, invRepo *repository.InvitationRepository, userRepo *repository.UserRepository, vacancyRepo *repository.VacancyRepository, notifier *notifier.Service, aesKey []byte) *ApplicationHandler {
	return &ApplicationHandler{appRepo: appRepo, invRepo: invRepo, userRepo: userRepo, vacancyRepo: vacancyRepo, notifier: notifier, aesKey: aesKey}
}

type CreateApplicationRequest struct {
	RecruiterID  string `json:"recruiter_id"`
	VacancyID    string `json:"vacancy_id"`
	InvitationID string `json:"invitation_id,omitempty"`
	CoverLetter  string `json:"cover_letter,omitempty"`
}

type ApplicationResponse struct {
	ID                   string   `json:"id"`
	StudentID            string   `json:"student_id"`
	StudentEmail         string   `json:"student_email,omitempty"`
	StudentFullName      string   `json:"student_full_name,omitempty"`
	RecruiterID          string   `json:"recruiter_id"`
	VacancyID            string   `json:"vacancy_id"`
	VacancyTitle         string   `json:"vacancy_title"`
	RecruiterCompanyName string   `json:"recruiter_company_name,omitempty"`
	CoverLetter          string   `json:"cover_letter,omitempty"`
	Status               string   `json:"status"`
	InterviewFormat      string   `json:"interview_format,omitempty"`
	InterviewMessage     string   `json:"interview_message,omitempty"`
	ProposedSlots        []string `json:"proposed_slots,omitempty"`
	InterviewScheduledAt string   `json:"interview_scheduled_at,omitempty"`
	DecisionReason       string   `json:"decision_reason,omitempty"`
	OfferStartDate       string   `json:"offer_start_date,omitempty"`
	OfferTerms           string   `json:"offer_terms,omitempty"`
	OfferDuration        string   `json:"offer_duration,omitempty"`
	CreatedAt            string   `json:"created_at"`
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req CreateApplicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	vacancyID, err := uuid.Parse(req.VacancyID)
	if err != nil {
		http.Error(w, `{"error":"invalid vacancy_id"}`, http.StatusBadRequest)
		return
	}
	vacancy, err := h.vacancyRepo.GetByID(r.Context(), vacancyID)
	if err != nil {
		http.Error(w, `{"error":"vacancy not found"}`, http.StatusNotFound)
		return
	}
	recruiterID, err := uuid.Parse(req.RecruiterID)
	if err != nil {
		http.Error(w, `{"error":"invalid recruiter_id"}`, http.StatusBadRequest)
		return
	}
	if vacancy.RecruiterID != recruiterID || vacancy.Status != model.VacancyStatusActive {
		http.Error(w, `{"error":"vacancy is not open for applications"}`, http.StatusConflict)
		return
	}
	_, err = h.userRepo.GetByID(r.Context(), recruiterID)
	if err != nil {
		http.Error(w, `{"error":"recruiter not found"}`, http.StatusNotFound)
		return
	}
	var invitationID *uuid.UUID
	if req.InvitationID != "" {
		id, err := uuid.Parse(req.InvitationID)
		if err != nil {
			http.Error(w, `{"error":"invalid invitation_id"}`, http.StatusBadRequest)
			return
		}
		invitationID = &id
	}
	var coverLetterEnc []byte
	if req.CoverLetter != "" {
		coverLetterEnc, err = crypto.Encrypt([]byte(req.CoverLetter), h.aesKey)
		if err != nil {
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
	}
	app, err := h.appRepo.Create(r.Context(), claims.UserID, recruiterID, vacancyID, invitationID, coverLetterEnc)
	if err != nil {
		http.Error(w, `{"error":"failed to create application"}`, http.StatusInternalServerError)
		return
	}
	resp := ApplicationResponse{
		ID:          app.ID.String(),
		StudentID:   app.StudentID.String(),
		RecruiterID: app.RecruiterID.String(),
		VacancyID:   vacancyID.String(),
		Status:      app.Status,
		CreatedAt:   app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	// заполнить vacancy_title и recruiter_company_name из вакансии
	if len(vacancy.TitleEnc) > 0 {
		if b, err := crypto.Decrypt(vacancy.TitleEnc, h.aesKey); err == nil {
			resp.VacancyTitle = string(b)
		}
	}
	if vacancy.CompanyName != "" {
		resp.RecruiterCompanyName = vacancy.CompanyName
	}
	if len(app.CoverLetterEnc) > 0 {
		b, _ := crypto.Decrypt(app.CoverLetterEnc, h.aesKey)
		resp.CoverLetter = string(b)
	}
	h.notifier.Notify(r.Context(), recruiterID, "application_created", "Новый отклик", "Студент откликнулся на вашу вакансию", map[string]interface{}{"application_id": app.ID.String(), "vacancy_id": vacancyID.String()})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ApplicationHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var list []model.Application
	var err error
	if claims.Role == model.RoleStudent {
		list, err = h.appRepo.ListByStudent(r.Context(), claims.UserID)
	} else if claims.Role == model.RoleRecruiter {
		list, err = h.appRepo.ListByRecruiter(r.Context(), claims.UserID)
	} else {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]ApplicationResponse, 0, len(list))
	for _, app := range list {
		ar := ApplicationResponse{
			ID:          app.ID.String(),
			StudentID:   app.StudentID.String(),
			RecruiterID: app.RecruiterID.String(),
			Status:      app.Status,
			CreatedAt:   app.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		fillApplicationLifecycle(&ar, &app)
		if claims.Role == model.RoleRecruiter {
			if student, err := h.userRepo.GetByID(r.Context(), app.StudentID); err == nil {
				ar.StudentEmail = student.Email
			}
			if profile, err := h.userRepo.GetStudentProfileByUserID(r.Context(), app.StudentID); err == nil && len(profile.FullNameEnc) > 0 {
				if value, err := crypto.Decrypt(profile.FullNameEnc, h.aesKey); err == nil {
					ar.StudentFullName = string(value)
				}
			}
		}
		if app.VacancyID != nil {
			ar.VacancyID = app.VacancyID.String()
			if vac, err := h.vacancyRepo.GetByID(r.Context(), *app.VacancyID); err == nil {
				if len(vac.TitleEnc) > 0 {
					if b, err := crypto.Decrypt(vac.TitleEnc, h.aesKey); err == nil {
						ar.VacancyTitle = string(b)
					}
				}
				if vac.CompanyName != "" {
					ar.RecruiterCompanyName = vac.CompanyName
				}
			}
		}
		if len(app.CoverLetterEnc) > 0 {
			if b, err := crypto.Decrypt(app.CoverLetterEnc, h.aesKey); err == nil {
				ar.CoverLetter = string(b)
			}
		}
		resp = append(resp, ar)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type UpdateApplicationStatusRequest struct {
	Status               string   `json:"status"`
	InterviewFormat      string   `json:"interview_format,omitempty"`
	InterviewMessage     string   `json:"interview_message,omitempty"`
	ProposedSlots        []string `json:"proposed_slots,omitempty"`
	InterviewScheduledAt string   `json:"interview_scheduled_at,omitempty"`
	DecisionReason       string   `json:"decision_reason,omitempty"`
	OfferStartDate       string   `json:"offer_start_date,omitempty"`
	OfferTerms           string   `json:"offer_terms,omitempty"`
	OfferDuration        string   `json:"offer_duration,omitempty"`
}

func (h *ApplicationHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || (claims.Role != model.RoleRecruiter && claims.Role != model.RoleStudent) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
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
	app, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"application not found"}`, http.StatusNotFound)
		return
	}
	if (claims.Role == model.RoleRecruiter && app.RecruiterID != claims.UserID) ||
		(claims.Role == model.RoleStudent && app.StudentID != claims.UserID) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req UpdateApplicationStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if !canTransition(claims.Role, app.Status, req.Status) {
		http.Error(w, `{"error":"invalid application status transition"}`, http.StatusConflict)
		return
	}
	update, err := parseLifecycleUpdate(req)
	if err != nil {
		http.Error(w, `{"error":"invalid lifecycle details"}`, http.StatusBadRequest)
		return
	}
	if err := validateLifecycleUpdate(req, update); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}
	if req.Status == "interview_scheduled" && !containsTime(app.ProposedSlots, *update.InterviewScheduledAt) {
		http.Error(w, `{"error":"selected interview slot was not proposed"}`, http.StatusBadRequest)
		return
	}
	if err := h.appRepo.UpdateLifecycle(r.Context(), id, update); err != nil {
		http.Error(w, `{"error":"failed to update"}`, http.StatusInternalServerError)
		return
	}
	recipient := app.StudentID
	if claims.Role == model.RoleStudent {
		recipient = app.RecruiterID
	}
	title, body := applicationNotification(req.Status, req.DecisionReason)
	h.notifier.Notify(r.Context(), recipient, "application_status", title, body, map[string]interface{}{"application_id": id.String(), "status": req.Status})
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": req.Status})
}

func (h *ApplicationHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		ApplicationID string `json:"application_id"`
		Rating        int    `json:"rating"`
		Comment       string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	applicationID, err := uuid.Parse(req.ApplicationID)
	if err != nil || req.Rating < 1 || req.Rating > 5 {
		http.Error(w, `{"error":"invalid review"}`, http.StatusBadRequest)
		return
	}
	app, err := h.appRepo.GetByID(r.Context(), applicationID)
	if err != nil || app.RecruiterID != claims.UserID || app.Status != model.AppStatusCompleted {
		http.Error(w, `{"error":"completed internship required"}`, http.StatusForbidden)
		return
	}
	if err := h.appRepo.CreateReview(r.Context(), applicationID, claims.UserID, app.StudentID, req.Rating, req.Comment); err != nil {
		http.Error(w, `{"error":"review already exists"}`, http.StatusConflict)
		return
	}
	h.notifier.Notify(r.Context(), app.StudentID, "internship_review", "Новый отзыв", "Работодатель оставил отзыв о вашей стажировке", map[string]interface{}{"application_id": applicationID.String()})
	jsonOK(w, map[string]string{"status": "ok"})
}

func fillApplicationLifecycle(resp *ApplicationResponse, app *model.Application) {
	resp.InterviewFormat = app.InterviewFormat
	resp.InterviewMessage = app.InterviewMessage
	for _, slot := range app.ProposedSlots {
		resp.ProposedSlots = append(resp.ProposedSlots, slot.Format(time.RFC3339))
	}
	if app.InterviewScheduledAt != nil {
		resp.InterviewScheduledAt = app.InterviewScheduledAt.Format(time.RFC3339)
	}
	resp.DecisionReason = app.DecisionReason
	if app.OfferStartDate != nil {
		resp.OfferStartDate = app.OfferStartDate.Format("2006-01-02")
	}
	resp.OfferTerms = app.OfferTerms
	resp.OfferDuration = app.OfferDuration
}

func canTransition(role model.UserRole, from, to string) bool {
	recruiter := map[string]map[string]bool{
		"new":                 {"viewed": true, "under_review": true, "interview_invited": true, "rejected": true},
		"viewed":              {"under_review": true, "interview_invited": true, "rejected": true},
		"under_review":        {"interview_invited": true, "rejected": true},
		"interview_invited":   {"interview_invited": true, "rejected": true},
		"interview_scheduled": {"awaiting_decision": true, "offer_sent": true, "rejected_after_interview": true},
		"awaiting_decision":   {"offer_sent": true, "rejected_after_interview": true},
		"offer_sent":          {"rejected_after_interview": true},
		"hired":               {"completed": true},
	}
	student := map[string]map[string]bool{
		"interview_invited": {"interview_scheduled": true},
		"offer_sent":        {"hired": true},
	}
	if role == model.RoleRecruiter {
		return recruiter[from][to]
	}
	return student[from][to]
}

func parseLifecycleUpdate(req UpdateApplicationStatusRequest) (repository.ApplicationLifecycleUpdate, error) {
	u := repository.ApplicationLifecycleUpdate{
		Status: req.Status, InterviewFormat: req.InterviewFormat, InterviewMessage: req.InterviewMessage,
		DecisionReason: req.DecisionReason, OfferTerms: req.OfferTerms, OfferDuration: req.OfferDuration,
	}
	for _, raw := range req.ProposedSlots {
		slot, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return u, err
		}
		u.ProposedSlots = append(u.ProposedSlots, slot)
	}
	if req.InterviewScheduledAt != "" {
		value, err := time.Parse(time.RFC3339, req.InterviewScheduledAt)
		if err != nil {
			return u, err
		}
		u.InterviewScheduledAt = &value
	}
	if req.OfferStartDate != "" {
		value, err := time.Parse("2006-01-02", req.OfferStartDate)
		if err != nil {
			return u, err
		}
		u.OfferStartDate = &value
	}
	return u, nil
}

func validateLifecycleUpdate(req UpdateApplicationStatusRequest, u repository.ApplicationLifecycleUpdate) error {
	if req.Status == "interview_invited" {
		if req.InterviewFormat != "online" && req.InterviewFormat != "offline" {
			return fmt.Errorf("interview_format must be online or offline")
		}
		if len(u.ProposedSlots) == 0 {
			return fmt.Errorf("at least one proposed interview slot is required")
		}
	}
	if req.Status == "interview_scheduled" && u.InterviewScheduledAt == nil {
		return fmt.Errorf("interview_scheduled_at is required")
	}
	if req.Status == "offer_sent" && (u.OfferStartDate == nil || req.OfferTerms == "") {
		return fmt.Errorf("offer_start_date and offer_terms are required")
	}
	return nil
}

func applicationNotification(status, reason string) (string, string) {
	switch status {
	case "under_review":
		return "Отклик на рассмотрении", "Работодатель рассматривает вашу кандидатуру"
	case "interview_invited":
		return "Приглашение на собеседование", "Работодатель предложил варианты времени собеседования"
	case "interview_scheduled":
		return "Собеседование назначено", "Студент подтвердил время собеседования"
	case "awaiting_decision":
		return "Ожидание решения", "Собеседование завершено, компания готовит решение"
	case "offer_sent":
		return "Вы получили оффер", "Работодатель отправил предложение о стажировке"
	case "hired":
		return "Оффер принят", "Студент принял предложение о стажировке"
	case "completed":
		return "Стажировка завершена", "Теперь вы можете оставить отзыв"
	case "rejected", "rejected_after_interview":
		if reason == "" {
			reason = "Работодатель завершил рассмотрение кандидатуры"
		}
		return "Статус отклика обновлён", reason
	default:
		return "Статус отклика обновлён", status
	}
}

func containsTime(slots []time.Time, selected time.Time) bool {
	for _, slot := range slots {
		if slot.Equal(selected) {
			return true
		}
	}
	return false
}
