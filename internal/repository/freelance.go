package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type FreelanceRepository struct {
	pool *pgxpool.Pool
}

func NewFreelanceRepository(pool *pgxpool.Pool) *FreelanceRepository {
	return &FreelanceRepository{pool: pool}
}

const taskCols = `id, recruiter_id, title_enc, description_enc, category, budget_kzt, deadline,
	required_skills, status, escrow_status, accepted_student_id, created_at, updated_at`

func scanTask(row interface{ Scan(dest ...any) error }) (model.FreelanceTask, error) {
	var t model.FreelanceTask
	err := row.Scan(&t.ID, &t.RecruiterID, &t.TitleEnc, &t.DescriptionEnc, &t.Category, &t.BudgetKZT,
		&t.Deadline, &t.RequiredSkills, &t.Status, &t.EscrowStatus, &t.AcceptedStudentID, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func (r *FreelanceRepository) CreateTask(ctx context.Context, recruiterID uuid.UUID, titleEnc, descEnc []byte, category string, budget float64, deadline time.Time, skills string) (*model.FreelanceTask, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO freelance_tasks (recruiter_id, title_enc, description_enc, category, budget_kzt, deadline, required_skills, status, escrow_status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'open','held') RETURNING `+taskCols,
		recruiterID, titleEnc, descEnc, category, budget, deadline, skills)
	t, err := scanTask(row)
	return &t, err
}

func (r *FreelanceRepository) GetTask(ctx context.Context, id uuid.UUID) (*model.FreelanceTask, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+taskCols+` FROM freelance_tasks WHERE id=$1`, id)
	t, err := scanTask(row)
	return &t, err
}

func (r *FreelanceRepository) ListOpen(ctx context.Context, category string, limit int) ([]model.FreelanceTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `SELECT ` + taskCols + ` FROM freelance_tasks WHERE status='open'`
	args := []any{}
	if category != "" {
		q += ` AND category=$1`
		args = append(args, category)
	}
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT %d`, limit)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.FreelanceTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *FreelanceRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.FreelanceTask, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+taskCols+` FROM freelance_tasks WHERE recruiter_id=$1 ORDER BY created_at DESC`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.FreelanceTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *FreelanceRepository) ListByStudent(ctx context.Context, studentID uuid.UUID) ([]model.FreelanceTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+taskCols+` FROM freelance_tasks
		WHERE accepted_student_id=$1 OR id IN (SELECT task_id FROM freelance_proposals WHERE student_id=$1)
		ORDER BY created_at DESC`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.FreelanceTask
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *FreelanceRepository) UpdateTaskStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE freelance_tasks SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (r *FreelanceRepository) AcceptProposal(ctx context.Context, taskID, studentID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `UPDATE freelance_tasks SET status='in_progress', accepted_student_id=$2, updated_at=NOW() WHERE id=$1`, taskID, studentID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE freelance_proposals SET status='accepted', updated_at=NOW() WHERE task_id=$1 AND student_id=$2`, taskID, studentID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE freelance_proposals SET status='rejected', updated_at=NOW() WHERE task_id=$1 AND student_id<>$2 AND status='pending'`, taskID, studentID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *FreelanceRepository) CreateProposal(ctx context.Context, taskID, studentID uuid.UUID, msgEnc []byte) (*model.FreelanceProposal, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO freelance_proposals (task_id, student_id, message_enc, status)
		VALUES ($1,$2,$3,'pending')
		RETURNING id, task_id, student_id, message_enc, status, created_at, updated_at
	`, taskID, studentID, msgEnc)
	var p model.FreelanceProposal
	err := row.Scan(&p.ID, &p.TaskID, &p.StudentID, &p.MessageEnc, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return &p, err
}

func (r *FreelanceRepository) GetProposal(ctx context.Context, id uuid.UUID) (*model.FreelanceProposal, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, task_id, student_id, message_enc, status, created_at, updated_at
		FROM freelance_proposals WHERE id=$1`, id)
	var p model.FreelanceProposal
	err := row.Scan(&p.ID, &p.TaskID, &p.StudentID, &p.MessageEnc, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return &p, err
}

func (r *FreelanceRepository) UpdateProposalStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE freelance_proposals SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *FreelanceRepository) CreateSubmission(ctx context.Context, taskID, studentID uuid.UUID, key, note string) (*model.FreelanceSubmission, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO freelance_submissions (task_id, student_id, deliverable_key, student_note, status)
		VALUES ($1,$2,$3,$4,'submitted')
		RETURNING id, task_id, student_id, deliverable_key, student_note, revision_count, status, created_at, updated_at
	`, taskID, studentID, key, note)
	var s model.FreelanceSubmission
	err := row.Scan(&s.ID, &s.TaskID, &s.StudentID, &s.DeliverableKey, &s.StudentNote, &s.RevisionCount, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err == nil {
		_, _ = r.pool.Exec(ctx, `UPDATE freelance_tasks SET status='submitted', updated_at=NOW() WHERE id=$1`, taskID)
	}
	return &s, err
}

func (r *FreelanceRepository) GetSubmission(ctx context.Context, id uuid.UUID) (*model.FreelanceSubmission, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, task_id, student_id, deliverable_key, student_note, revision_count, status, created_at, updated_at
		FROM freelance_submissions WHERE id=$1`, id)
	var s model.FreelanceSubmission
	err := row.Scan(&s.ID, &s.TaskID, &s.StudentID, &s.DeliverableKey, &s.StudentNote, &s.RevisionCount, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	return &s, err
}

