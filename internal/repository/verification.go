package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type VerificationRepository struct {
	pool *pgxpool.Pool
}

func NewVerificationRepository(pool *pgxpool.Pool) *VerificationRepository {
	return &VerificationRepository{pool: pool}
}

func (r *VerificationRepository) Upsert(ctx context.Context, recruiterID uuid.UUID, bin string, docKey *string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO recruiter_verifications (recruiter_id, bin, document_key, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (recruiter_id) DO UPDATE SET
			bin = EXCLUDED.bin,
			document_key = COALESCE(EXCLUDED.document_key, recruiter_verifications.document_key),
			status = 'pending', updated_at = NOW()
	`, recruiterID, bin, docKey)
	return err
}

func (r *VerificationRepository) GetByRecruiter(ctx context.Context, recruiterID uuid.UUID) (*model.RecruiterVerification, error) {
	var v model.RecruiterVerification
	err := r.pool.QueryRow(ctx, `
		SELECT id, recruiter_id, bin, document_key, status, reviewed_by, COALESCE(comment,''), created_at, updated_at
		FROM recruiter_verifications WHERE recruiter_id = $1
	`, recruiterID).Scan(&v.ID, &v.RecruiterID, &v.BIN, &v.DocumentKey, &v.Status, &v.ReviewedBy, &v.Comment, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VerificationRepository) List(ctx context.Context, status string, limit int) ([]model.RecruiterVerification, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, recruiter_id, bin, document_key, status, reviewed_by, COALESCE(comment,''), created_at, updated_at
			FROM recruiter_verifications WHERE status = $1 ORDER BY created_at ASC LIMIT $2
		`, status, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, recruiter_id, bin, document_key, status, reviewed_by, COALESCE(comment,''), created_at, updated_at
			FROM recruiter_verifications ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.RecruiterVerification
	for rows.Next() {
		var v model.RecruiterVerification
		if err := rows.Scan(&v.ID, &v.RecruiterID, &v.BIN, &v.DocumentKey, &v.Status, &v.ReviewedBy, &v.Comment, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *VerificationRepository) Review(ctx context.Context, recruiterID, reviewerID uuid.UUID, status, comment string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE recruiter_verifications
		SET status = $3, reviewed_by = $2, comment = $4, updated_at = NOW()
		WHERE recruiter_id = $1
	`, recruiterID, reviewerID, status, comment)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *VacancyRepository) AddFavorite(ctx context.Context, userID, vacancyID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO vacancy_favorites (user_id, vacancy_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, userID, vacancyID)
	return err
}

func (r *VacancyRepository) RemoveFavorite(ctx context.Context, userID, vacancyID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM vacancy_favorites WHERE user_id = $1 AND vacancy_id = $2`, userID, vacancyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *VacancyRepository) ListFavorites(ctx context.Context, userID uuid.UUID) ([]model.Vacancy, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+vacancySelectCols+`
		FROM vacancies v
		JOIN vacancy_favorites f ON f.vacancy_id = v.id
		WHERE f.user_id = $1 ORDER BY f.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Vacancy
	for rows.Next() {
		v, err := scanVacancy(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

type VacancyDraftInput struct {
	TitleEnc             []byte
	DescriptionEnc       []byte
	CompanyName          string
	RequiredSkills       string
	Location             string
	EmploymentType       string
	MinExperienceYears   int
	ListingTier          model.ListingTier
	VacancyType          string
	ResponsibilitiesEnc  []byte
	RequirementsEnc      []byte
	OffersEnc            []byte
	SalaryType           string
	SalaryMin            *int
	SalaryMax            *int
	DurationMonths       *int
	ApplicationDeadline  *time.Time
	ContactNameEnc       []byte
	ContactEmail         string
	Specialty            string
	DesiredStartDate     *time.Time
}

func (r *VacancyRepository) SaveDraft(ctx context.Context, recruiterID uuid.UUID, draft VacancyDraftInput) (*model.Vacancy, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO vacancies (
			recruiter_id, title_enc, description_enc, company_name, required_skills, location,
			employment_type, min_experience_years, listing_tier, status,
			vacancy_type, responsibilities_enc, requirements_enc, offers_enc,
			salary_type, salary_min, salary_max, duration_months,
			application_deadline, contact_name_enc, contact_email, specialty, desired_start_date
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',
			$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22
		) RETURNING `+vacancySelectCols,
		recruiterID, draft.TitleEnc, draft.DescriptionEnc, draft.CompanyName, draft.RequiredSkills, draft.Location,
		draft.EmploymentType, draft.MinExperienceYears, string(draft.ListingTier),
		draft.VacancyType, draft.ResponsibilitiesEnc, draft.RequirementsEnc, draft.OffersEnc,
		draft.SalaryType, draft.SalaryMin, draft.SalaryMax, draft.DurationMonths,
		draft.ApplicationDeadline, draft.ContactNameEnc, draft.ContactEmail, draft.Specialty, draft.DesiredStartDate,
	)
	v, err := scanVacancy(row)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VacancyRepository) ListExpiringSoon(ctx context.Context, days int) ([]uuid.UUID, []uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, recruiter_id FROM vacancies
		WHERE status = 'active' AND expires_at IS NOT NULL
		  AND expires_at > NOW() AND expires_at <= NOW() + ($1::int * interval '1 day')
	`, days)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var vacIDs, recIDs []uuid.UUID
	for rows.Next() {
		var v, rID uuid.UUID
		if err := rows.Scan(&v, &rID); err != nil {
			return nil, nil, err
		}
		vacIDs = append(vacIDs, v)
		recIDs = append(recIDs, rID)
	}
	return vacIDs, recIDs, rows.Err()
}
