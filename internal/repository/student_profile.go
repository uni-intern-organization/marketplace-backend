package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

func (r *UserRepository) CreateStudentProfile(ctx context.Context, userID uuid.UUID, fullNameEnc, phoneEnc, bioEnc []byte, skills, education, location, availability string, experienceYears int) (*model.StudentProfile, error) {
	var p model.StudentProfile
	err := r.pool.QueryRow(ctx, `
		INSERT INTO student_profiles (user_id, full_name_enc, phone_enc, bio_enc, skills, education, experience_years, location, availability)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, user_id, full_name_enc, phone_enc, bio_enc, resume_object_key, skills, education, experience_years, location, availability, created_at, updated_at
	`, userID, fullNameEnc, phoneEnc, bioEnc, skills, education, experienceYears, location, availability).Scan(
		&p.ID, &p.UserID, &p.FullNameEnc, &p.PhoneEnc, &p.BioEnc, &p.ResumeObjectKey, &p.Skills, &p.Education, &p.ExperienceYears, &p.Location, &p.Availability, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *UserRepository) GetStudentProfileByUserID(ctx context.Context, userID uuid.UUID) (*model.StudentProfile, error) {
	var p model.StudentProfile
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name_enc, phone_enc, bio_enc, resume_object_key, COALESCE(skills,''), COALESCE(education,''), COALESCE(experience_years,0), COALESCE(location,''), COALESCE(availability,''), created_at, updated_at
		FROM student_profiles WHERE user_id = $1
	`, userID).Scan(&p.ID, &p.UserID, &p.FullNameEnc, &p.PhoneEnc, &p.BioEnc, &p.ResumeObjectKey, &p.Skills, &p.Education, &p.ExperienceYears, &p.Location, &p.Availability, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *UserRepository) UpdateStudentProfile(ctx context.Context, userID uuid.UUID, fullNameEnc, phoneEnc, bioEnc []byte, resumeKey *string, skills, education, location, availability *string, experienceYears *int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE student_profiles
		SET full_name_enc = COALESCE($2, full_name_enc),
		    phone_enc = COALESCE($3, phone_enc),
		    bio_enc = COALESCE($4, bio_enc),
		    resume_object_key = COALESCE($5, resume_object_key),
		    skills = COALESCE($6, skills),
		    education = COALESCE($7, education),
		    experience_years = COALESCE($8, experience_years),
		    location = COALESCE($9, location),
		    availability = COALESCE($10, availability),
		    updated_at = NOW()
		WHERE user_id = $1
	`, userID, fullNameEnc, phoneEnc, bioEnc, resumeKey, skills, education, experienceYears, location, availability)
	return err
}

func (r *UserRepository) SetStudentResumeKey(ctx context.Context, userID uuid.UUID, objectKey string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE student_profiles SET resume_object_key = $2, updated_at = NOW() WHERE user_id = $1
	`, userID, objectKey)
	return err
}

// StudentProfileForMatching is used by the matching engine (no encrypted fields).
type StudentProfileForMatching struct {
	UserID          uuid.UUID
	Skills          string
	Education       string
	ExperienceYears int
	Location        string
	Availability    string
}

func (r *UserRepository) ListStudentProfilesForMatching(ctx context.Context) ([]StudentProfileForMatching, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, COALESCE(skills,''), COALESCE(education,''), COALESCE(experience_years,0), COALESCE(location,''), COALESCE(availability,'')
		FROM student_profiles
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []StudentProfileForMatching
	for rows.Next() {
		var p StudentProfileForMatching
		if err := rows.Scan(&p.UserID, &p.Skills, &p.Education, &p.ExperienceYears, &p.Location, &p.Availability); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}
