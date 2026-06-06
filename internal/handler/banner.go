package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/payment"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	"github.com/uni-intern-organization/marketplace-backend/internal/storage"
)

type BannerHandler struct {
	repo        *repository.BannerRepository
	billingRepo *repository.BillingRepository
	paymentRepo *repository.PaymentRepository
	paymentProv payment.PaymentProvider
	auditRepo   *repository.AuditRepository
	staffRepo   *repository.StaffRepository
	notifier    *notifier.Service
	storage     *storage.S3Storage
}

func NewBannerHandler(
	repo *repository.BannerRepository,
	billingRepo *repository.BillingRepository,
	paymentRepo *repository.PaymentRepository,
	paymentProv payment.PaymentProvider,
	auditRepo *repository.AuditRepository,
	staffRepo *repository.StaffRepository,
	notifier *notifier.Service,
	s *storage.S3Storage,
) *BannerHandler {
	return &BannerHandler{
		repo: repo, billingRepo: billingRepo, paymentRepo: paymentRepo, paymentProv: paymentProv,
		auditRepo: auditRepo, staffRepo: staffRepo, notifier: notifier, storage: s,
	}
}

type bannerCampaignResp struct {
	ID             string  `json:"id"`
	PlacementCode  string  `json:"placement_code"`
	RecruiterID    *string `json:"recruiter_id,omitempty"`
	RecruiterEmail *string `json:"recruiter_email,omitempty"`
	ImageKey       string  `json:"image_key"`
	ImageURL       string  `json:"image_url,omitempty"`
	LinkURL        string  `json:"link_url"`
	StartsAt       string  `json:"starts_at"`
	EndsAt         string  `json:"ends_at"`
	Status         string  `json:"status"`
	AmountKZT      int     `json:"amount_kzt"`
	RejectReason   *string `json:"reject_reason,omitempty"`
	Impressions    int64   `json:"impressions"`
	Clicks         int64   `json:"clicks"`
	Priority       int     `json:"priority"`
	CreatedAt      string  `json:"created_at"`
}

func campaignToResp(c *model.BannerCampaign, imageURL string, recruiterEmail string) bannerCampaignResp {
	r := bannerCampaignResp{
		ID: c.ID.String(), PlacementCode: c.PlacementCode, ImageKey: c.ImageKey, ImageURL: imageURL,
		LinkURL: c.LinkURL, StartsAt: c.StartsAt.Format(time.RFC3339), EndsAt: c.EndsAt.Format(time.RFC3339),
		Status: c.Status, AmountKZT: c.AmountKZT, RejectReason: c.RejectReason,
		Impressions: c.Impressions, Clicks: c.Clicks, Priority: c.Priority,
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
	}
	if c.RecruiterID != nil {
		s := c.RecruiterID.String()
		r.RecruiterID = &s
		if recruiterEmail != "" {
			r.RecruiterEmail = &recruiterEmail
		}
	}
	return r
}

func (h *BannerHandler) toCampaignResp(ctx context.Context, c *model.BannerCampaign, imageURL string) bannerCampaignResp {
	email := ""
	if c.RecruiterID != nil {
		email, _ = h.repo.GetRecruiterInfo(ctx, *c.RecruiterID)
	}
	return campaignToResp(c, imageURL, email)
}

func (h *BannerHandler) presignImage(ctx context.Context, key string) string {
	return h.bannerImageURL(key)
}

// bannerImageURL serves images via API proxy (MinIO is not reachable from the browser).
func (h *BannerHandler) bannerImageURL(key string) string {
	if key == "" {
		return ""
	}
	return "/api/public/banners/asset?key=" + url.QueryEscape(key)
}

