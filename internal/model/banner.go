package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	BannerStatusDraft          = "draft"
	BannerStatusPendingPayment = "pending_payment"
	BannerStatusPendingReview  = "pending_review"
	BannerStatusActive         = "active"
	BannerStatusCompleted      = "completed"
	BannerStatusRejected       = "rejected"
	BannerStatusCancelled      = "cancelled"
)

type BannerPlacement struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	PriceWeekKZT  int    `json:"price_week_kzt"`
	PriceMonthKZT int    `json:"price_month_kzt"`
	IsActive      bool   `json:"is_active"`
}

type BannerCampaign struct {
	ID               uuid.UUID  `json:"id"`
	PlacementCode    string     `json:"placement_code"`
	RecruiterID      *uuid.UUID `json:"recruiter_id,omitempty"`
	CreatedBy        uuid.UUID  `json:"created_by"`
	ImageKey         string     `json:"image_key"`
	LinkURL          string     `json:"link_url"`
	StartsAt         time.Time  `json:"starts_at"`
	EndsAt           time.Time  `json:"ends_at"`
	Status           string     `json:"status"`
	PaymentSessionID *uuid.UUID `json:"payment_session_id,omitempty"`
	AmountKZT        int        `json:"amount_kzt"`
	RejectReason     *string    `json:"reject_reason,omitempty"`
	Impressions      int64      `json:"impressions"`
	Clicks           int64      `json:"clicks"`
	Priority         int        `json:"priority"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
