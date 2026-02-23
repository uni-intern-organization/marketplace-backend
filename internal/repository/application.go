package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type ApplicationRepository struct {
	pool *pgxpool.Pool
}

func NewApplicationRepository(pool *pgxpool.Pool) *ApplicationRepository {
	return &ApplicationRepository{pool: pool}
}

func (r *ApplicationRepository) Create(ctx context.Context, studentID, recruiterID uuid.UUID, invitationID *uuid.UUID, coverLetterEnc []byte) (*model.Application, error) {
	var app model.Application
	err := r.pool.QueryRow(ctx, `
		INSERT INTO applications (student_id, recruiter_id, invitation_id, cover_letter_enc, status)
		VALUES ($1, $2, $3, $4, 'submitted')
		RETURNING id, student_id, recruiter_id, invitation_id, cover_letter_enc, status, created_at, updated_at
	`, studentID, recruiterID, invitationID, coverLetterEnc).Scan(
		&app.ID, &app.StudentID, &app.RecruiterID, &app.InvitationID, &app.CoverLetterEnc, &app.Status, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *ApplicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	var app model.Application
	err := r.pool.QueryRow(ctx, `
		SELECT id, student_id, recruiter_id, invitation_id, cover_letter_enc, status, created_at, updated_at
		FROM applications WHERE id = $1
	`, id).Scan(&app.ID, &app.StudentID, &app.RecruiterID, &app.InvitationID, &app.CoverLetterEnc, &app.Status, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *ApplicationRepository) ListByStudent(ctx context.Context, studentID uuid.UUID) ([]model.Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, student_id, recruiter_id, invitation_id, cover_letter_enc, status, created_at, updated_at
		FROM applications WHERE student_id = $1 ORDER BY created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Application
	for rows.Next() {
		var app model.Application
		if err := rows.Scan(&app.ID, &app.StudentID, &app.RecruiterID, &app.InvitationID, &app.CoverLetterEnc, &app.Status, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	return list, rows.Err()
}

func (r *ApplicationRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, student_id, recruiter_id, invitation_id, cover_letter_enc, status, created_at, updated_at
		FROM applications WHERE recruiter_id = $1 ORDER BY created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Application
	for rows.Next() {
		var app model.Application
		if err := rows.Scan(&app.ID, &app.StudentID, &app.RecruiterID, &app.InvitationID, &app.CoverLetterEnc, &app.Status, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	return list, rows.Err()
}

func (r *ApplicationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE applications SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	return err
}
