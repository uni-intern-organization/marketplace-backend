package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/payment"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type BillingHandler struct {
	billingRepo   *repository.BillingRepository
	recruiterRepo *repository.RecruiterProfileRepository
	billingSvc    *billing.Service
	vacancyRepo   *repository.VacancyRepository
	paymentRepo   *repository.PaymentRepository
	paymentProv   payment.PaymentProvider
	cfg           *config.BillingConfig
}

func NewBillingHandler(
	billingRepo *repository.BillingRepository,
	recruiterRepo *repository.RecruiterProfileRepository,
	billingSvc *billing.Service,
	vacancyRepo *repository.VacancyRepository,
	paymentRepo *repository.PaymentRepository,
	paymentProv payment.PaymentProvider,
	cfg *config.BillingConfig,
) *BillingHandler {
	return &BillingHandler{
		billingRepo: billingRepo, recruiterRepo: recruiterRepo, billingSvc: billingSvc,
		vacancyRepo: vacancyRepo, paymentRepo: paymentRepo, paymentProv: paymentProv, cfg: cfg,
	}
}

type planInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	PriceKZT    int      `json:"price_kzt"`
	Period      string   `json:"period"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
}

func (h *BillingHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	plans := []planInfo{
		{ID: "free", Name: "Free", PriceKZT: 0, Period: "month", Description: "Одна базовая вакансия", Features: []string{"post_one_vacancy", "view_applications"}},
		{ID: "starter", Name: "Стартовая", PriceKZT: 30000, Period: "month", Description: "3 standard-публикации в месяц", Features: []string{"3_standard_posts", "basic_search", "email_notifications"}},
		{ID: "business", Name: "Бизнес", PriceKZT: 80000, Period: "month", Description: "10 публикаций, расширенный поиск", Features: append(billing.ProFeatureLabels(), "advanced_search", "50_invitations")},
		{ID: "corporate", Name: "Корпоративная", PriceKZT: 200000, Period: "month", Description: "Индивидуальный пакет", Features: append(billing.ProFeatureLabels(), "unlimited_posts", "dedicated_manager")},
		{ID: "pro", Name: "Pro (legacy)", PriceKZT: 15000, Period: "month", Description: "Полный доступ", Features: billing.ProFeatureLabels()},
	}
	tiers := map[string]int{
		"basic":    h.cfg.VacancyTierBasicKZT,
		"standard": h.cfg.VacancyTierStandardKZT,
		"premium":  h.cfg.VacancyTierPremiumKZT,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"plans": plans,
		"vacancy_tiers": tiers,
		"promotion": map[string]interface{}{
			"id": "vacancy_boost", "name": "Продвижение (legacy)", "price_kzt": 5000, "period": "7_days",
		},
	})
}

func (h *BillingHandler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || (claims.Role != model.RoleRecruiter && claims.Role != model.RoleAdmin) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, `{"error":"failed to load billing"}`, http.StatusInternalServerError)
		return
	}
	resp := map[string]interface{}{
		"plan": ent.Plan, "plan_expires_at": formatTimePtr(ent.PlanExpiresAt),
		"vacancy_count": ent.VacancyCount, "max_vacancies": ent.MaxVacancies,
		"is_pro": ent.IsPro, "features": ent.Features,
		"publications_quota": ent.PublicationsQuota, "publications_used": ent.PublicationsUsed,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type subscribeRequest struct {
	Plan string `json:"plan"`
}

func (h *BillingHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	expires := time.Now().Add(30 * 24 * time.Hour)
	pubQuota := 0
	invQuota := 0
	plan := model.RecruiterPlanPro
	eventType := "subscribe_pro"
	switch req.Plan {
	case "starter":
		plan = model.RecruiterPlanStarter
		pubQuota = 3
		eventType = "subscribe_starter"
	case "business":
		plan = model.RecruiterPlanBusiness
		pubQuota = 10
		invQuota = 50
		eventType = "subscribe_business"
	case "corporate":
		plan = model.RecruiterPlanCorporate
		pubQuota = -1
		invQuota = -1
		eventType = "subscribe_corporate"
	case "pro", "pro_monthly_5", "":
		pubQuota = 0
		if req.Plan == "pro_monthly_5" {
			pubQuota = 5
			eventType = "subscribe_pro_monthly_5"
		}
	default:
		http.Error(w, `{"error":"unknown plan"}`, http.StatusBadRequest)
		return
	}
	if err := h.billingRepo.SetRecruiterPlan(r.Context(), claims.UserID, plan, expires, pubQuota, invQuota); err != nil {
		http.Error(w, `{"error":"failed to activate plan"}`, http.StatusInternalServerError)
		return
	}
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, eventType, map[string]interface{}{
		"plan": req.Plan, "expires_at": expires.Format(time.RFC3339), "publications_quota": pubQuota, "demo": true,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok", "plan": req.Plan, "plan_expires_at": expires.Format(time.RFC3339), "publications_quota": pubQuota,
	})
}

type publishVacancyRequest struct {
	VacancyID   string `json:"vacancy_id"`
	ListingTier string `json:"listing_tier"`
}

func (h *BillingHandler) PublishVacancy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req publishVacancyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	vacancyID, err := uuid.Parse(req.VacancyID)
	if err != nil {
		http.Error(w, `{"error":"invalid vacancy_id"}`, http.StatusBadRequest)
		return
	}
	tier := model.ListingTierBasic
	switch req.ListingTier {
	case "standard":
		tier = model.ListingTierStandard
	case "premium":
		tier = model.ListingTierPremium
	}
	if err := h.vacancyRepo.SetTier(r.Context(), vacancyID, claims.UserID, tier); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"vacancy not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	ok, code := h.billingSvc.CanPublishTier(r.Context(), claims.UserID, claims.Role, tier)
	if !ok {
		billing.WriteError(w, http.StatusForbidden, code, "cannot publish at this tier")
		return
	}
	credits := billing.PublicationCreditsForTier(tier)
	if err := h.vacancyRepo.SubmitForModeration(r.Context(), vacancyID, claims.UserID, tier); err != nil {
		http.Error(w, `{"error":"failed to submit"}`, http.StatusInternalServerError)
		return
	}
	price := h.cfg.VacancyTierBasicKZT
	switch tier {
	case model.ListingTierStandard:
		price = h.cfg.VacancyTierStandardKZT
		for i := 0; i < credits; i++ {
			_ = h.recruiterRepo.IncrementPublicationsUsed(r.Context(), claims.UserID)
		}
	case model.ListingTierPremium:
		price = h.cfg.VacancyTierPremiumKZT
		for i := 0; i < credits; i++ {
			_ = h.recruiterRepo.IncrementPublicationsUsed(r.Context(), claims.UserID)
		}
	}
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "vacancy_tier", map[string]interface{}{
		"vacancy_id": req.VacancyID, "tier": req.ListingTier, "price_kzt": price, "demo": true,
	})
	if tier == model.ListingTierStandard || tier == model.ListingTierPremium {
		_ = h.billingRepo.LogNotification(r.Context(), claims.UserID, "email", "Vacancy newsletter", "Demo: students notified about "+req.ListingTier+" vacancy")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "listing_tier": req.ListingTier, "price_kzt": price, "vacancy_status": "pending_review"})
}

type promoteRequest struct {
	VacancyID string `json:"vacancy_id"`
}

func (h *BillingHandler) PromoteVacancy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req promoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	vacancyID, err := uuid.Parse(req.VacancyID)
	if err != nil {
		http.Error(w, `{"error":"invalid vacancy_id"}`, http.StatusBadRequest)
		return
	}
	until := time.Now().Add(7 * 24 * time.Hour)
	if err := h.billingRepo.PromoteVacancy(r.Context(), vacancyID, claims.UserID, until); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"vacancy not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed to promote"}`, http.StatusInternalServerError)
		return
	}
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "promote_vacancy", map[string]interface{}{
		"vacancy_id": req.VacancyID, "featured_until": until.Format(time.RFC3339), "demo": true,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "featured_until": until.Format(time.RFC3339)})
}

