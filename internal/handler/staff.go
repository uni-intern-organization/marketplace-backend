package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/auth"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type StaffHandler struct {
	staffRepo   *repository.StaffRepository
	auditRepo   *repository.AuditRepository
	userRepo    *repository.UserRepository
	appRepo     *repository.ApplicationRepository
	msgRepo     *repository.MessagingRepository
	authSecRepo *repository.AuthSecurityRepository
	billingRepo *repository.BillingRepository
	paymentRepo *repository.PaymentRepository
	notifier    *notifier.Service
	aesKey      []byte
}

func NewStaffHandler(
	staffRepo *repository.StaffRepository,
	auditRepo *repository.AuditRepository,
	userRepo *repository.UserRepository,
	appRepo *repository.ApplicationRepository,
	msgRepo *repository.MessagingRepository,
	authSecRepo *repository.AuthSecurityRepository,
	billingRepo *repository.BillingRepository,
	paymentRepo *repository.PaymentRepository,
	notifier *notifier.Service,
	aesKey []byte,
) *StaffHandler {
	return &StaffHandler{
		staffRepo: staffRepo, auditRepo: auditRepo, userRepo: userRepo,
		appRepo: appRepo, msgRepo: msgRepo,
		authSecRepo: authSecRepo, billingRepo: billingRepo, paymentRepo: paymentRepo, notifier: notifier, aesKey: aesKey,
	}
}

func (h *StaffHandler) CreateComplaint(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Reason     string `json:"reason"`
		Details    string `json:"details"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TargetType == "" || req.Reason == "" {
		http.Error(w, `{"error":"target_type and reason required"}`, http.StatusBadRequest)
		return
	}
	targetID, err := uuid.Parse(req.TargetID)
	if err != nil {
		http.Error(w, `{"error":"invalid target_id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.staffRepo.CreateComplaint(r.Context(), claims.UserID, req.TargetType, targetID, req.Reason, req.Details)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, repository.ComplaintToMap(c))
}

func (h *StaffHandler) ListComplaints(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.staffRepo.ListComplaints(r.Context(), status, 100)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		out = append(out, repository.ComplaintToMap(&list[i]))
	}
	jsonOK(w, map[string]interface{}{"complaints": out})
}

