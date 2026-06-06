package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	"github.com/uni-intern-organization/marketplace-backend/internal/storage"
)

type ModerationHandler struct {
	modRepo          *repository.ModerationRepository
	vacancyRepo      *repository.VacancyRepository
	hackRepo         *repository.HackathonRepository
	freelanceRepo    *repository.FreelanceRepository
	verificationRepo *repository.VerificationRepository
	userRepo         *repository.UserRepository
	staffRepo        *repository.StaffRepository
	authSecRepo      *repository.AuthSecurityRepository
	auditRepo        *repository.AuditRepository
	notifier         *notifier.Service
	aesKey           []byte
}

func NewModerationHandler(
	modRepo *repository.ModerationRepository,
	vacancyRepo *repository.VacancyRepository,
	hackRepo *repository.HackathonRepository,
	freelanceRepo *repository.FreelanceRepository,
	verificationRepo *repository.VerificationRepository,
	userRepo *repository.UserRepository,
	staffRepo *repository.StaffRepository,
	authSecRepo *repository.AuthSecurityRepository,
	auditRepo *repository.AuditRepository,
	notifier *notifier.Service,
	aesKey []byte,
) *ModerationHandler {
	return &ModerationHandler{
		modRepo: modRepo, vacancyRepo: vacancyRepo, hackRepo: hackRepo, freelanceRepo: freelanceRepo,
		verificationRepo: verificationRepo, userRepo: userRepo, staffRepo: staffRepo,
		authSecRepo: authSecRepo, auditRepo: auditRepo, notifier: notifier, aesKey: aesKey,
	}
}

func (h *ModerationHandler) Queue(w http.ResponseWriter, r *http.Request) {
	entity := r.URL.Query().Get("type")
	if entity == "hackathons" {
		ids, err := h.modRepo.ListPendingHackathons(r.Context(), 50)
		if err != nil {
			http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
			return
		}
		out := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			item := map[string]string{"id": id.String(), "type": "hackathon"}
			if hack, err := h.hackRepo.Get(r.Context(), id); err == nil {
				item["status"] = hack.Status
				if len(hack.TitleEnc) > 0 {
					if b, err := crypto.Decrypt(hack.TitleEnc, h.aesKey); err == nil {
						item["title"] = string(b)
					}
				}
				if hack.Theme != "" {
					item["description"] = hack.Theme
				}
			}
			out = append(out, item)
		}
		jsonOK(w, map[string]interface{}{"items": out})
		return
	}
	if entity == "freelance" {
		list, err := h.freelanceRepo.ListPendingReview(r.Context(), 50)
		if err != nil {
			http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
			return
		}
		items := make([]freelanceTaskResp, 0, len(list))
		for i := range list {
			items = append(items, taskToResp(&list[i], h.aesKey))
		}
		jsonOK(w, map[string]interface{}{"items": items})
		return
	}
	list, err := h.modRepo.ListPendingVacancies(r.Context(), 50)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	queueKind := r.URL.Query().Get("type")
	items := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		v := list[i]
		if queueKind == "internships" && !strings.Contains(strings.ToLower(v.EmploymentType), "intern") {
			continue
		}
		if queueKind != "internships" && strings.Contains(strings.ToLower(v.EmploymentType), "intern") {
			continue
		}
		resp := vacancyToResponse(&v, h.aesKey)
		item := map[string]interface{}{
			"id": resp.ID, "recruiter_id": resp.RecruiterID, "title": resp.Title,
			"description": resp.Description, "company_name": resp.CompanyName,
			"required_skills": resp.RequiredSkills, "location": resp.Location,
			"employment_type": resp.EmploymentType, "min_experience_years": resp.MinExperienceYears,
			"listing_tier": resp.ListingTier, "status": resp.Status, "created_at": resp.CreatedAt,
			"company_verified": false,
		}
		if h.verificationRepo != nil {
			if ok, _ := h.verificationRepo.IsApproved(r.Context(), v.RecruiterID); ok {
				item["company_verified"] = true
			}
		}
		items = append(items, item)
	}
	jsonOK(w, map[string]interface{}{"items": items})
}