func (r *FreelanceRepository) UpdateSubmissionStatus(ctx context.Context, id uuid.UUID, status string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE freelance_submissions SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *FreelanceRepository) RequestRevision(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE freelance_submissions SET status='revision_requested', revision_count = revision_count + 1, updated_at=NOW()
		WHERE id=$1
	`, id)
	return err
}

func (r *FreelanceRepository) ListProposalsByTask(ctx context.Context, taskID uuid.UUID) ([]model.FreelanceProposal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, task_id, student_id, message_enc, status, created_at, updated_at
		FROM freelance_proposals WHERE task_id=$1 ORDER BY created_at DESC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.FreelanceProposal
	for rows.Next() {
		var p model.FreelanceProposal
		if err := rows.Scan(&p.ID, &p.TaskID, &p.StudentID, &p.MessageEnc, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (r *FreelanceRepository) ListDisputes(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT d.id, d.task_id, d.opened_by, d.reason, d.status, d.created_at, t.recruiter_id
		FROM freelance_disputes d JOIN freelance_tasks t ON t.id = d.task_id
		WHERE d.status = 'open' ORDER BY d.created_at ASC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, taskID, openedBy, recruiterID uuid.UUID
		var reason, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &taskID, &openedBy, &reason, &status, &createdAt, &recruiterID); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id.String(), "task_id": taskID.String(), "opened_by": openedBy.String(),
			"recruiter_id": recruiterID.String(), "reason": reason, "status": status,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	return out, rows.Err()
}

func (r *FreelanceRepository) CreateReview(ctx context.Context, taskID, reviewerID uuid.UUID, rating int, comment string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO freelance_reviews (task_id, reviewer_id, rating, comment) VALUES ($1,$2,$3,$4)
		ON CONFLICT (task_id, reviewer_id) DO UPDATE SET rating=$3, comment=$4
	`, taskID, reviewerID, rating, comment)
	return err
}

func (r *FreelanceRepository) CreateDispute(ctx context.Context, taskID, openedBy uuid.UUID, reason string) (*model.FreelanceDispute, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO freelance_disputes (task_id, opened_by, reason, status)
		VALUES ($1,$2,$3,'open')
		RETURNING id, task_id, opened_by, reason, COALESCE(resolution,''), status, resolved_by, created_at, resolved_at
	`, taskID, openedBy, reason)
	var d model.FreelanceDispute
	err := row.Scan(&d.ID, &d.TaskID, &d.OpenedBy, &d.Reason, &d.Resolution, &d.Status, &d.ResolvedBy, &d.CreatedAt, &d.ResolvedAt)
	if err == nil {
		_, _ = r.pool.Exec(ctx, `UPDATE freelance_tasks SET status='disputed', updated_at=NOW() WHERE id=$1`, taskID)
	}
	return &d, err
}

func (r *FreelanceRepository) ResolveDispute(ctx context.Context, id uuid.UUID, adminID uuid.UUID, resolution string, favorStudent bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var taskID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT task_id FROM freelance_disputes WHERE id=$1`, id).Scan(&taskID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE freelance_disputes SET status='resolved', resolution=$2, resolved_by=$3, resolved_at=NOW() WHERE id=$1
	`, id, resolution, adminID)
	if err != nil {
		return err
	}
	st := "completed"
	if !favorStudent {
		st = "cancelled"
	}
	_, err = tx.Exec(ctx, `UPDATE freelance_tasks SET status=$2, updated_at=NOW() WHERE id=$1`, taskID, st)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *FreelanceRepository) PortfolioForStudent(ctx context.Context, studentID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.category, t.budget_kzt, t.status,
		       COALESCE((SELECT AVG(rating)::float FROM freelance_reviews WHERE task_id=t.id),0)
		FROM freelance_tasks t
		WHERE t.accepted_student_id=$1 AND t.status='completed'
		ORDER BY t.updated_at DESC LIMIT 20
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var cat, status string
		var budget, avg float64
		if err := rows.Scan(&id, &cat, &budget, &status, &avg); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"task_id": id.String(), "category": cat, "budget_kzt": budget, "status": status, "avg_rating": avg,
		})
	}
	return out, rows.Err()
}
