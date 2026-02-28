package repository

import (
	"context"
	"fmt"
	"strings"

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

func (r *VacancyRepository) Create(ctx context.Context, recruiterID uuid.UUID, titleEnc, descriptionEnc []byte, requiredSkills, location, employmentType string, minExperienceYears int) (*model.Vacancy, error) {
	var v model.Vacancy
	err := r.pool.QueryRow(ctx, `
		INSERT INTO vacancies (recruiter_id, title_enc, description_enc, required_skills, location, employment_type, min_experience_years)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, recruiter_id, title_enc, description_enc, required_skills, location, employment_type, min_experience_years, created_at, updated_at
	`, recruiterID, titleEnc, descriptionEnc, requiredSkills, location, employmentType, minExperienceYears).Scan(
		&v.ID, &v.RecruiterID, &v.TitleEnc, &v.DescriptionEnc, &v.RequiredSkills, &v.Location, &v.EmploymentType, &v.MinExperienceYears, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VacancyRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Vacancy, error) {
	var v model.Vacancy
	err := r.pool.QueryRow(ctx, `
		SELECT id, recruiter_id, title_enc, description_enc, COALESCE(required_skills,''), COALESCE(location,''), COALESCE(employment_type,''), COALESCE(min_experience_years,0), created_at, updated_at
		FROM vacancies WHERE id = $1
	`, id).Scan(&v.ID, &v.RecruiterID, &v.TitleEnc, &v.DescriptionEnc, &v.RequiredSkills, &v.Location, &v.EmploymentType, &v.MinExperienceYears, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VacancyRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.Vacancy, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, recruiter_id, title_enc, description_enc, COALESCE(required_skills,''), COALESCE(location,''), COALESCE(employment_type,''), COALESCE(min_experience_years,0), created_at, updated_at
		FROM vacancies WHERE recruiter_id = $1 ORDER BY created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Vacancy
	for rows.Next() {
		var v model.Vacancy
		if err := rows.Scan(&v.ID, &v.RecruiterID, &v.TitleEnc, &v.DescriptionEnc, &v.RequiredSkills, &v.Location, &v.EmploymentType, &v.MinExperienceYears, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, nil
}

type VacancyFilter struct {
	Skills            string // comma-separated, any match
	Location          string
	EmploymentType    string
	MinExperienceYears *int
}

func (r *VacancyRepository) List(ctx context.Context, filter VacancyFilter, limit int) ([]model.Vacancy, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT id, recruiter_id, title_enc, description_enc, COALESCE(required_skills,''), COALESCE(location,''), COALESCE(employment_type,''), COALESCE(min_experience_years,0), created_at, updated_at
		FROM vacancies WHERE 1=1
	`
	args := []interface{}{}
	n := 1
	if filter.Location != "" {
		query += fmt.Sprintf(" AND location ILIKE $%d", n)
		args = append(args, "%"+filter.Location+"%")
		n++
	}
	if filter.EmploymentType != "" {
		query += fmt.Sprintf(" AND employment_type ILIKE $%d", n)
		args = append(args, filter.EmploymentType)
		n++
	}
	if filter.MinExperienceYears != nil {
		query += fmt.Sprintf(" AND min_experience_years <= $%d", n)
		args = append(args, *filter.MinExperienceYears)
		n++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", n)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Vacancy
	for rows.Next() {
		var v model.Vacancy
		if err := rows.Scan(&v.ID, &v.RecruiterID, &v.TitleEnc, &v.DescriptionEnc, &v.RequiredSkills, &v.Location, &v.EmploymentType, &v.MinExperienceYears, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		if filter.Skills != "" {
			if !skillsOverlap(v.RequiredSkills, filter.Skills) {
				continue
			}
		}
		list = append(list, v)
	}
	return list, nil
}

func skillsOverlap(required, filter string) bool {
	// simple: if filter is comma-separated, at least one must be in required
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
	// comma-separated, trim spaces
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

func (r *VacancyRepository) Update(ctx context.Context, id, recruiterID uuid.UUID, titleEnc, descriptionEnc []byte, requiredSkills, location, employmentType string, minExperienceYears int) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE vacancies SET title_enc = $3, description_enc = $4, required_skills = $5, location = $6, employment_type = $7, min_experience_years = $8, updated_at = NOW()
		WHERE id = $1 AND recruiter_id = $2
	`, id, recruiterID, titleEnc, descriptionEnc, requiredSkills, location, employmentType, minExperienceYears)
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
