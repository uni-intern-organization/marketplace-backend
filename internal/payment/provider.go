package payment

import (
	"context"

	"github.com/google/uuid"
)

type CheckoutRequest struct {
	RecruiterID uuid.UUID
	AmountKZT   int
	Purpose     string
	Metadata    map[string]interface{}
}

type CheckoutResult struct {
	SessionID  uuid.UUID
	ExternalID string
	PaymentURL string
	Status     string
}

type PaymentProvider interface {
	Name() string
	CreateCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error)
	ConfirmPayment(ctx context.Context, externalID string) (sessionID uuid.UUID, ok bool, err error)
}
