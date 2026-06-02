package billing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

const FreeVacancyLimit = 1
const MinWithdrawalKZT = 5000

type Entitlements struct {
	Plan                   model.RecruiterPlan
	PlanExpiresAt          *time.Time
	VacancyCount           int
	MaxVacancies           int
	IsPro                  bool
	Features               []string
	CanSearch              bool
	CanInvite              bool
	CanMatch               bool
	CanAnalytics           bool
	CanCreateVacancy       bool
	PublicationsQuota      int
	PublicationsUsed       int
	CanUsePublicationQuota bool
	InvitationsQuota       int
	InvitationsUsed        int
	PremiumCreditCost      int
}

type Service struct {
	recruiterRepo *repository.RecruiterProfileRepository
	vacancyRepo   *repository.VacancyRepository
	modRepo       *repository.ModerationRepository
}

func NewService(recruiterRepo *repository.RecruiterProfileRepository, vacancyRepo *repository.VacancyRepository) *Service {
	return &Service{recruiterRepo: recruiterRepo, vacancyRepo: vacancyRepo}
}

func (s *Service) WithModeration(modRepo *repository.ModerationRepository) *Service {
	s.modRepo = modRepo
	return s
}

func IsFeaturedActive(v *model.Vacancy, now time.Time) bool {
	if v == nil {
		return false
	}
	if v.ListingTier == model.ListingTierPremium {
		if v.Status == model.VacancyStatusActive {
			if v.ExpiresAt == nil || v.ExpiresAt.After(now) {
				return true
			}
		}
	}
	if !v.IsFeatured {
		return false
	}
	if v.FeaturedUntil == nil {
		return true
	}
	return v.FeaturedUntil.After(now)
}

func planFeatures(plan model.RecruiterPlan) []string {
	switch plan {
	case model.RecruiterPlanStarter:
		return []string{"3_standard_posts", "basic_search", "email_notifications"}
	case model.RecruiterPlanBusiness:
		return append(ProFeatureLabels(), "advanced_search", "50_invitations", "full_analytics")
	case model.RecruiterPlanCorporate:
		return append(ProFeatureLabels(), "unlimited_posts", "unlimited_invitations", "dedicated_manager")
	case model.RecruiterPlanPro:
		return ProFeatureLabels()
	default:
		return []string{"view_applications", "post_one_vacancy"}
	}
}

func isPaidPlan(plan model.RecruiterPlan, expires *time.Time) bool {
	if plan == model.RecruiterPlanFree || plan == "" {
		return false
	}
	if expires == nil {
		return plan == model.RecruiterPlanCorporate
	}
	return expires.After(time.Now())
}

func (s *Service) GetRecruiterEntitlements(ctx context.Context, userID uuid.UUID, role model.UserRole) (Entitlements, error) {
	if role == model.RoleAdmin {
		return Entitlements{
			Plan: model.RecruiterPlanCorporate, MaxVacancies: -1, IsPro: true,
			Features: planFeatures(model.RecruiterPlanCorporate),
			CanSearch: true, CanInvite: true, CanMatch: true, CanAnalytics: true,
			CanCreateVacancy: true, PublicationsQuota: -1, CanUsePublicationQuota: true,
			InvitationsQuota: -1, PremiumCreditCost: 3,
		}, nil
	}

	plan := model.RecruiterPlanFree
	var expires *time.Time
	quota, used, invQuota, invUsed := 0, 0, 0, 0
	if p, err := s.recruiterRepo.GetByUserID(ctx, userID); err == nil {
		plan = p.Plan
		if plan == "" {
			plan = model.RecruiterPlanFree
		}
		expires = p.PlanExpiresAt
		quota = p.PublicationsQuota
		used = p.PublicationsUsed
		invQuota = p.InvitationsQuota
		invUsed = p.InvitationsUsed
		_ = s.recruiterRepo.ResetQuotaIfDue(ctx, userID)
	}

	paid := isPaidPlan(plan, expires)
	count, err := s.vacancyRepo.CountActiveByRecruiter(ctx, userID)
	if err != nil {
		return Entitlements{}, err
	}

	ent := Entitlements{
		Plan: plan, PlanExpiresAt: expires, VacancyCount: count,
		IsPro: paid, PublicationsQuota: quota, PublicationsUsed: used,
		InvitationsQuota: invQuota, InvitationsUsed: invUsed, PremiumCreditCost: 3,
		Features: planFeatures(plan),
	}

	switch plan {
	case model.RecruiterPlanStarter:
		ent.CanSearch = paid
		ent.CanCreateVacancy = paid || count < FreeVacancyLimit
		ent.CanUsePublicationQuota = paid && (quota < 0 || used < quota)
	case model.RecruiterPlanBusiness, model.RecruiterPlanCorporate, model.RecruiterPlanPro:
		ent.MaxVacancies = -1
		ent.CanSearch = paid
		ent.CanInvite = paid
		ent.CanMatch = paid
		ent.CanAnalytics = paid
		ent.CanCreateVacancy = paid
		ent.CanUsePublicationQuota = paid && (quota < 0 || used < quota)
	default:
		ent.MaxVacancies = FreeVacancyLimit
		ent.CanCreateVacancy = count < FreeVacancyLimit
	}
	return ent, nil
}

func (s *Service) CanPublishTier(ctx context.Context, userID uuid.UUID, role model.UserRole, tier model.ListingTier) (bool, string) {
	ent, err := s.GetRecruiterEntitlements(ctx, userID, role)
	if err != nil {
		return false, "internal error"
	}
	if s.modRepo != nil {
		v, verr := s.modRepo.GetVerification(ctx, userID)
		if verr != nil || v == nil || v.Status != "approved" {
			if role != model.RoleAdmin {
				return false, "verification_required"
			}
		}
	}
	credits := 1
	if tier == model.ListingTierPremium {
		credits = ent.PremiumCreditCost
	}
	if tier == model.ListingTierBasic {
		if ent.CanCreateVacancy || ent.MaxVacancies < 0 {
			return true, ""
		}
		return false, "plan_limit_reached"
	}
	if ent.CanUsePublicationQuota && ent.PublicationsQuota >= 0 {
		if ent.PublicationsUsed+credits <= ent.PublicationsQuota {
			return true, ""
		}
	}
	if ent.IsPro || role == model.RoleAdmin {
		return true, ""
	}
	if tier == model.ListingTierPremium || tier == model.ListingTierStandard {
		return true, ""
	}
	return false, "subscription_required"
}

func PublicationCreditsForTier(tier model.ListingTier) int {
	if tier == model.ListingTierPremium {
		return 3
	}
	return 1
}

func ProFeatureLabels() []string {
	return []string{
		"unlimited_vacancies", "student_search", "invitations", "matching", "analytics", "publication_quota",
	}
}

func (e Entitlements) RequiresPro() bool {
	return !e.IsPro
}

func HackathonListingFee(prizePool float64, base int) float64 {
	fee := float64(base)
	if prizePool > 500000 {
		fee += (prizePool - 500000) * 0.05
	}
	return fee
}