func (h *ModerationHandler) ReviewVacancy(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Action  string `json:"action"`
		Reason  string `json:"reason"`
		Comment string `json:"comment"`
		Fraud   bool   `json:"fraud"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	v, err := h.vacancyRepo.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	notifyBody := req.Comment
	if req.Reason != "" {
		if notifyBody != "" {
			notifyBody = req.Reason + ": " + notifyBody
		} else {
			notifyBody = req.Reason
		}
	}
	switch req.Action {
	case "approve":
		if err := h.modRepo.SetVacancyStatus(r.Context(), id, model.VacancyStatusActive, true); err != nil {
			http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
			return
		}
		h.notifier.Notify(r.Context(), v.RecruiterID, "vacancy_approved", "Объявление одобрено", "Ваше объявление опубликовано", map[string]interface{}{"vacancy_id": id.String()})
	case "reject":
		_ = h.modRepo.SetVacancyStatus(r.Context(), id, model.VacancyStatusRejected, false)
		if notifyBody == "" {
			notifyBody = "Объявление не соответствует правилам платформы. Исправьте нарушения и подайте заново."
		}
		h.notifier.Notify(r.Context(), v.RecruiterID, "vacancy_rejected", "Объявление отклонено", notifyBody, map[string]interface{}{"vacancy_id": id.String(), "reason": req.Reason})
		if req.Fraud && h.userRepo != nil {
			_ = h.userRepo.SetBlocked(r.Context(), v.RecruiterID, true)
			_ = h.userRepo.InsertUserBlock(r.Context(), v.RecruiterID, claims.UserID, "moderation_fraud:"+req.Reason)
			if h.authSecRepo != nil {
				_ = h.authSecRepo.RevokeAllUserTokens(r.Context(), v.RecruiterID)
			}
			if h.staffRepo != nil {
				entityID := id
				_, _ = h.staffRepo.CreateStaffTask(r.Context(), claims.UserID,
					"Подозрение на мошенничество: работодатель заблокирован",
					notifyBody, "vacancy", &entityID)
			}
		}
	case "needs_revision":
		_ = h.modRepo.SetVacancyStatus(r.Context(), id, model.VacancyStatusNeedsRevision, false)
		if notifyBody == "" {
			notifyBody = "Исправьте замечания модератора и подайте объявление заново."
		}
		h.notifier.Notify(r.Context(), v.RecruiterID, "vacancy_revision", "Требуется доработка", notifyBody, map[string]interface{}{"vacancy_id": id.String()})
	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusBadRequest)
		return
	}
	_ = h.modRepo.CreateReview(r.Context(), "vacancy", id, claims.UserID, req.Action, req.Reason, req.Comment)
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "moderate_vacancy", "vacancy", &id, map[string]interface{}{"action": req.Action, "fraud": req.Fraud, "reason": req.Reason})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *ModerationHandler) ReviewHackathon(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Action  string `json:"action"`
		Reason  string `json:"reason"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	hack, err := h.hackRepo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	switch req.Action {
	case "approve":
		_ = h.modRepo.SetHackathonStatus(r.Context(), id, "registration_open")
		h.notifier.Notify(r.Context(), hack.OrganizerID, "hackathon_approved", "Хакатон одобрен", "Регистрация открыта", map[string]interface{}{"hackathon_id": id.String()})
	case "reject":
		_ = h.modRepo.SetHackathonStatus(r.Context(), id, "rejected")
	case "needs_revision":
		_ = h.modRepo.SetHackathonStatus(r.Context(), id, "needs_revision")
	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusBadRequest)
		return
	}
	_ = h.modRepo.CreateReview(r.Context(), "hackathon", id, claims.UserID, req.Action, req.Reason, req.Comment)
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "moderate_hackathon", "hackathon", &id, map[string]interface{}{"action": req.Action})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *ModerationHandler) ReviewFreelance(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Action  string `json:"action"`
		Reason  string `json:"reason"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	t, err := h.freelanceRepo.GetTask(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	switch req.Action {
	case "approve":
		_ = h.freelanceRepo.UpdateTaskStatus(r.Context(), id, "open")
		h.notifier.Notify(r.Context(), t.RecruiterID, "freelance_approved", "Задача опубликована", "Фриланс-задача прошла модерацию", map[string]interface{}{"task_id": id.String()})
	case "reject":
		_ = h.freelanceRepo.UpdateTaskStatus(r.Context(), id, "rejected")
		h.notifier.Notify(r.Context(), t.RecruiterID, "freelance_rejected", "Задача отклонена", req.Comment, map[string]interface{}{"task_id": id.String()})
	case "needs_revision":
		_ = h.freelanceRepo.UpdateTaskStatus(r.Context(), id, "needs_revision")
		h.notifier.Notify(r.Context(), t.RecruiterID, "freelance_revision", "Требуется доработка", req.Comment, map[string]interface{}{"task_id": id.String()})
	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusBadRequest)
		return
	}
	_ = h.modRepo.CreateReview(r.Context(), "freelance_task", id, claims.UserID, req.Action, req.Reason, req.Comment)
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "moderate_freelance", "freelance_task", &id, map[string]interface{}{"action": req.Action})
	jsonOK(w, map[string]string{"status": "ok"})
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type MessagingHandler struct {
	repo     *repository.MessagingRepository
	notifier *notifier.Service
	s3       *storage.S3Storage
	aesKey   []byte
}

func NewMessagingHandler(repo *repository.MessagingRepository, notifier *notifier.Service, s3 *storage.S3Storage, aesKey []byte) *MessagingHandler {
	return &MessagingHandler{repo: repo, notifier: notifier, s3: s3, aesKey: aesKey}
}

func validMessageContext(contextType string) bool {
	switch contextType {
	case "application", "invitation", "vacancy", "freelance_task", "freelance":
		return true
	default:
		return contextType == ""
	}
}

func validAttachmentKey(senderID uuid.UUID, key string) bool {
	if key == "" {
		return true
	}
	uid := senderID.String()
	return strings.Contains(key, uid) && (strings.HasPrefix(key, "resumes/") ||
		strings.HasPrefix(key, "attachments/") || strings.HasPrefix(key, "messages/"))
}

func (h *MessagingHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	list, err := h.repo.ListConversations(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	type convResp struct {
		ID           string `json:"id"`
		StudentID    string `json:"student_id"`
		RecruiterID  string `json:"recruiter_id"`
		ContextType  string `json:"context_type"`
		ContextID    string `json:"context_id"`
		ContextTitle string `json:"context_title"`
		UpdatedAt    string `json:"updated_at"`
	}
	out := make([]convResp, 0, len(list))
	for _, c := range list {
		out = append(out, convResp{
			ID: c.ID.String(), StudentID: c.StudentID.String(), RecruiterID: c.RecruiterID.String(),
			ContextType: c.ContextType, ContextID: c.ContextID.String(), ContextTitle: c.ContextTitle,
			UpdatedAt: c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	jsonOK(w, map[string]interface{}{"conversations": out})
}

func (h *MessagingHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	convID, err := uuid.Parse(r.URL.Query().Get("conversation_id"))
	if err != nil {
		http.Error(w, `{"error":"invalid conversation_id"}`, http.StatusBadRequest)
		return
	}
	if _, err := h.repo.GetConversation(r.Context(), convID, claims.UserID); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	msgs, err := h.repo.ListMessages(r.Context(), convID, 200)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	_ = h.repo.MarkMessagesRead(r.Context(), convID, claims.UserID)
	type msgResp struct {
		ID         string  `json:"id"`
		SenderID   string  `json:"sender_id"`
		Body       string  `json:"body"`
		Attachment *string `json:"attachment_key,omitempty"`
		ReadAt     *string `json:"read_at,omitempty"`
		CreatedAt  string  `json:"created_at"`
	}
	out := make([]msgResp, 0, len(msgs))
	for _, m := range msgs {
		body := ""
		if len(m.BodyEnc) > 0 {
			b, _ := crypto.Decrypt(m.BodyEnc, h.aesKey)
			body = string(b)
		}
		var readAt *string
		if m.ReadAt != nil {
			s := m.ReadAt.Format("2006-01-02T15:04:05Z07:00")
			readAt = &s
		}
		out = append(out, msgResp{
			ID: m.ID.String(), SenderID: m.SenderID.String(), Body: body,
			Attachment: m.AttachmentKey, ReadAt: readAt,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	jsonOK(w, map[string]interface{}{"messages": out})
}

func (h *MessagingHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		ConversationID string `json:"conversation_id"`
		Body           string `json:"body"`
		AttachmentKey  string `json:"attachment_key"`
		StudentID      string `json:"student_id"`
		RecruiterID    string `json:"recruiter_id"`
		ContextType    string `json:"context_type"`
		ContextID      string `json:"context_id"`
		ContextTitle   string `json:"context_title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	var conv *model.Conversation
	var err error
	if req.ConversationID != "" {
		convID, parseErr := uuid.Parse(req.ConversationID)
		if parseErr != nil {
			http.Error(w, `{"error":"invalid conversation_id"}`, http.StatusBadRequest)
			return
		}
		conv, err = h.repo.GetConversation(r.Context(), convID, claims.UserID)
	} else {
		studentID, _ := uuid.Parse(req.StudentID)
		recruiterID, _ := uuid.Parse(req.RecruiterID)
		contextID, _ := uuid.Parse(req.ContextID)
		if !validMessageContext(req.ContextType) {
			http.Error(w, `{"error":"invalid context_type"}`, http.StatusBadRequest)
			return
		}
		if claims.Role == model.RoleStudent {
			studentID = claims.UserID
		}
		if claims.Role == model.RoleRecruiter {
			recruiterID = claims.UserID
		}
		if studentID == uuid.Nil || recruiterID == uuid.Nil || contextID == uuid.Nil {
			http.Error(w, `{"error":"student_id, recruiter_id and context_id required"}`, http.StatusBadRequest)
			return
		}
		conv, err = h.repo.FindOrCreateConversation(r.Context(), studentID, recruiterID, req.ContextType, contextID, req.ContextTitle)
	}
	if err != nil {
		http.Error(w, `{"error":"conversation not found"}`, http.StatusNotFound)
		return
	}
	if claims.Role == model.RoleStudent && conv.StudentID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if claims.Role == model.RoleRecruiter && conv.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if req.AttachmentKey != "" && !validAttachmentKey(claims.UserID, req.AttachmentKey) {
		http.Error(w, `{"error":"invalid attachment_key"}`, http.StatusBadRequest)
		return
	}
	if req.Body == "" && req.AttachmentKey == "" {
		http.Error(w, `{"error":"body or attachment required"}`, http.StatusBadRequest)
		return
	}
	bodyEnc, _ := crypto.Encrypt([]byte(req.Body), h.aesKey)
	var attach *string
	if req.AttachmentKey != "" {
		attach = &req.AttachmentKey
	}
	msg, err := h.repo.AddMessage(r.Context(), conv.ID, claims.UserID, bodyEnc, attach)
	if err != nil {
		http.Error(w, `{"error":"failed to send"}`, http.StatusInternalServerError)
		return
	}
	recipient := conv.StudentID
	if claims.UserID == conv.StudentID {
		recipient = conv.RecruiterID
	}
	h.notifier.Notify(r.Context(), recipient, "new_message", "Новое сообщение", req.Body, map[string]interface{}{"conversation_id": conv.ID.String()})
	jsonOK(w, map[string]string{"id": msg.ID.String(), "conversation_id": conv.ID.String()})
}
