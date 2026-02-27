package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

func (r *UserRepository) CreateStudentProfile(ctx context.Context, userID uuid.UUID, fullNameEnc, phoneEnc, bioEnc []byte) (*model.StudentProfile, error) {
	var p model.StudentProfile
	err := r.pool.QueryRow(ctx, `
		INSERT INTO student_profiles (user_id, full_name_enc, phone_enc, bio_enc)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, full_name_enc, phone_enc, bio_enc, resume_object_key, created_at, updated_at
	`, userID, fullNameEnc, phoneEnc, bioEnc).Scan(
		&p.ID, &p.UserID, &p.FullNameEnc, &p.PhoneEnc, &p.BioEnc, &p.ResumeObjectKey, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *UserRepository) GetStudentProfileByUserID(ctx context.Context, userID uuid.UUID) (*model.StudentProfile, error) {
	var p model.StudentProfile
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name_enc, phone_enc, bio_enc, resume_object_key, created_at, updated_at
		FROM student_profiles WHERE user_id = $1
	`, userID).Scan(&p.ID, &p.UserID, &p.FullNameEnc, &p.PhoneEnc, &p.BioEnc, &p.ResumeObjectKey, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *UserRepository) UpdateStudentProfile(ctx context.Context, userID uuid.UUID, fullNameEnc, phoneEnc, bioEnc []byte, resumeKey *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE student_profiles
		SET full_name_enc = COALESCE($2, full_name_enc),
		    phone_enc = COALESCE($3, phone_enc),
		    bio_enc = COALESCE($4, bio_enc),
		    resume_object_key = COALESCE($5, resume_object_key),
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, fullNameEnc, phoneEnc, bioEnc, resumeKey)
	return err
}

func (r *UserRepository) SetStudentResumeKey(ctx context.Context, userID uuid.UUID, objectKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE student_profiles SET resume_object_key = $2, updated_at = NOW() WHERE user_id = $1
	`, userID, objectKey)
	return err
}
