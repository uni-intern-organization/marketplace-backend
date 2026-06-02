package notifier

import (
	"context"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/email"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type Service struct {
	notifRepo   *repository.NotificationRepository
	billingRepo *repository.BillingRepository
	emailSvc    *email.Service
	userRepo    *repository.UserRepository
}

func NewService(
	notifRepo *repository.NotificationRepository,
	billingRepo *repository.BillingRepository,
	emailSvc *email.Service,
	userRepo *repository.UserRepository,
) *Service {
	return &Service{notifRepo: notifRepo, billingRepo: billingRepo, emailSvc: emailSvc, userRepo: userRepo}
}

func (s *Service) Notify(ctx context.Context, userID uuid.UUID, nType, title, body string, payload map[string]interface{}) {
	if s == nil || s.notifRepo == nil {
		return
	}
	prefs, _ := s.notifRepo.GetPreferences(ctx, userID)
	inApp := true
	emailOn := true
	for _, p := range prefs {
		if p.NotificationType == nType || p.NotificationType == "*" {
			inApp = p.ChannelInApp
			emailOn = p.ChannelEmail
			break
		}
	}
	if inApp {
		_, _ = s.notifRepo.Create(ctx, userID, nType, title, body, payload)
	}
	if s.billingRepo != nil {
		_ = s.billingRepo.LogNotification(ctx, userID, "email", title, body)
	}
	if emailOn && s.emailSvc != nil && s.userRepo != nil {
		user, err := s.userRepo.GetByID(ctx, userID)
		if err == nil && user.Email != "" {
			_ = s.emailSvc.SendNotification(user.Email, title, body)
		}
	}
}
