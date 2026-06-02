package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type NotificationHandler struct {
	repo        *repository.NotificationRepository
	paymentRepo *repository.PaymentRepository
}

func NewNotificationHandler(repo *repository.NotificationRepository, paymentRepo *repository.PaymentRepository) *NotificationHandler {
	return &NotificationHandler{repo: repo, paymentRepo: paymentRepo}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	list, err := h.repo.ListByUser(r.Context(), claims.UserID, 50)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	unread, _ := h.repo.UnreadCount(r.Context(), claims.UserID)
	type nResp struct {
		ID        string  `json:"id"`
		Type      string  `json:"type"`
		Title     string  `json:"title"`
		Body      string  `json:"body"`
		ReadAt    *string `json:"read_at,omitempty"`
		CreatedAt string  `json:"created_at"`
	}
	out := make([]nResp, 0, len(list))
	for _, n := range list {
		var readAt *string
		if n.ReadAt != nil {
			s := n.ReadAt.Format("2006-01-02T15:04:05Z07:00")
			readAt = &s
		}
		out = append(out, nResp{
			ID: n.ID.String(), Type: n.Type, Title: n.Title, Body: n.Body,
			ReadAt: readAt, CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	jsonOK(w, map[string]interface{}{"notifications": out, "unread_count": unread})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		IDs []string `json:"ids"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var ids []uuid.UUID
	for _, s := range req.IDs {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}
	if err := h.repo.MarkRead(r.Context(), claims.UserID, ids); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) Preferences(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if r.Method == http.MethodGet {
		prefs, err := h.repo.GetPreferences(r.Context(), claims.UserID)
		if err != nil {
			http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]interface{}{"preferences": prefs})
		return
	}
	var req struct {
		NotificationType string `json:"notification_type"`
		ChannelInApp     bool   `json:"channel_in_app"`
		ChannelEmail     bool   `json:"channel_email"`
		ChannelPush      bool   `json:"channel_push"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	p := model.NotificationPreference{
		UserID: claims.UserID, NotificationType: req.NotificationType,
		ChannelInApp: req.ChannelInApp, ChannelEmail: req.ChannelEmail, ChannelPush: req.ChannelPush,
	}
	if err := h.repo.UpsertPreference(r.Context(), p); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) PushSubscribe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		Endpoint string `json:"endpoint"`
		P256dh   string `json:"p256dh"`
		AuthKey  string `json:"auth_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" {
		http.Error(w, `{"error":"endpoint required"}`, http.StatusBadRequest)
		return
	}
	if h.paymentRepo == nil {
		http.Error(w, `{"error":"push not configured"}`, http.StatusServiceUnavailable)
		return
	}
	if err := h.paymentRepo.UpsertPushSubscription(r.Context(), claims.UserID, req.Endpoint, req.P256dh, req.AuthKey); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

type WalletHandler struct {
	repo     *repository.WalletRepository
	notifier *notifier.Service
}

func NewWalletHandler(repo *repository.WalletRepository, notifier *notifier.Service) *WalletHandler {
	return &WalletHandler{repo: repo, notifier: notifier}
}

func (h *WalletHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	bal, err := h.repo.GetBalance(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	txs, _ := h.repo.ListTransactions(r.Context(), claims.UserID, 20)
	jsonOK(w, map[string]interface{}{"balance_kzt": bal, "recent_transactions": txs})
}

func (h *WalletHandler) Transactions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	txs, err := h.repo.ListTransactions(r.Context(), claims.UserID, limit)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"transactions": txs})
}

func (h *WalletHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		AmountKZT float64 `json:"amount_kzt"`
		CardLast4 string  `json:"card_last4"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AmountKZT < 5000 {
		http.Error(w, `{"error":"minimum withdrawal is 5000 KZT"}`, http.StatusBadRequest)
		return
	}
	wd, err := h.repo.CreateWithdrawal(r.Context(), claims.UserID, req.AmountKZT, req.CardLast4)
	if err != nil {
		http.Error(w, `{"error":"insufficient balance"}`, http.StatusBadRequest)
		return
	}
	h.notifier.Notify(r.Context(), claims.UserID, "withdrawal_pending", "Заявка на вывод", "Обрабатывается 1-3 рабочих дня", nil)
	jsonOK(w, map[string]interface{}{"id": wd.ID.String(), "status": wd.Status})
}

type AdminHandler struct {
	userRepo    *repository.UserRepository
	walletRepo  *repository.WalletRepository
	auditRepo   *repository.AuditRepository
	billingRepo *repository.BillingRepository
	paymentRepo *repository.PaymentRepository
}

func NewAdminHandler(userRepo *repository.UserRepository, walletRepo *repository.WalletRepository, auditRepo *repository.AuditRepository, billingRepo *repository.BillingRepository, paymentRepo *repository.PaymentRepository) *AdminHandler {
	return &AdminHandler{userRepo: userRepo, walletRepo: walletRepo, auditRepo: auditRepo, billingRepo: billingRepo, paymentRepo: paymentRepo}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if h.paymentRepo == nil {
		jsonOK(w, map[string]interface{}{"active_students_30d": 0})
		return
	}
	stats, err := h.paymentRepo.AdminDashboardStats(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, stats)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	users, err := h.userRepo.Search(r.Context(), q, 50)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"users": users})
}

func (h *AdminHandler) PatchUser(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Role      string `json:"role"`
		IsBlocked *bool  `json:"is_blocked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if req.Role != "" {
		_ = h.userRepo.UpdateRole(r.Context(), id, model.UserRole(req.Role))
	}
	if req.IsBlocked != nil {
		_ = h.userRepo.SetBlocked(r.Context(), id, *req.IsBlocked)
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "admin_update_user", "user", &id, map[string]interface{}{"role": req.Role, "blocked": req.IsBlocked})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *AdminHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	entries, err := h.auditRepo.List(r.Context(), 50, offset)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"entries": entries})
}

func (h *AdminHandler) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.walletRepo.ListWithdrawals(r.Context(), status, 100)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"withdrawals": list})
}

func (h *AdminHandler) ProcessWithdrawal(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Approve bool `json:"approve"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.walletRepo.ProcessWithdrawal(r.Context(), id, claims.UserID, req.Approve); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "process_withdrawal", "withdrawal", &id, map[string]interface{}{"approve": req.Approve})
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *AdminHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	if h.paymentRepo == nil {
		jsonOK(w, map[string]interface{}{"transactions": []interface{}{}, "escrow_held_kzt": 0})
		return
	}
	list, escrow, err := h.paymentRepo.ListAdminTransactions(r.Context(), 100)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"transactions": list, "escrow_held_kzt": escrow})
}

func (h *AdminHandler) PatchTariff(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		BasicKZT    int `json:"basic_kzt"`
		StandardKZT int `json:"standard_kzt"`
		PremiumKZT  int `json:"premium_kzt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if err := h.paymentRepo.UpdateTariffPrices(r.Context(), req.BasicKZT, req.StandardKZT, req.PremiumKZT); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "update_tariffs", "platform", nil, map[string]interface{}{
		"basic": req.BasicKZT, "standard": req.StandardKZT, "premium": req.PremiumKZT,
	})
	jsonOK(w, map[string]string{"status": "ok"})
}