func (h *StaffHandler) ComplaintContext(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.staffRepo.GetComplaint(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	ctx := map[string]interface{}{
		"complaint":   repository.ComplaintToMap(c),
		"review":      nil,
		"application": nil,
		"messages":    []interface{}{},
	}
	loadAppContext := func(appID uuid.UUID) {
		if app, err := h.appRepo.GetByID(r.Context(), appID); err == nil {
			ctx["application"] = map[string]interface{}{
				"id": app.ID.String(), "status": app.Status, "student_id": app.StudentID.String(),
				"recruiter_id": app.RecruiterID.String(), "vacancy_id": app.VacancyID.String(),
				"decision_reason": app.DecisionReason, "created_at": app.CreatedAt.Format(time.RFC3339),
				"updated_at": app.UpdatedAt.Format(time.RFC3339),
			}
			if h.msgRepo != nil {
				if conv, err := h.msgRepo.GetConversationByContext(r.Context(), "application", appID); err == nil {
					msgs, _ := h.msgRepo.ListMessages(r.Context(), conv.ID, 200)
					out := make([]map[string]interface{}, 0, len(msgs))
					for _, m := range msgs {
						body := ""
						if len(m.BodyEnc) > 0 {
							if b, err := crypto.Decrypt(m.BodyEnc, h.aesKey); err == nil {
								body = string(b)
							}
						}
						out = append(out, map[string]interface{}{
							"id": m.ID.String(), "sender_id": m.SenderID.String(),
							"body": body, "created_at": m.CreatedAt.Format(time.RFC3339),
						})
					}
					ctx["messages"] = out
				}
			}
		}
	}
	if c.TargetType == "application" && h.appRepo != nil {
		loadAppContext(c.TargetID)
		if review, err := h.appRepo.GetReviewByApplication(r.Context(), c.TargetID); err == nil {
			ctx["review"] = review
		}
	}
	if c.TargetType == "application_review" && h.appRepo != nil {
		if review, err := h.appRepo.GetReviewByID(r.Context(), c.TargetID); err == nil {
			ctx["review"] = review
			if appIDStr, ok := review["application_id"].(string); ok {
				if appID, err := uuid.Parse(appIDStr); err == nil {
					loadAppContext(appID)
				}
			}
		}
	}
	jsonOK(w, ctx)
}

func (h *StaffHandler) PatchComplaint(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	complaint, _ := h.staffRepo.GetComplaint(r.Context(), id)
	var req struct {
		Action     string `json:"action"`
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	status := "dismissed"
	switch req.Action {
	case "upheld":
		status = "upheld"
	case "warn_recruiter":
		status = "upheld"
		if complaint != nil && h.appRepo != nil {
			var recruiterID *uuid.UUID
			if complaint.TargetType == "application_review" {
				if review, err := h.appRepo.GetReviewByID(r.Context(), complaint.TargetID); err == nil {
					if ridStr, ok := review["reviewer_id"].(string); ok {
						if rid, err := uuid.Parse(ridStr); err == nil {
							recruiterID = &rid
						}
					}
				}
			} else if complaint.TargetType == "application" {
				if app, err := h.appRepo.GetByID(r.Context(), complaint.TargetID); err == nil {
					recruiterID = &app.RecruiterID
				}
			}
			if recruiterID != nil {
				msg := req.Resolution
				if msg == "" {
					msg = "Модератор предупреждает о необходимости конструктивных отзывов."
				}
				h.notifier.Notify(r.Context(), *recruiterID, "moderator_warning", "Предупреждение модератора", msg, map[string]interface{}{"complaint_id": id.String()})
			}
		}
	case "escalate":
		status = "escalated"
		taskTitle := "Жалоба требует решения администратора"
		var entityID *uuid.UUID
		entityID = &id
		_, _ = h.staffRepo.CreateStaffTask(r.Context(), claims.UserID, taskTitle, req.Resolution, "complaint", entityID)
	default:
		if req.Action != "dismiss" {
			http.Error(w, `{"error":"unknown action"}`, http.StatusBadRequest)
			return
		}
	}
	if err := h.staffRepo.ResolveComplaint(r.Context(), id, claims.UserID, status, req.Resolution); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	if complaint != nil {
		body := req.Resolution
		if body == "" {
			body = "Модератор рассмотрел вашу жалобу. Статус: " + status
		}
		h.notifier.Notify(r.Context(), complaint.ReporterID, "complaint_resolved", "Решение по жалобе", body, map[string]interface{}{"complaint_id": id.String(), "status": status})
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "moderate_complaint", "complaint", &id, map[string]interface{}{"action": req.Action})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *StaffHandler) CreateStaffTask(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		EntityType  string `json:"entity_type"`
		EntityID    string `json:"entity_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, `{"error":"title required"}`, http.StatusBadRequest)
		return
	}
	var entityID *uuid.UUID
	if req.EntityID != "" {
		if id, err := uuid.Parse(req.EntityID); err == nil {
			entityID = &id
		}
	}
	t, err := h.staffRepo.CreateStaffTask(r.Context(), claims.UserID, req.Title, req.Description, req.EntityType, entityID)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, repository.StaffTaskToMap(t))
}

func (h *StaffHandler) ListStaffTasks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}
	list, err := h.staffRepo.ListStaffTasks(r.Context(), status, 100)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		out = append(out, repository.StaffTaskToMap(&list[i]))
	}
	jsonOK(w, map[string]interface{}{"tasks": out})
}

func (h *StaffHandler) ResolveStaffTask(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Resolution string `json:"resolution"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.staffRepo.ResolveStaffTask(r.Context(), id, claims.UserID, req.Resolution); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "resolve_staff_task", "staff_task", &id, map[string]interface{}{"resolution": req.Resolution})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *StaffHandler) ModeratorStats(w http.ResponseWriter, r *http.Request) {
	v, hck, f, c, err := h.staffRepo.ModeratorDashboardCounts(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{
		"pending_vacancies": v, "pending_hackathons": hck,
		"pending_freelance": f, "open_complaints": c,
	})
}

func (h *StaffHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		key = "platform"
	}
	val, err := h.staffRepo.GetSetting(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"key": key, "value": val})
}