func (h *BannerHandler) PublicAsset(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		http.Error(w, `{"error":"storage unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" || !strings.HasPrefix(key, "banners/") {
		http.Error(w, `{"error":"invalid key"}`, http.StatusBadRequest)
		return
	}
	body, contentType, err := h.storage.Get(r.Context(), key)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	defer body.Close()
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = io.Copy(w, body)
}

func validBannerLink(link string) bool {
	if link == "" {
		return false
	}
	if strings.HasPrefix(link, "/") {
		return true
	}
	u, err := url.Parse(link)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func parseBannerDates(startsS, endsS string) (time.Time, time.Time, error) {
	starts, err := time.Parse(time.RFC3339, startsS)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	ends, err := time.Parse(time.RFC3339, endsS)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !ends.After(starts) {
		return time.Time{}, time.Time{}, pgx.ErrNoRows
	}
	return starts, ends, nil
}

// PublicGetActive serves active banner for a placement.
func (h *BannerHandler) PublicGetActive(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("placement")
	if code == "" {
		http.Error(w, `{"error":"placement required"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListActiveForPlacement(r.Context(), code, 20)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	banners := make([]bannerCampaignResp, 0, len(list))
	for i := range list {
		img := h.presignImage(r.Context(), list[i].ImageKey)
		banners = append(banners, h.toCampaignResp(r.Context(), &list[i], img))
	}
	var first *bannerCampaignResp
	if len(banners) > 0 {
		first = &banners[0]
	}
	jsonOK(w, map[string]interface{}{"banners": banners, "banner": first})
}

func (h *BannerHandler) PublicImpression(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	_ = h.repo.IncrementImpressions(r.Context(), id)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *BannerHandler) PublicClick(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	_ = h.repo.IncrementClicks(r.Context(), id)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *BannerHandler) ListPlacements(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	placements, err := h.repo.ListPlacements(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	type slot struct {
		model.BannerPlacement
		OccupiedUntil *string `json:"occupied_until,omitempty"`
		IsFree        bool    `json:"is_free"`
	}
	out := make([]slot, 0, len(placements))
	for _, p := range placements {
		s := slot{BannerPlacement: p, IsFree: true}
		if until, _ := h.repo.GetOccupiedUntil(r.Context(), p.Code); until != nil {
			t := until.Format(time.RFC3339)
			s.OccupiedUntil = &t
			s.IsFree = false
		}
		out = append(out, s)
	}
	jsonOK(w, map[string]interface{}{"placements": out})
}

func (h *BannerHandler) Quote(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	code := r.URL.Query().Get("placement")
	startsS := r.URL.Query().Get("starts_at")
	endsS := r.URL.Query().Get("ends_at")
	p, err := h.repo.GetPlacement(r.Context(), code)
	if err != nil {
		http.Error(w, `{"error":"unknown placement"}`, http.StatusBadRequest)
		return
	}
	starts, ends, err := parseBannerDates(startsS, endsS)
	if err != nil {
		http.Error(w, `{"error":"invalid dates"}`, http.StatusBadRequest)
		return
	}
	amount := repository.CalcBannerPrice(p, starts, ends)
	jsonOK(w, map[string]interface{}{"amount_kzt": amount, "placement": code})
}

func (h *BannerHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	list, err := h.repo.ListByRecruiter(r.Context(), claims.UserID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	resp := make([]bannerCampaignResp, 0, len(list))
	for i := range list {
		img := h.presignImage(r.Context(), list[i].ImageKey)
		resp = append(resp, h.toCampaignResp(r.Context(), &list[i], img))
	}
	jsonOK(w, map[string]interface{}{"campaigns": resp})
}

func (h *BannerHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		PlacementCode string `json:"placement_code"`
		ImageKey      string `json:"image_key"`
		LinkURL       string `json:"link_url"`
		StartsAt      string `json:"starts_at"`
		EndsAt        string `json:"ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlacementCode == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	p, err := h.repo.GetPlacement(r.Context(), req.PlacementCode)
	if err != nil {
		http.Error(w, `{"error":"unknown placement"}`, http.StatusBadRequest)
		return
	}
	starts, ends, err := parseBannerDates(req.StartsAt, req.EndsAt)
	if err != nil {
		http.Error(w, `{"error":"invalid dates"}`, http.StatusBadRequest)
		return
	}
	if req.LinkURL != "" && !validBannerLink(req.LinkURL) {
		http.Error(w, `{"error":"invalid link_url"}`, http.StatusBadRequest)
		return
	}
	rid := claims.UserID
	amount := repository.CalcBannerPrice(p, starts, ends)
	c := &model.BannerCampaign{
		PlacementCode: req.PlacementCode, RecruiterID: &rid, CreatedBy: claims.UserID,
		ImageKey: req.ImageKey, LinkURL: req.LinkURL, StartsAt: starts, EndsAt: ends,
		Status: model.BannerStatusDraft, AmountKZT: amount,
	}
	if err := h.repo.CreateCampaign(r.Context(), c); err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	img := h.presignImage(r.Context(), c.ImageKey)
	jsonOK(w, h.toCampaignResp(r.Context(), c, img))
}

func (h *BannerHandler) Patch(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.repo.GetCampaign(r.Context(), id)
	if err != nil || c.RecruiterID == nil || *c.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		ImageKey string `json:"image_key"`
		LinkURL  string `json:"link_url"`
		StartsAt string `json:"starts_at"`
		EndsAt   string `json:"ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	starts, ends, err := parseBannerDates(req.StartsAt, req.EndsAt)
	if err != nil {
		http.Error(w, `{"error":"invalid dates"}`, http.StatusBadRequest)
		return
	}
	if req.LinkURL != "" && !validBannerLink(req.LinkURL) {
		http.Error(w, `{"error":"invalid link_url"}`, http.StatusBadRequest)
		return
	}
	p, _ := h.repo.GetPlacement(r.Context(), c.PlacementCode)
	amount := repository.CalcBannerPrice(p, starts, ends)
	if err := h.repo.UpdateCampaignDraft(r.Context(), id, req.ImageKey, req.LinkURL, starts, ends, amount); err != nil {
		RespondError(w, http.StatusInternalServerError, "update failed", err)
		return
	}
	updated, _ := h.repo.GetCampaign(r.Context(), id)
	img := h.presignImage(r.Context(), updated.ImageKey)
	jsonOK(w, h.toCampaignResp(r.Context(), updated, img))
}

func (h *BannerHandler) Submit(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.repo.GetCampaign(r.Context(), id)
	if err != nil || c.RecruiterID == nil || *c.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if c.ImageKey == "" || c.LinkURL == "" {
		http.Error(w, `{"error":"image and link required"}`, http.StatusBadRequest)
		return
	}
	if h.paymentProv == nil {
		http.Error(w, `{"error":"payment not configured"}`, http.StatusServiceUnavailable)
		return
	}
	result, err := h.paymentProv.CreateCheckout(r.Context(), payment.CheckoutRequest{
		RecruiterID: claims.UserID, AmountKZT: c.AmountKZT, Purpose: "banner_campaign",
		Metadata: map[string]interface{}{
			"campaign_id": c.ID.String(), "placement": c.PlacementCode,
			"starts_at": c.StartsAt.Format(time.RFC3339), "ends_at": c.EndsAt.Format(time.RFC3339),
		},
	})
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "checkout failed", err)
		return
	}
	if err := h.repo.SetPaymentSession(r.Context(), id, result.SessionID, c.AmountKZT); err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "banner_submitted", map[string]interface{}{
		"campaign_id": id.String(), "amount_kzt": c.AmountKZT,
	})
	if h.notifier != nil {
		h.notifier.Notify(r.Context(), claims.UserID, "banner_submitted", "Заявка на баннер отправлена",
			"Ожидайте проверки администратором", map[string]interface{}{"campaign_id": id.String()})
	}
	jsonOK(w, map[string]interface{}{
		"status": "pending_review", "session_id": result.SessionID.String(), "amount_kzt": c.AmountKZT,
	})
}

func (h *BannerHandler) GetOne(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	c, err := h.repo.GetCampaign(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if claims != nil && claims.Role == model.RoleRecruiter {
		if c.RecruiterID == nil || *c.RecruiterID != claims.UserID {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
	} else if claims == nil || claims.Role != model.RoleAdmin {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	img := h.presignImage(r.Context(), c.ImageKey)
	jsonOK(w, h.toCampaignResp(r.Context(), c, img))
}

// AdminList returns all campaigns and placement map.
func (h *BannerHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListAll(r.Context(), "", 200)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	placements, _ := h.repo.ListPlacements(r.Context())
	resp := make([]bannerCampaignResp, 0, len(list))
	for i := range list {
		img := h.presignImage(r.Context(), list[i].ImageKey)
		resp = append(resp, h.toCampaignResp(r.Context(), &list[i], img))
	}
	jsonOK(w, map[string]interface{}{"campaigns": resp, "placements": placements})
}

func (h *BannerHandler) AdminQueue(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListAll(r.Context(), model.BannerStatusPendingReview, 100)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	resp := make([]bannerCampaignResp, 0, len(list))
	for i := range list {
		img := h.presignImage(r.Context(), list[i].ImageKey)
		resp = append(resp, h.toCampaignResp(r.Context(), &list[i], img))
	}
	jsonOK(w, map[string]interface{}{"campaigns": resp})
}

func (h *BannerHandler) AdminReview(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Action   string `json:"action"`
		Reason   string `json:"reason"`
		StartsAt string `json:"starts_at"`
		EndsAt   string `json:"ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	c, err := h.repo.GetCampaign(r.Context(), id)
	if err != nil || c.Status != model.BannerStatusPendingReview {
		http.Error(w, `{"error":"not found or not pending"}`, http.StatusNotFound)
		return
	}
	switch req.Action {
	case "approve":
		startsAt := c.StartsAt
		endsAt := c.EndsAt
		if req.StartsAt != "" {
			if t, err := time.Parse(time.RFC3339, req.StartsAt); err == nil {
				startsAt = t
			}
		}
		if req.EndsAt != "" {
			if t, err := time.Parse(time.RFC3339, req.EndsAt); err == nil {
				endsAt = t
			}
		}
		now := time.Now()
		if startsAt.Before(now) {
			startsAt = now
		}
		if !endsAt.After(startsAt) {
			http.Error(w, `{"error":"invalid dates"}`, http.StatusBadRequest)
			return
		}
		if !startsAt.Equal(c.StartsAt) || !endsAt.Equal(c.EndsAt) {
			_ = h.repo.UpdateCampaignPeriod(r.Context(), id, startsAt, endsAt)
			c.StartsAt = startsAt
			c.EndsAt = endsAt
		}
		if c.PaymentSessionID != nil && h.paymentRepo != nil {
			_ = h.paymentRepo.CompleteSession(r.Context(), *c.PaymentSessionID)
		}
		_ = h.repo.SetCampaignStatus(r.Context(), id, model.BannerStatusActive, nil)
		if c.RecruiterID != nil {
			_ = h.billingRepo.InsertEvent(r.Context(), *c.RecruiterID, "banner_purchase", map[string]interface{}{
				"campaign_id": id.String(), "amount_kzt": c.AmountKZT,
				"starts_at": startsAt.Format(time.RFC3339), "ends_at": endsAt.Format(time.RFC3339),
			})
			if h.notifier != nil {
				h.notifier.Notify(r.Context(), *c.RecruiterID, "banner_approved", "Баннер одобрен",
					"Ваш баннер будет показан с "+startsAt.Format("02.01.2006"),
					map[string]interface{}{"campaign_id": id.String(), "starts_at": startsAt.Format(time.RFC3339)})
			}
		}
	case "reject":
		reason := req.Reason
		if reason == "" {
			reason = "Не соответствует правилам платформы"
		}
		if c.PaymentSessionID != nil && h.paymentRepo != nil {
			_ = h.paymentRepo.FailSession(r.Context(), *c.PaymentSessionID)
		}
		if c.RecruiterID != nil {
			_ = h.billingRepo.CreditWallet(r.Context(), *c.RecruiterID, float64(c.AmountKZT))
		}
		_ = h.repo.SetCampaignStatus(r.Context(), id, model.BannerStatusRejected, &reason)
		if c.RecruiterID != nil && h.notifier != nil {
			h.notifier.Notify(r.Context(), *c.RecruiterID, "banner_rejected", "Баннер отклонён", reason,
				map[string]interface{}{"campaign_id": id.String()})
		}
	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusBadRequest)
		return
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "review_banner", "banner_campaign", &id, map[string]interface{}{"action": req.Action})
	resp := map[string]string{"status": "ok"}
	if req.Action == "approve" {
		resp["starts_at"] = c.StartsAt.Format(time.RFC3339)
		resp["ends_at"] = c.EndsAt.Format(time.RFC3339)
	}
	jsonOK(w, resp)
}

func (h *BannerHandler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		PlacementCode string `json:"placement_code"`
		ImageKey      string `json:"image_key"`
		LinkURL       string `json:"link_url"`
		StartsAt      string `json:"starts_at"`
		EndsAt        string `json:"ends_at"`
		Priority      int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlacementCode == "" || req.ImageKey == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if !validBannerLink(req.LinkURL) {
		http.Error(w, `{"error":"invalid link_url"}`, http.StatusBadRequest)
		return
	}
	starts, ends, err := parseBannerDates(req.StartsAt, req.EndsAt)
	if err != nil {
		http.Error(w, `{"error":"invalid dates"}`, http.StatusBadRequest)
		return
	}
	c := &model.BannerCampaign{
		PlacementCode: req.PlacementCode, RecruiterID: nil, CreatedBy: claims.UserID,
		ImageKey: req.ImageKey, LinkURL: req.LinkURL, StartsAt: starts, EndsAt: ends,
		Status: model.BannerStatusActive, Priority: req.Priority,
	}
	if err := h.repo.CreateCampaign(r.Context(), c); err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	actor := claims.UserID
	_ = h.auditRepo.Log(r.Context(), &actor, "create_platform_banner", "banner_campaign", &c.ID, nil)
	img := h.presignImage(r.Context(), c.ImageKey)
	jsonOK(w, h.toCampaignResp(r.Context(), c, img))
}

func (h *BannerHandler) AdminPatchPlacement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code     string `json:"code"`
		WeekKZT  int    `json:"price_week_kzt"`
		MonthKZT int    `json:"price_month_kzt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdatePlacementPricing(r.Context(), req.Code, req.WeekKZT, req.MonthKZT); err != nil {
		RespondError(w, http.StatusInternalServerError, "failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *BannerHandler) Extend(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	old, err := h.repo.GetCampaign(r.Context(), id)
	if err != nil || old.RecruiterID == nil || *old.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var req struct {
		EndsAt string `json:"ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	ends, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil || !ends.After(old.EndsAt) {
		http.Error(w, `{"error":"invalid ends_at"}`, http.StatusBadRequest)
		return
	}
	p, _ := h.repo.GetPlacement(r.Context(), old.PlacementCode)
	amount := repository.CalcBannerPrice(p, old.EndsAt, ends)
	rid := claims.UserID
	c := &model.BannerCampaign{
		PlacementCode: old.PlacementCode, RecruiterID: &rid, CreatedBy: claims.UserID,
		ImageKey: old.ImageKey, LinkURL: old.LinkURL, StartsAt: old.EndsAt, EndsAt: ends,
		Status: model.BannerStatusDraft, AmountKZT: amount,
	}
	if err := h.repo.CreateCampaign(r.Context(), c); err != nil {
		RespondError(w, http.StatusInternalServerError, "extend failed", err)
		return
	}
	img := h.presignImage(r.Context(), c.ImageKey)
	jsonOK(w, h.toCampaignResp(r.Context(), c, img))
}

func (h *BannerHandler) PublicRules(w http.ResponseWriter, r *http.Request) {
	if h.staffRepo == nil {
		jsonOK(w, map[string]interface{}{"rules_text": ""})
		return
	}
	val, err := h.staffRepo.GetSetting(r.Context(), "banner_rules")
	if err != nil {
		jsonOK(w, map[string]interface{}{"rules_text": ""})
		return
	}
	jsonOK(w, val)
}
