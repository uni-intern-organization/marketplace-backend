package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

var ErrSessionNotOwned = errors.New("session not owned by recruiter")
var ErrSessionNotPending = errors.New("session is not pending")
var ErrUnsupportedPurpose = errors.New("unsupported payment purpose")

// FulfillmentResult is returned after a successful subscription fulfillment.
type FulfillmentResult struct {
	Plan             string    `json:"plan"`
	PlanExpiresAt    time.Time `json:"plan_expires_at"`
	SessionID        uuid.UUID `json:"session_id"`
	AlreadyFulfilled bool      `json:"already_fulfilled"`
	AmountKZT        int       `json:"amount_kzt"`
	PlanName         string    `json:"plan_name"`
}

// FulfillmentService activates subscriptions after payment sessions complete.
type FulfillmentService struct {
	billingRepo *repository.BillingRepository
	paymentRepo *repository.PaymentRepository
	notifier    *notifier.Service
	pricing     billing.PlanPricing
}

func NewFulfillmentService(
	billingRepo *repository.BillingRepository,
	paymentRepo *repository.PaymentRepository,
	notifierSvc *notifier.Service,
	pricing billing.PlanPricing,
) *FulfillmentService {
	return &FulfillmentService{
		billingRepo: billingRepo,
		paymentRepo: paymentRepo,
		notifier:    notifierSvc,
		pricing:     pricing,
	}
}

func (f *FulfillmentService) FulfillSession(ctx context.Context, sessionID uuid.UUID) (*FulfillmentResult, error) {
	sess, err := f.paymentRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != "completed" {
		return nil, fmt.Errorf("session not completed")
	}
	return f.fulfillCompleted(ctx, sess)
}

func (f *FulfillmentService) CompleteAndFulfill(ctx context.Context, sessionID, recruiterID uuid.UUID) (*FulfillmentResult, error) {
	sess, err := f.paymentRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.RecruiterID != recruiterID {
		return nil, ErrSessionNotOwned
	}
	if sess.Status == "completed" {
		return f.fulfillCompleted(ctx, sess)
	}
	if sess.Status == "failed" {
		return nil, fmt.Errorf("payment failed")
	}
	if sess.Status != "pending" {
		return nil, ErrSessionNotPending
	}
	if err := f.paymentRepo.CompleteSession(ctx, sess.ID); err != nil {
		return nil, err
	}
	sess.Status = "completed"
	now := time.Now()
	sess.CompletedAt = &now
	return f.fulfillCompleted(ctx, sess)
}

func (f *FulfillmentService) FailSession(ctx context.Context, sessionID, recruiterID uuid.UUID) error {
	sess, err := f.paymentRepo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if sess.RecruiterID != recruiterID {
		return ErrSessionNotOwned
	}
	if sess.Status != "pending" {
		return ErrSessionNotPending
	}
	return f.paymentRepo.FailSession(ctx, sess.ID)
}

func (f *FulfillmentService) fulfillCompleted(ctx context.Context, sess *repository.PaymentSession) (*FulfillmentResult, error) {
	meta := parseMetadata(sess.Metadata)
	if fulfilled, ok := meta["fulfilled"].(bool); ok && fulfilled {
		planID, _ := meta["plan"].(string)
		expStr, _ := meta["plan_expires_at"].(string)
		exp, _ := time.Parse(time.RFC3339, expStr)
		name, _ := meta["plan_name"].(string)
		return &FulfillmentResult{
			Plan: planID, PlanExpiresAt: exp, SessionID: sess.ID,
			AlreadyFulfilled: true, AmountKZT: sess.AmountKZT, PlanName: name,
		}, nil
	}

	if sess.Purpose != "subscription" {
		return nil, ErrUnsupportedPurpose
	}

	planID, _ := meta["plan"].(string)
	if planID == "" {
		return nil, fmt.Errorf("missing plan in session metadata")
	}
	act, ok := billing.ResolvePlan(planID, f.pricing)
	if !ok {
		return nil, fmt.Errorf("unknown plan: %s", planID)
	}

	expires := time.Now().Add(billing.SubscriptionPeriodDays * 24 * time.Hour)
	paymentMethod, _ := meta["payment_method"].(string)

	eventMeta := map[string]interface{}{
		"plan":               planID,
		"expires_at":         expires.Format(time.RFC3339),
		"publications_quota": act.PubQuota,
		"demo":               false,
		"session_id":         sess.ID.String(),
		"amount_kzt":         sess.AmountKZT,
		"payment_method":     paymentMethod,
		"provider":           sess.Provider,
	}

	if err := billing.ActivateRecruiterPlan(ctx, f.billingRepo, sess.RecruiterID, act, expires, eventMeta); err != nil {
		return nil, err
	}

	meta["fulfilled"] = true
	meta["plan_expires_at"] = expires.Format(time.RFC3339)
	meta["plan_name"] = act.Name
	_ = f.paymentRepo.UpdateSessionMetadata(ctx, sess.ID, meta)

	if f.notifier != nil {
		title := fmt.Sprintf("Подписка «%s» активирована", act.Name)
		body := fmt.Sprintf(
			"Оплата %d ₸ прошла успешно. Подписка активна до %s. Номер транзакции: %s.",
			sess.AmountKZT, expires.Format("02.01.2006"), sess.ID.String(),
		)
		f.notifier.Notify(ctx, sess.RecruiterID, "subscription_activated", title, body, map[string]interface{}{
			"plan": planID, "session_id": sess.ID.String(), "amount_kzt": sess.AmountKZT,
		})
	}

	return &FulfillmentResult{
		Plan: planID, PlanExpiresAt: expires, SessionID: sess.ID,
		AmountKZT: sess.AmountKZT, PlanName: act.Name,
	}, nil
}

func parseMetadata(raw []byte) map[string]interface{} {
	out := map[string]interface{}{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

// FulfillByExternalID completes webhook flow: confirm payment then fulfill.
func (f *FulfillmentService) FulfillByExternalID(ctx context.Context, provider PaymentProvider, externalID string) (*FulfillmentResult, error) {
	sessionID, ok, err := provider.ConfirmPayment(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, pgx.ErrNoRows
	}
	return f.FulfillSession(ctx, sessionID)
}
