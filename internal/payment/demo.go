package payment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type DemoProvider struct {
	repo *repository.PaymentRepository
}

func NewDemoProvider(repo *repository.PaymentRepository) *DemoProvider {
	return &DemoProvider{repo: repo}
}

func (p *DemoProvider) Name() string { return "demo" }

func (p *DemoProvider) CreateCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
	externalID := fmt.Sprintf("demo_%s", uuid.New().String())
	sess, err := p.repo.CreateSession(ctx, req.RecruiterID, p.Name(), externalID, req.AmountKZT, req.Purpose, req.Metadata)
	if err != nil {
		return nil, err
	}
	return &CheckoutResult{
		SessionID:  sess.ID,
		ExternalID: externalID,
		PaymentURL: fmt.Sprintf("/demo-pay?session=%s", sess.ID.String()),
		Status:     "pending",
	}, nil
}

func (p *DemoProvider) ConfirmPayment(ctx context.Context, externalID string) (uuid.UUID, bool, error) {
	sess, err := p.repo.GetSessionByExternalID(ctx, externalID)
	if err != nil {
		return uuid.Nil, false, err
	}
	if sess.Status == "completed" {
		return sess.ID, true, nil
	}
	if err := p.repo.CompleteSession(ctx, sess.ID); err != nil {
		return uuid.Nil, false, err
	}
	return sess.ID, true, nil
}