func (h *StaffHandler) PatchSettings(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	key := r.URL.Query().Get("key")
	if key == "" {
		key = "platform"
	}
	var req struct {
		Value map[string]interface{} `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Value == nil {
		http.Error(w, `{"error":"value required"}`, http.StatusBadRequest)
		return
	}
	if err := h.staffRepo.UpdateSetting(r.Context(), key, req.Value, claims.UserID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "update_settings", "platform_settings", nil, map[string]interface{}{"key": key})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *StaffHandler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.NewPassword) < 8 {
		http.Error(w, `{"error":"password min 8 chars"}`, http.StatusBadRequest)
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	if err := h.authSecRepo.UpdatePassword(r.Context(), id, hash); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	_ = h.authSecRepo.RevokeAllUserTokens(r.Context(), id)
	user, _ := h.userRepo.GetByID(r.Context(), id)
	if user != nil {
		h.notifier.Notify(r.Context(), id, "password_reset", "Пароль сброшен", "Администратор сбросил пароль вашего аккаунта", nil)
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "admin_reset_password", "user", &id, nil)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *StaffHandler) AdminGrantSubscription(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	recruiterID, err := uuid.Parse(r.URL.Query().Get("recruiter_id"))
	if err != nil {
		http.Error(w, `{"error":"recruiter_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Plan          string `json:"plan"`
		Months        int    `json:"months"`
		StartsAt      string `json:"starts_at"`
		EndsAt        string `json:"ends_at"`
		Reason        string `json:"reason"`
		PaymentMethod string `json:"payment_method"`
		AmountKZT     int    `json:"amount_kzt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Plan == "" {
		http.Error(w, `{"error":"plan required"}`, http.StatusBadRequest)
		return
	}
	act, ok := billing.ResolvePlan(req.Plan, billing.PlanPricing{})
	if !ok {
		http.Error(w, `{"error":"unknown plan"}`, http.StatusBadRequest)
		return
	}
	startsAt := time.Now()
	if req.StartsAt != "" {
		if t, err := time.Parse(time.RFC3339, req.StartsAt); err == nil {
			startsAt = t
		}
	}
	expires := startsAt.AddDate(0, 1, 0)
	if req.EndsAt != "" {
		if t, err := time.Parse(time.RFC3339, req.EndsAt); err == nil {
			expires = t
		}
	} else if req.Months > 0 {
		expires = startsAt.AddDate(0, req.Months, 0)
	}
	if req.PaymentMethod == "" {
		req.PaymentMethod = "bank_transfer"
	}
	amount := req.AmountKZT
	if amount <= 0 {
		amount = act.PriceKZT
	}
	if err := h.billingRepo.SetRecruiterPlanWithMeta(r.Context(), recruiterID, act.Plan, startsAt, expires, act.PubQuota, act.InvQuota, "admin_manual"); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"recruiter not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	eventMeta := map[string]interface{}{
		"manual": true, "plan": req.Plan, "starts_at": startsAt.Format(time.RFC3339),
		"ends_at": expires.Format(time.RFC3339), "reason": req.Reason,
		"payment_method": req.PaymentMethod, "amount_kzt": amount,
	}
	_ = h.billingRepo.InsertEvent(r.Context(), recruiterID, "subscription_activated", eventMeta)
	if h.paymentRepo != nil {
		_, _ = h.paymentRepo.CreateCompletedSession(r.Context(), recruiterID, "manual", amount, "subscription", map[string]interface{}{
			"manually_activated": true, "plan": req.Plan, "reason": req.Reason,
			"payment_method": req.PaymentMethod, "starts_at": startsAt.Format(time.RFC3339),
			"ends_at": expires.Format(time.RFC3339),
		})
	}
	body := "Администратор активировал тариф " + act.Name + " до " + expires.Format("02.01.2006")
	if req.Reason != "" {
		body += ". Причина: " + req.Reason
	}
	h.notifier.Notify(r.Context(), recruiterID, "subscription_activated", "Подписка активирована", body,
		map[string]interface{}{"plan": req.Plan, "starts_at": startsAt, "ends_at": expires})
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "grant_subscription_manual", "recruiter", &recruiterID, eventMeta)
	jsonOK(w, map[string]string{"status": "ok"})
}
