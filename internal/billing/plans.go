package billing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

const SubscriptionPeriodDays = 30

// PlanActivation holds resolved subscription activation parameters.
type PlanActivation struct {
	PlanID    string
	Plan      model.RecruiterPlan
	EventType string
	PubQuota  int
	InvQuota  int
	PriceKZT  int
	Name      string
}

// PlanPricing carries plan prices from config.
type PlanPricing struct {
	StarterKZT   int
	BusinessKZT  int
	CorporateKZT int
}

// ResolvePlan maps a plan id to activation parameters and price.
func ResolvePlan(planID string, cfg PlanPricing) (PlanActivation, bool) {
	switch planID {
	case "starter":
		return PlanActivation{
			PlanID: "starter", Plan: model.RecruiterPlanStarter, EventType: "subscribe_starter",
			PubQuota: 3, InvQuota: 0, PriceKZT: cfg.StarterKZT, Name: "Стартовая",
		}, true
	case "business":
		return PlanActivation{
			PlanID: "business", Plan: model.RecruiterPlanBusiness, EventType: "subscribe_business",
			PubQuota: 10, InvQuota: 50, PriceKZT: cfg.BusinessKZT, Name: "Бизнес",
		}, true
	case "corporate":
		return PlanActivation{
			PlanID: "corporate", Plan: model.RecruiterPlanCorporate, EventType: "subscribe_corporate",
			PubQuota: -1, InvQuota: -1, PriceKZT: cfg.CorporateKZT, Name: "Корпоративная",
		}, true
	case "pro":
		return PlanActivation{
			PlanID: "pro", Plan: model.RecruiterPlanPro, EventType: "subscribe_pro",
			PubQuota: 0, InvQuota: 0, PriceKZT: 15000, Name: "Pro",
		}, true
	case "pro_monthly_5":
		return PlanActivation{
			PlanID: "pro_monthly_5", Plan: model.RecruiterPlanPro, EventType: "subscribe_pro_monthly_5",
			PubQuota: 5, InvQuota: 0, PriceKZT: 15000, Name: "Pro",
		}, true
	default:
		return PlanActivation{}, false
	}
}

// ActivateRecruiterPlan sets recruiter profile plan and records a billing event.
func ActivateRecruiterPlan(
	ctx context.Context,
	billingRepo *repository.BillingRepository,
	recruiterID uuid.UUID,
	act PlanActivation,
	expires time.Time,
	eventMeta map[string]interface{},
) error {
	if err := billingRepo.SetRecruiterPlan(ctx, recruiterID, act.Plan, expires, act.PubQuota, act.InvQuota); err != nil {
		return err
	}
	return billingRepo.InsertEvent(ctx, recruiterID, act.EventType, eventMeta)
}
