package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VacancyReviewRepository struct {
	pool *pgxpool.Pool
}

type VacancyReview struct {
	ID           uuid.UUID
	VacancyID    uuid.UUID
	AuthorUserID uuid.UUID
	AuthorName   string
	AuthorRole   string
	Text         string
	CreatedAt    time.Time
}

func NewVacancyReviewRepository(pool *pgxpool.Pool) *VacancyReviewRepository {
	return &VacancyReviewRepository{pool: pool}
}

func (r *VacancyReviewRepository) HasCompletedInternship(ctx context.Context, studentID, vacancyID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM applications a
			JOIN vacancies completed_vacancy ON completed_vacancy.id = a.vacancy_id
			JOIN vacancies target_vacancy ON target_vacancy.id = $2
			WHERE a.student_id = $1
			  AND a.status = 'completed'
			  AND completed_vacancy.recruiter_id = target_vacancy.recruiter_id
		)
	`, studentID, vacancyID).Scan(&exists)
	return exists, err
}

func (r *VacancyReviewRepository) Create(ctx context.Context, vacancyID, authorID uuid.UUID, text string) (*VacancyReview, error) {
	var review VacancyReview
	err := r.pool.QueryRow(ctx, `
		INSERT INTO vacancy_reviews (vacancy_id, author_user_id, text)
		VALUES ($1, $2, $3)
		RETURNING id, vacancy_id, author_user_id, text, created_at
	`, vacancyID, authorID, text).Scan(&review.ID, &review.VacancyID, &review.AuthorUserID, &review.Text, &review.CreatedAt)
	if err != nil {
		return nil, err
	}
	review.AuthorRole = "student"
	return &review, nil
}

func (r *VacancyReviewRepository) List(ctx context.Context, vacancyID uuid.UUID) ([]VacancyReview, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.vacancy_id, r.author_user_id, u.email, u.role, r.text, r.created_at
		FROM vacancy_reviews r
		JOIN users u ON u.id = r.author_user_id
		WHERE r.vacancy_id = $1
		ORDER BY r.created_at DESC
	`, vacancyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []VacancyReview
	for rows.Next() {
		var review VacancyReview
		if err := rows.Scan(&review.ID, &review.VacancyID, &review.AuthorUserID, &review.AuthorName, &review.AuthorRole, &review.Text, &review.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, review)
	}
	return list, rows.Err()
}
