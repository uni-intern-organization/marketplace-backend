package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type VacancyRepository struct {
	pool *pgxpool.Pool
}

func NewVacancyRepository(pool *pgxpool.Pool) *VacancyRepository {
	return &VacancyRepository{pool: pool}
}

const vacancySelectCols = `
	id, recruiter_id, title_enc, description_enc, COALESCE(company_name,''),
	COALESCE(required_skills,''), COALESCE(location,''), COALESCE(employment_type,''),
	COALESCE(min_experience_years,0),
	COALESCE(listing_tier::text, 'basic'), published_at, expires_at,
	COALESCE(status::text, 'active'),
	COALESCE(is_featured, false), featured_until,
	created_at, updated_at
`

func scanVacancy(row interface {
	Scan(dest ...any) error
}) (model.Vacancy, error) {
	var v model.Vacancy
	var tier, status string
	err := row.Scan(
		&v.ID, &v.RecruiterID, &v.TitleEnc, &v.DescriptionEnc, &v.CompanyName,
		&v.RequiredSkills, &v.Location, &v.EmploymentType, &v.MinExperienceYears,
		&tier, &v.PublishedAt, &v.ExpiresAt, &status,
		&v.IsFeatured, &v.FeaturedUntil, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return v, err
	}
	v.ListingTier = model.ListingTier(tier)
	v.Status = model.VacancyStatus(status)
	return v, nil
}

func (r *VacancyRepository) Create(ctx context.Context, recruiterID uuid.UUID, titleEnc, descriptionEnc []byte, companyName, requiredSkills, location, employmentType string, minExperienceYears int, tier model.ListingTier) (*model.Vacancy, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO vacancies (
			recruiter_id, title_enc, description_enc, company_name, required_skills, location,
			employment_type, min_experience_years, listing_tier, status, is_featured
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'draft', false)
		RETURNING `+vacancySelectCols,
		recruiterID, titleEnc, descriptionEnc, companyName, requiredSkills, location, employmentType,
		minExperienceYears, string(tier))
	v, err := scanVacancy(row)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VacancyRepository) SubmitForModeration(ctx context.Context, id, recruiterID uuid.UUID, tier model.ListingTier) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE vacancies SET listing_tier = $3, status = 'pending_review', moderation_submitted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2
	`, id, recruiterID, string(tier))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *VacancyRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Vacancy, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+vacancySelectCols+` FROM vacancies WHERE id = $1`, id)
	v, err := scanVacancy(row)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VacancyRepository) CountActiveByRecruiter(ctx context.Context, recruiterID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM vacancies WHERE recruiter_id = $1 AND status = 'active'
	`, recruiterID).Scan(&n)
	return n, err
}

func (r *VacancyRepository) CountByRecruiter(ctx context.Context, recruiterID uuid.UUID) (int, error) {
	return r.CountActiveByRecruiter(ctx, recruiterID)
}

func (r *VacancyRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.Vacancy, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+vacancySelectCols+`
		FROM vacancies WHERE recruiter_id = $1 ORDER BY created_at DESC
	`, recruiterID)
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

type VacancyFilter struct {
	Query              string
	Skills             string
	Location           string
	EmploymentType     string
	MinExperienceYears *int
	Tier               string
	IncludeArchived    bool
	PremiumOnly        bool
	CreatedAfter       *time.Time
	Offset             int
}

func tierOrderSQL() string {
	return `
		CASE listing_tier::text
			WHEN 'premium' THEN 0
			WHEN 'standard' THEN 1
			ELSE 2
		END,
		created_at DESC
	`
}

func (r *VacancyRepository) List(ctx context.Context, filter VacancyFilter, limit int) ([]model.Vacancy, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `SELECT ` + vacancySelectCols + ` FROM vacancies WHERE 1=1`
	args := []interface{}{}
	n := 1
	if !filter.IncludeArchived {
		query += ` AND status = 'active'`
	}
	if filter.PremiumOnly {
		query += ` AND listing_tier = 'premium'`
	}
	if filter.Tier != "" {
		query += fmt.Sprintf(` AND listing_tier = $%d`, n)
		args = append(args, filter.Tier)
		n++
	}
	if filter.Location != "" {
		query += fmt.Sprintf(` AND location ILIKE $%d`, n)
		args = append(args, "%"+filter.Location+"%")
		n++
	}
	if filter.EmploymentType != "" {
		query += fmt.Sprintf(` AND employment_type ILIKE $%d`, n)
		args = append(args, filter.EmploymentType)
		n++
	}
	if filter.MinExperienceYears != nil {
		query += fmt.Sprintf(` AND min_experience_years <= $%d`, n)
		args = append(args, *filter.MinExperienceYears)
		n++
	}
	if filter.CreatedAfter != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, n)
		args = append(args, *filter.CreatedAfter)
		n++
	}
	if filter.Query != "" {
		query += fmt.Sprintf(` AND (company_name ILIKE $%d OR location ILIKE $%d OR required_skills ILIKE $%d)`, n, n, n)
		args = append(args, "%"+filter.Query+"%")
		n++
	}
	query += ` ORDER BY ` + tierOrderSQL()
	if filter.Offset > 0 {
		query += fmt.Sprintf(` OFFSET $%d`, n)
		args = append(args, filter.Offset)
		n++
	}
	query += fmt.Sprintf(` LIMIT $%d`, n)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
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
		if filter.Skills != "" && !skillsOverlap(v.RequiredSkills, filter.Skills) {
			continue
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *VacancyRepository) RecordView(ctx context.Context, vacancyID uuid.UUID, viewerID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO vacancy_views (vacancy_id, viewer_id) VALUES ($1, $2)
	`, vacancyID, viewerID)
	return err
}

func (r *VacancyRepository) ViewCount(ctx context.Context, vacancyID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vacancy_views WHERE vacancy_id = $1`, vacancyID).Scan(&n)
	return n, err
}

func (r *VacancyRepository) Renew(ctx context.Context, id, recruiterID uuid.UUID, tier model.ListingTier) error {
	expires := time.Now().Add(30 * 24 * time.Hour)
	result, err := r.pool.Exec(ctx, `
		UPDATE vacancies
		SET listing_tier = $3, published_at = NOW(), expires_at = $4, status = 'active',
		    is_featured = ($3 = 'premium'), updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2
	`, id, recruiterID, string(tier), expires)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *VacancyRepository) SetTier(ctx context.Context, id, recruiterID uuid.UUID, tier model.ListingTier) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE vacancies
		SET listing_tier = $3, is_featured = ($3 = 'premium'), updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2
	`, id, recruiterID, string(tier))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func skillsOverlap(required, filter string) bool {
	r := splitTrim(required)
	f := splitTrim(filter)
	for _, s := range f {
		for _, t := range r {
			if stringsEqualFold(s, t) {
				return true
			}
		}
	}
	return len(f) == 0
}

func splitTrim(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func stringsEqualFold(a, b string) bool {
	return strings.EqualFold(a, b)
}

func (r *VacancyRepository) Update(ctx context.Context, id, recruiterID uuid.UUID, titleEnc, descriptionEnc []byte, companyName, requiredSkills, location, employmentType string, minExperienceYears int) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE vacancies SET title_enc = $3, description_enc = $4, company_name = $5, required_skills = $6,
			location = $7, employment_type = $8, min_experience_years = $9, updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2
	`, id, recruiterID, titleEnc, descriptionEnc, companyName, requiredSkills, location, employmentType, minExperienceYears)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *VacancyRepository) Delete(ctx context.Context, id, recruiterID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM vacancies WHERE id = $1 AND recruiter_id = $2`, id, recruiterID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
