package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type InvitationRepository struct {
	pool *pgxpool.Pool
}

func NewInvitationRepository(pool *pgxpool.Pool) *InvitationRepository {
	return &InvitationRepository{pool: pool}
}

func (r *InvitationRepository) Create(ctx context.Context, recruiterID, studentID uuid.UUID, messageEnc []byte) (*model.Invitation, error) {
	var inv model.Invitation
	err := r.pool.QueryRow(ctx, `
		INSERT INTO invitations (recruiter_id, student_id, message_enc, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, recruiter_id, student_id, message_enc, status, created_at, updated_at
	`, recruiterID, studentID, messageEnc).Scan(
		&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvitationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
	var inv model.Invitation
	err := r.pool.QueryRow(ctx, `
		SELECT id, recruiter_id, student_id, message_enc, status, created_at, updated_at
		FROM invitations WHERE id = $1
	`, id).Scan(&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvitationRepository) ListByStudent(ctx context.Context, studentID uuid.UUID) ([]model.Invitation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, recruiter_id, student_id, message_enc, status, created_at, updated_at
		FROM invitations WHERE student_id = $1 ORDER BY created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Invitation
	for rows.Next() {
		var inv model.Invitation
		if err := rows.Scan(&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func (r *InvitationRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.Invitation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, recruiter_id, student_id, message_enc, status, created_at, updated_at
		FROM invitations WHERE recruiter_id = $1 ORDER BY created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Invitation
	for rows.Next() {
		var inv model.Invitation
		if err := rows.Scan(&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func (r *InvitationRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE invitations SET status = $2, updated_at = NOW() WHERE id = $1`, id, status)
	return err
}

func (r *InvitationRepository) Exists(ctx context.Context, recruiterID, studentID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invitations WHERE recruiter_id = $1 AND student_id = $2)`, recruiterID, studentID).Scan(&exists)
	return exists, err
}
