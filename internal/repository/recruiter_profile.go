package repository

import (
	"context"

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

func (r *RecruiterProfileRepository) Create(ctx context.Context, userID uuid.UUID, companyNameEnc, fullNameEnc, phoneEnc []byte) (*model.RecruiterProfile, error) {
	var p model.RecruiterProfile
	err := r.pool.QueryRow(ctx, `
		INSERT INTO recruiter_profiles (user_id, company_name_enc, full_name_enc, phone_enc)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, company_name_enc, full_name_enc, phone_enc, company_logo_object_key, created_at, updated_at
	`, userID, companyNameEnc, fullNameEnc, phoneEnc).Scan(
		&p.ID, &p.UserID, &p.CompanyNameEnc, &p.FullNameEnc, &p.PhoneEnc, &p.CompanyLogoObjectKey, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *RecruiterProfileRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.RecruiterProfile, error) {
	var p model.RecruiterProfile
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, company_name_enc, full_name_enc, phone_enc, company_logo_object_key, created_at, updated_at
		FROM recruiter_profiles WHERE user_id = $1
	`, userID).Scan(&p.ID, &p.UserID, &p.CompanyNameEnc, &p.FullNameEnc, &p.PhoneEnc, &p.CompanyLogoObjectKey, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
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
