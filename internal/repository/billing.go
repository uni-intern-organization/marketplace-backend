package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type BillingRepository struct {
	pool *pgxpool.Pool
}

func NewBillingRepository(pool *pgxpool.Pool) *BillingRepository {
	return &BillingRepository{pool: pool}
}

func (r *BillingRepository) SetRecruiterPlan(ctx context.Context, userID uuid.UUID, plan model.RecruiterPlan, expiresAt time.Time, pubQuota, invQuota int) error {
	return r.SetRecruiterPlanWithMeta(ctx, userID, plan, time.Now(), expiresAt, pubQuota, invQuota, "self_serve")
}

func (r *BillingRepository) SetRecruiterPlanWithMeta(
	ctx context.Context,
	userID uuid.UUID,
	plan model.RecruiterPlan,
	startedAt, expiresAt time.Time,
	pubQuota, invQuota int,
	activationSource string,
) error {
	if activationSource == "" {
		activationSource = "self_serve"
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE recruiter_profiles
		SET plan = $2, plan_started_at = $3, plan_expires_at = $4, publications_quota = $5, publications_used = 0,
		    invitations_quota = $6, invitations_used = 0, activation_source = $7,
		    quota_reset_at = $4, updated_at = NOW()
		WHERE user_id = $1
	`, userID, plan, startedAt, expiresAt, pubQuota, invQuota, activationSource)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO recruiter_profiles (user_id, plan, plan_started_at, plan_expires_at, publications_quota, invitations_quota, activation_source, quota_reset_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $4)
	`, userID, plan, startedAt, expiresAt, pubQuota, invQuota, activationSource)
	return err
}

func (r *BillingRepository) PromoteVacancy(ctx context.Context, vacancyID, recruiterID uuid.UUID, until time.Time) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE vacancies
		SET is_featured = true, featured_until = $3, listing_tier = 'premium', updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2
	`, vacancyID, recruiterID, until)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *BillingRepository) InsertEvent(ctx context.Context, recruiterID uuid.UUID, eventType string, metadata map[string]interface{}) error {
	var meta []byte
	if metadata != nil {
		var err error
		meta, err = json.Marshal(metadata)
		if err != nil {
			return err
		}
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO billing_events (recruiter_id, event_type, metadata)
		VALUES ($1, $2, $3)
	`, recruiterID, eventType, meta)
	return err
}

func (r *BillingRepository) LogNotification(ctx context.Context, userID uuid.UUID, channel, subject, body string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_log (user_id, channel, subject, body) VALUES ($1, $2, $3, $4)
	`, userID, channel, subject, body)
	return err
}

func (r *BillingRepository) EnsureWallet(ctx context.Context, userID uuid.UUID, initialBalance float64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO wallets (user_id, balance_kzt) VALUES ($1, $2)
		ON CONFLICT (user_id) DO NOTHING
	`, userID, initialBalance)
	return err
}

func (r *BillingRepository) CreateEscrow(ctx context.Context, recruiterID uuid.UUID, refType string, refID uuid.UUID, amount float64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO escrow_holds (recruiter_id, reference_type, reference_id, amount_kzt, status)
		VALUES ($1, $2, $3, $4, 'held')
	`, recruiterID, refType, refID, amount)
	return err
}

func (r *BillingRepository) ReleaseEscrow(ctx context.Context, refType string, refID uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE escrow_holds SET status = 'released', released_at = NOW()
		WHERE reference_type = $1 AND reference_id = $2 AND status = 'held'
	`, refType, refID)
	return tag.RowsAffected() > 0, err
}

func (r *BillingRepository) ReleaseEscrowAndCredit(ctx context.Context, refType string, refID, userID uuid.UUID, amount float64) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		UPDATE escrow_holds SET status = 'released', released_at = NOW()
		WHERE reference_type = $1 AND reference_id = $2 AND status = 'held'
	`, refType, refID)
	if err != nil || tag.RowsAffected() == 0 {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO wallets (user_id, balance_kzt) VALUES ($1, 0)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE wallets SET balance_kzt = balance_kzt + $2, updated_at = NOW() WHERE user_id = $1
	`, userID, amount); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (r *BillingRepository) CreditWallet(ctx context.Context, userID uuid.UUID, amount float64) error {
	_ = r.EnsureWallet(ctx, userID, 0)
	_, err := r.pool.Exec(ctx, `
		UPDATE wallets SET balance_kzt = balance_kzt + $2, updated_at = NOW() WHERE user_id = $1
	`, userID, amount)
	return err
}

type RecruiterAnalytics struct {
	VacancyCount         int            `json:"vacancy_count"`
	VacancyViews         int            `json:"vacancy_views"`
	ApplicationsByStatus map[string]int `json:"applications_by_status"`
	InvitationsSent      int            `json:"invitations_sent"`
	InvitationsAccepted  int            `json:"invitations_accepted"`
	FreelanceTasksOpen   int            `json:"freelance_tasks_open"`
	FreelanceCompleted   int            `json:"freelance_completed"`
	HackathonsActive     int            `json:"hackathons_active"`
}

func (r *BillingRepository) GetAnalytics(ctx context.Context, recruiterID uuid.UUID) (*RecruiterAnalytics, error) {
	var a RecruiterAnalytics
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM vacancies WHERE recruiter_id = $1 AND status = 'active'
	`, recruiterID).Scan(&a.VacancyCount)
	if err != nil {
		return nil, err
	}
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM vacancy_views vv
		JOIN vacancies v ON v.id = vv.vacancy_id WHERE v.recruiter_id = $1
	`, recruiterID).Scan(&a.VacancyViews)

	rows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM applications WHERE recruiter_id = $1 GROUP BY status
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	a.ApplicationsByStatus = make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		a.ApplicationsByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'accepted')
		FROM invitations WHERE recruiter_id = $1
	`, recruiterID).Scan(&a.InvitationsSent, &a.InvitationsAccepted)
	if err != nil {
		return nil, err
	}

	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM freelance_tasks WHERE recruiter_id = $1 AND status = 'open'
	`, recruiterID).Scan(&a.FreelanceTasksOpen)
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM freelance_tasks WHERE recruiter_id = $1 AND status = 'completed'
	`, recruiterID).Scan(&a.FreelanceCompleted)
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM hackathons WHERE organizer_id = $1 AND status IN ('published', 'active')
	`, recruiterID).Scan(&a.HackathonsActive)

	return &a, nil
}
