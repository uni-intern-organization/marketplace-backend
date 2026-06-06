package hackathon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

var (
	ErrForbidden     = errors.New("forbidden")
	ErrNotFound      = errors.New("not found")
	ErrInvalidState  = errors.New("invalid state")
	ErrDeadline      = errors.New("submission deadline passed")
	ErrRegistration  = errors.New("registration not allowed")
	ErrResultsLocked = errors.New("results locked")
)

type BillingEvents interface {
	InsertEvent(ctx context.Context, recruiterID uuid.UUID, eventType string, metadata map[string]interface{}) error
	LogNotification(ctx context.Context, userID uuid.UUID, channel, subject, body string) error
}

type Service struct {
	repo    *repository.HackathonRepository
	billing BillingEvents
}

func New(repo *repository.HackathonRepository, billing BillingEvents) *Service {
	return &Service{repo: repo, billing: billing}
}

func ListingFee(prize float64) int {
	switch {
	case prize >= 500000:
		return 50000
	case prize >= 100000:
		return 25000
	default:
		return 10000
	}
}

func PrizeHoldTotal(breakdown json.RawMessage) float64 {
	if len(breakdown) == 0 {
		return 0
	}
	var m struct {
		First  float64 `json:"first"`
		Second float64 `json:"second"`
		Third  float64 `json:"third"`
		Extra  []struct {
			Amount float64 `json:"amount"`
		} `json:"nominations"`
	}
	if err := json.Unmarshal(breakdown, &m); err != nil {
		return 0
	}
	total := m.First + m.Second + m.Third
	for _, n := range m.Extra {
		total += n.Amount
	}
	return total
}

func CatalogPriority(prizeType string, prizePool float64) int {
	if prizeType == model.HackathonPrizeCash && prizePool > 0 {
		return 100
	}
	if prizeType == model.HackathonPrizeNonMonetary {
		return 50
	}
	return 0
}

func (s *Service) ValidateTimeline(h *model.Hackathon) error {
	regOpen := h.RegistrationOpensAt
	if regOpen == nil {
		t := h.CreatedAt
		regOpen = &t
	}
	if regOpen.After(h.RegistrationDeadline) {
		return fmt.Errorf("registration_opens_at must be before registration_deadline")
	}
	if h.RegistrationDeadline.After(h.StartsAt) {
		return fmt.Errorf("registration_deadline must be before starts_at")
	}
	if h.StartsAt.After(h.EndsAt) {
		return fmt.Errorf("starts_at must be before ends_at")
	}
	if h.ResultsAnnouncedAt != nil && h.ResultsAnnouncedAt.Before(h.EndsAt) {
		return fmt.Errorf("results_announced_at must be after ends_at")
	}
	return nil
}

func (s *Service) CanRegister(ctx context.Context, h *model.Hackathon) error {
	if h.Status != "registration_open" && h.Status != "published" {
		return ErrRegistration
	}
	now := time.Now()
	if h.RegistrationOpensAt != nil && now.Before(*h.RegistrationOpensAt) {
		return ErrRegistration
	}
	if now.After(h.RegistrationDeadline) {
		return ErrRegistration
	}
	n, _ := s.repo.ApprovedRegistrationCount(ctx, h.ID)
	if n >= h.MaxParticipants {
		return ErrRegistration
	}
	return nil
}

func (s *Service) RegistrationStatus(h *model.Hackathon) string {
	if h.RegistrationMode == model.HackathonRegManual {
		return model.RegStatusPending
	}
	return model.RegStatusApproved
}

func (s *Service) CanSubmit(h *model.Hackathon) bool {
	now := time.Now()
	if now.Before(h.StartsAt) {
		return false
	}
	return now.Before(h.EndsAt) || now.Equal(h.EndsAt)
}

func (s *Service) TaskVisible(h *model.Hackathon) bool {
	if h.TaskReveal == model.HackathonTaskInDescription {
		return true
	}
	return time.Now().After(h.StartsAt) || time.Now().Equal(h.StartsAt)
}

func (s *Service) ComputeRanking(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	return s.repo.ComputeRanking(ctx, hackathonID)
}

func (s *Service) AutoWinners(ctx context.Context, hackathonID uuid.UUID, limit int) ([]model.HackathonResult, error) {
	ranking, err := s.repo.ComputeRanking(ctx, hackathonID)
	if err != nil {
		return nil, err
	}
	var results []model.HackathonResult
	for i, row := range ranking {
		if i >= limit {
			break
		}
		place := i + 1
		res := model.HackathonResult{HackathonID: hackathonID, Place: place}
		if tid, ok := row["team_id"].(string); ok && tid != "" {
			id, _ := uuid.Parse(tid)
			res.TeamID = &id
		}
		if sid, ok := row["submission_id"].(string); ok && sid != "" {
			sub, err := s.repo.GetSubmission(ctx, uuid.MustParse(sid))
			if err == nil && sub.StudentID != nil {
				res.StudentID = sub.StudentID
			}
		}
		if score, ok := row["total_score"].(float64); ok {
			_ = score
		}
		results = append(results, res)
	}
	return results, nil
}

func (s *Service) NotifyRegistration(ctx context.Context, studentID uuid.UUID, hackathonID uuid.UUID, status string) {
	if s.billing == nil {
		return
	}
	subj := "Hackathon registration"
	body := fmt.Sprintf("Your registration status: %s", status)
	_ = s.billing.LogNotification(ctx, studentID, "email", subj, body)
}

func (s *Service) NotifyHackathonStart(ctx context.Context, hackathonID uuid.UUID) error {
	ids, err := s.repo.ApprovedStudentIDs(ctx, hackathonID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		_ = s.billing.LogNotification(ctx, id, "email", "Hackathon started", "The hackathon task is now available.")
	}
	return nil
}
