package repository

import (
	"context"
	"time"

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

func (r *ApplicationRepository) Create(ctx context.Context, studentID, recruiterID, vacancyID uuid.UUID, invitationID *uuid.UUID, coverLetterEnc []byte) (*model.Application, error) {
	var app model.Application
	err := r.pool.QueryRow(ctx, `
		INSERT INTO applications (student_id, recruiter_id, vacancy_id, invitation_id, cover_letter_enc, status)
		VALUES ($1, $2, $3, $4, $5, 'new')
		RETURNING id, student_id, recruiter_id, vacancy_id, invitation_id, cover_letter_enc, status,
		          COALESCE(interview_format,''), interview_message, proposed_slots, interview_scheduled_at,
		          decision_reason, offer_start_date, offer_terms, offer_duration, created_at, updated_at
	`, studentID, recruiterID, vacancyID, invitationID, coverLetterEnc).Scan(
		&app.ID, &app.StudentID, &app.RecruiterID, &app.VacancyID, &app.InvitationID, &app.CoverLetterEnc, &app.Status,
		&app.InterviewFormat, &app.InterviewMessage, &app.ProposedSlots, &app.InterviewScheduledAt,
		&app.DecisionReason, &app.OfferStartDate, &app.OfferTerms, &app.OfferDuration, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *ApplicationRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Application, error) {
	var app model.Application
	err := r.pool.QueryRow(ctx, `
		SELECT id, student_id, recruiter_id, vacancy_id, invitation_id, cover_letter_enc, status,
		       COALESCE(interview_format,''), interview_message, proposed_slots, interview_scheduled_at,
		       decision_reason, offer_start_date, offer_terms, offer_duration, created_at, updated_at
		FROM applications WHERE id = $1
	`, id).Scan(&app.ID, &app.StudentID, &app.RecruiterID, &app.VacancyID, &app.InvitationID, &app.CoverLetterEnc, &app.Status,
		&app.InterviewFormat, &app.InterviewMessage, &app.ProposedSlots, &app.InterviewScheduledAt,
		&app.DecisionReason, &app.OfferStartDate, &app.OfferTerms, &app.OfferDuration, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (r *ApplicationRepository) ListByStudent(ctx context.Context, studentID uuid.UUID) ([]model.Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, student_id, recruiter_id, vacancy_id, invitation_id, cover_letter_enc, status,
		       COALESCE(interview_format,''), interview_message, proposed_slots, interview_scheduled_at,
		       decision_reason, offer_start_date, offer_terms, offer_duration, created_at, updated_at
		FROM applications WHERE student_id = $1 ORDER BY created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Application
	for rows.Next() {
		var app model.Application
		if err := scanApplication(rows, &app); err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	return list, rows.Err()
}

func (r *ApplicationRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.Application, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, student_id, recruiter_id, vacancy_id, invitation_id, cover_letter_enc, status,
		       COALESCE(interview_format,''), interview_message, proposed_slots, interview_scheduled_at,
		       decision_reason, offer_start_date, offer_terms, offer_duration, created_at, updated_at
		FROM applications WHERE recruiter_id = $1 ORDER BY created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Application
	for rows.Next() {
		var app model.Application
		if err := scanApplication(rows, &app); err != nil {
			return nil, err
		}
		list = append(list, app)
	}
	return list, rows.Err()
}

type applicationScanner interface {
	Scan(dest ...interface{}) error
}

func scanApplication(row applicationScanner, app *model.Application) error {
	return row.Scan(&app.ID, &app.StudentID, &app.RecruiterID, &app.VacancyID, &app.InvitationID, &app.CoverLetterEnc, &app.Status,
		&app.InterviewFormat, &app.InterviewMessage, &app.ProposedSlots, &app.InterviewScheduledAt,
		&app.DecisionReason, &app.OfferStartDate, &app.OfferTerms, &app.OfferDuration, &app.CreatedAt, &app.UpdatedAt)
}

type ApplicationLifecycleUpdate struct {
	Status               string
	InterviewFormat      string
	InterviewMessage     string
	ProposedSlots        []time.Time
	InterviewScheduledAt *time.Time
	DecisionReason       string
	OfferStartDate       *time.Time
	OfferTerms           string
	OfferDuration        string
}

func (r *ApplicationRepository) UpdateLifecycle(ctx context.Context, id uuid.UUID, u ApplicationLifecycleUpdate) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE applications SET
			status = $2,
			interview_format = COALESCE(NULLIF($3,''), interview_format),
			interview_message = CASE WHEN $4 <> '' THEN $4 ELSE interview_message END,
			proposed_slots = CASE WHEN cardinality($5::timestamptz[]) > 0 THEN $5 ELSE proposed_slots END,
			interview_scheduled_at = COALESCE($6, interview_scheduled_at),
			decision_reason = CASE WHEN $7 <> '' THEN $7 ELSE decision_reason END,
			offer_start_date = COALESCE($8, offer_start_date),
			offer_terms = CASE WHEN $9 <> '' THEN $9 ELSE offer_terms END,
			offer_duration = CASE WHEN $10 <> '' THEN $10 ELSE offer_duration END,
			updated_at = NOW()
		WHERE id = $1
	`, id, u.Status, u.InterviewFormat, u.InterviewMessage, u.ProposedSlots, u.InterviewScheduledAt,
		u.DecisionReason, u.OfferStartDate, u.OfferTerms, u.OfferDuration)
	return err
}

func (r *ApplicationRepository) GetReviewByApplication(ctx context.Context, applicationID uuid.UUID) (map[string]interface{}, error) {
	var id, reviewerID, studentID uuid.UUID
	var rating int
	var comment string
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT id, reviewer_id, student_id, rating, comment, created_at
		FROM application_reviews WHERE application_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, applicationID).Scan(&id, &reviewerID, &studentID, &rating, &comment, &createdAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id.String(), "application_id": applicationID.String(),
		"reviewer_id": reviewerID.String(), "student_id": studentID.String(),
		"rating": rating, "comment": comment, "created_at": createdAt.Format(time.RFC3339),
	}, nil
}

func (r *ApplicationRepository) GetReviewByID(ctx context.Context, id uuid.UUID) (map[string]interface{}, error) {
	var applicationID, reviewerID, studentID uuid.UUID
	var rating int
	var comment string
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT application_id, reviewer_id, student_id, rating, comment, created_at
		FROM application_reviews WHERE id = $1
	`, id).Scan(&applicationID, &reviewerID, &studentID, &rating, &comment, &createdAt)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id": id.String(), "application_id": applicationID.String(),
		"reviewer_id": reviewerID.String(), "student_id": studentID.String(),
		"rating": rating, "comment": comment, "created_at": createdAt.Format(time.RFC3339),
	}, nil
}

func (r *ApplicationRepository) CreateReview(ctx context.Context, applicationID, reviewerID, studentID uuid.UUID, rating int, comment string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO application_reviews (application_id, reviewer_id, student_id, rating, comment)
		VALUES ($1, $2, $3, $4, $5)
	`, applicationID, reviewerID, studentID, rating, comment)
	return err
}

func (r *ApplicationRepository) ListReviewsByStudent(ctx context.Context, studentID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.application_id, r.rating, r.comment, r.created_at,
		       COALESCE(v.company_name, '')
		FROM application_reviews r
		JOIN applications a ON a.id = r.application_id
		LEFT JOIN vacancies v ON v.id = a.vacancy_id
		WHERE r.student_id = $1
		ORDER BY r.created_at DESC
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id, applicationID uuid.UUID
		var rating int
		var comment, companyName string
		var createdAt time.Time
		if err := rows.Scan(&id, &applicationID, &rating, &comment, &createdAt, &companyName); err != nil {
			return nil, err
		}
		list = append(list, map[string]interface{}{
			"id": id.String(), "application_id": applicationID.String(), "rating": rating,
			"comment": comment, "company_name": companyName, "created_at": createdAt.Format(time.RFC3339),
		})
	}
	return list, rows.Err()
}
