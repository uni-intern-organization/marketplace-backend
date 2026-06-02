package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type RecruiterProfileRepository struct {
	pool *pgxpool.Pool
}

func NewRecruiterProfileRepository(pool *pgxpool.Pool) *RecruiterProfileRepository {
	return &RecruiterProfileRepository{pool: pool}
}

const recruiterProfileSelect = `
	SELECT id, user_id, company_name_enc, full_name_enc, phone_enc, company_logo_object_key,
	       COALESCE(plan, 'free'), plan_expires_at,
	       COALESCE(organizer_type::text, 'company'),
	       COALESCE(publications_quota, 0), COALESCE(publications_used, 0), quota_reset_at,
	       COALESCE(invitations_quota, 0), COALESCE(invitations_used, 0),
	       created_at, updated_at
	FROM recruiter_profiles WHERE user_id = $1
`

func scanRecruiterProfile(row interface {
	Scan(dest ...any) error
}) (*model.RecruiterProfile, error) {
	var p model.RecruiterProfile
	var plan, organizer string
	err := row.Scan(
		&p.ID, &p.UserID, &p.CompanyNameEnc, &p.FullNameEnc, &p.PhoneEnc, &p.CompanyLogoObjectKey,
		&plan, &p.PlanExpiresAt, &organizer,
		&p.PublicationsQuota, &p.PublicationsUsed, &p.QuotaResetAt,
		&p.InvitationsQuota, &p.InvitationsUsed,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.Plan = model.RecruiterPlan(plan)
	p.OrganizerType = model.OrganizerType(organizer)
	return &p, nil
}

func (r *RecruiterProfileRepository) Create(ctx context.Context, userID uuid.UUID, companyNameEnc, fullNameEnc, phoneEnc []byte) (*model.RecruiterProfile, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO recruiter_profiles (user_id, company_name_enc, full_name_enc, phone_enc)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, company_name_enc, full_name_enc, phone_enc, company_logo_object_key,
		          COALESCE(plan, 'free'), plan_expires_at,
		          COALESCE(organizer_type::text, 'company'),
		          COALESCE(publications_quota, 0), COALESCE(publications_used, 0), quota_reset_at,
		          COALESCE(invitations_quota, 0), COALESCE(invitations_used, 0),
		          created_at, updated_at
	`, userID, companyNameEnc, fullNameEnc, phoneEnc)
	return scanRecruiterProfile(row)
}

func (r *RecruiterProfileRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.RecruiterProfile, error) {
	row := r.pool.QueryRow(ctx, recruiterProfileSelect, userID)
	return scanRecruiterProfile(row)
}

func (r *RecruiterProfileRepository) Update(ctx context.Context, userID uuid.UUID, companyNameEnc, fullNameEnc, phoneEnc []byte) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE recruiter_profiles
		SET company_name_enc = COALESCE($2, company_name_enc),
		    full_name_enc = COALESCE($3, full_name_enc),
		    phone_enc = COALESCE($4, phone_enc),
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, companyNameEnc, fullNameEnc, phoneEnc)
	return err
}

func (r *RecruiterProfileRepository) UpdateLogo(ctx context.Context, userID uuid.UUID, objectKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE recruiter_profiles SET company_logo_object_key = $2, updated_at = NOW() WHERE user_id = $1
	`, userID, objectKey)
	return err
}

func (r *RecruiterProfileRepository) SetPublicationQuota(ctx context.Context, userID uuid.UUID, quota int, resetAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE recruiter_profiles
		SET publications_quota = $2, publications_used = 0, quota_reset_at = $3, updated_at = NOW()
		WHERE user_id = $1
	`, userID, quota, resetAt)
	return err
}

func (r *RecruiterProfileRepository) IncrementPublicationsUsed(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE recruiter_profiles
		SET publications_used = publications_used + 1, updated_at = NOW()
		WHERE user_id = $1
	`, userID)
	return err
}

func (r *RecruiterProfileRepository) ResetQuotaIfDue(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE recruiter_profiles
		SET publications_used = 0, quota_reset_at = quota_reset_at + INTERVAL '30 days', updated_at = NOW()
		WHERE user_id = $1 AND quota_reset_at IS NOT NULL AND quota_reset_at < NOW()
	`, userID)
	return err
}