func (h *BillingHandler) PublishHackathon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		HackathonID string `json:"hackathon_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "hackathon_listing", map[string]interface{}{
		"hackathon_id": req.HackathonID, "demo": true,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *BillingHandler) Analytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
	if err != nil {
		http.Error(w, `{"error":"failed to load billing"}`, http.StatusInternalServerError)
		return
	}
	if !ent.CanAnalytics {
		billing.WriteError(w, http.StatusForbidden, "subscription_required", "analytics requires Pro subscription")
		return
	}
	stats, err := h.billingRepo.GetAnalytics(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"analytics failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func (h *BillingHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Purpose  string                 `json:"purpose"`
		AmountKZT int                   `json:"amount_kzt"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Purpose == "" || req.AmountKZT <= 0 {
		http.Error(w, `{"error":"purpose and amount_kzt required"}`, http.StatusBadRequest)
		return
	}
	if h.paymentProv == nil {
		http.Error(w, `{"error":"payment not configured"}`, http.StatusServiceUnavailable)
		return
	}
	result, err := h.paymentProv.CreateCheckout(r.Context(), payment.CheckoutRequest{
		RecruiterID: claims.UserID, AmountKZT: req.AmountKZT, Purpose: req.Purpose, Metadata: req.Metadata,
	})
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "checkout failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{
		"session_id": result.SessionID.String(), "external_id": result.ExternalID,
		"payment_url": result.PaymentURL, "status": result.Status,
	})
}

func (h *BillingHandler) ApplyPromo(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, `{"error":"code required"}`, http.StatusBadRequest)
		return
	}
	discount, err := h.paymentRepo.ApplyPromo(r.Context(), req.Code)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"invalid or expired promo"}`, http.StatusBadRequest)
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"discount_percent": discount, "status": "ok"})
}

func (h *BillingHandler) ListPaymentMethods(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	list, err := h.paymentRepo.ListMethods(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, m := range list {
		out = append(out, map[string]interface{}{
			"id": m.ID.String(), "provider": m.Provider, "last4": m.Last4, "brand": m.Brand,
			"created_at": m.CreatedAt.Format(time.RFC3339),
		})
	}
	jsonOK(w, map[string]interface{}{"payment_methods": out})
}

func (h *BillingHandler) AddPaymentMethod(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Provider  string `json:"provider"`
		TokenRef  string `json:"token_ref"`
		Last4     string `json:"last4"`
		Brand     string `json:"brand"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TokenRef == "" {
		http.Error(w, `{"error":"token_ref required"}`, http.StatusBadRequest)
		return
	}
	if req.Provider == "" {
		req.Provider = "demo"
	}
	m, err := h.paymentRepo.AddMethod(r.Context(), claims.UserID, req.Provider, req.TokenRef, req.Last4, req.Brand)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]interface{}{"id": m.ID.String(), "last4": m.Last4, "brand": m.Brand})
}

func (h *BillingHandler) DeletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if err := h.paymentRepo.DeleteMethod(r.Context(), id, claims.UserID); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
