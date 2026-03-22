package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type InvitationRepository struct {
	pool   *pgxpool.Pool
	aesKey []byte
}

func NewInvitationRepository(pool *pgxpool.Pool, aesKey []byte) *InvitationRepository {
	return &InvitationRepository{pool: pool, aesKey: aesKey}
}

func (r *InvitationRepository) Create(ctx context.Context, recruiterID, studentID, vacancyID uuid.UUID, messageEnc []byte) (*model.Invitation, error) {
	var inv model.Invitation
	vacancyIDPtr := &vacancyID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO invitations (recruiter_id, student_id, vacancy_id, message_enc, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id, recruiter_id, student_id, vacancy_id, message_enc, status, created_at, updated_at
	`, recruiterID, studentID, vacancyIDPtr, messageEnc).Scan(
		&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.VacancyID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *InvitationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Invitation, error) {
	var inv model.Invitation
	var companyNameEnc, titleEnc []byte
	err := r.pool.QueryRow(ctx, `
		SELECT i.id, i.recruiter_id, i.student_id, i.vacancy_id, i.message_enc, i.status, i.created_at, i.updated_at,
		       COALESCE(rp.company_name_enc, ''::bytea) as company_name_enc,
		       COALESCE(v.title_enc, ''::bytea) as title_enc
		FROM invitations i
		LEFT JOIN recruiter_profiles rp ON i.recruiter_id = rp.user_id
		LEFT JOIN vacancies v ON i.vacancy_id = v.id
		WHERE i.id = $1
	`, id).Scan(&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.VacancyID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt, &companyNameEnc, &titleEnc)
	if err != nil {
		return nil, err
	}
	// Decrypt company name
	if len(companyNameEnc) > 0 {
		decrypted, err := crypto.Decrypt(companyNameEnc, r.aesKey)
		if err == nil {
			inv.RecruiterCompanyName = string(decrypted)
		}
	}
	// Decrypt job title
	if len(titleEnc) > 0 {
		decrypted, err := crypto.Decrypt(titleEnc, r.aesKey)
		if err == nil {
			inv.VacancyTitle = string(decrypted)
		}
	}
	return &inv, nil
}

func (r *InvitationRepository) ListByStudent(ctx context.Context, studentID uuid.UUID) ([]model.Invitation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.recruiter_id, i.student_id, i.vacancy_id, i.message_enc, i.status, i.created_at, i.updated_at,
		       COALESCE(rp.company_name_enc, ''::bytea) as company_name_enc,
		       COALESCE(v.title_enc, ''::bytea) as title_enc
		FROM invitations i
		LEFT JOIN recruiter_profiles rp ON i.recruiter_id = rp.user_id
		LEFT JOIN vacancies v ON i.vacancy_id = v.id
		WHERE i.student_id = $1 
		ORDER BY i.created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Invitation
	for rows.Next() {
		var inv model.Invitation
		var companyNameEnc, titleEnc []byte
		if err := rows.Scan(&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.VacancyID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt, &companyNameEnc, &titleEnc); err != nil {
			return nil, err
		}
		// Decrypt company name
		if len(companyNameEnc) > 0 {
			decrypted, err := crypto.Decrypt(companyNameEnc, r.aesKey)
			if err == nil {
				inv.RecruiterCompanyName = string(decrypted)
			}
		}
		// Decrypt job title
		if len(titleEnc) > 0 {
			decrypted, err := crypto.Decrypt(titleEnc, r.aesKey)
			if err == nil {
				inv.VacancyTitle = string(decrypted)
			}
		}
		list = append(list, inv)
	}
	return list, rows.Err()
}

func (r *InvitationRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.Invitation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT i.id, i.recruiter_id, i.student_id, i.vacancy_id, i.message_enc, i.status, i.created_at, i.updated_at,
		       COALESCE(rp.company_name_enc, ''::bytea) as company_name_enc,
		       COALESCE(v.title_enc, ''::bytea) as title_enc
		FROM invitations i
		LEFT JOIN recruiter_profiles rp ON i.recruiter_id = rp.user_id
		LEFT JOIN vacancies v ON i.vacancy_id = v.id
		WHERE i.recruiter_id = $1 
		ORDER BY i.created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Invitation
	for rows.Next() {
		var inv model.Invitation
		var companyNameEnc, titleEnc []byte
		if err := rows.Scan(&inv.ID, &inv.RecruiterID, &inv.StudentID, &inv.VacancyID, &inv.MessageEnc, &inv.Status, &inv.CreatedAt, &inv.UpdatedAt, &companyNameEnc, &titleEnc); err != nil {
			return nil, err
		}
		// Decrypt company name
		if len(companyNameEnc) > 0 {
			decrypted, err := crypto.Decrypt(companyNameEnc, r.aesKey)
			if err == nil {
				inv.RecruiterCompanyName = string(decrypted)
			}
		}
		// Decrypt job title
		if len(titleEnc) > 0 {
			decrypted, err := crypto.Decrypt(titleEnc, r.aesKey)
			if err == nil {
				inv.VacancyTitle = string(decrypted)
			}
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
