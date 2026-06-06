package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type StaffRepository struct {
	pool *pgxpool.Pool
}

func NewStaffRepository(pool *pgxpool.Pool) *StaffRepository {
	return &StaffRepository{pool: pool}
}

func (r *StaffRepository) CreateComplaint(ctx context.Context, reporterID uuid.UUID, targetType string, targetID uuid.UUID, reason, details string) (*model.UserComplaint, error) {
	var c model.UserComplaint
	err := r.pool.QueryRow(ctx, `
		INSERT INTO user_complaints (reporter_id, target_type, target_id, reason, details, status)
		VALUES ($1, $2, $3, $4, $5, 'open')
		RETURNING id, reporter_id, target_type, target_id, reason, details, status, reviewed_by, resolution, created_at, resolved_at
	`, reporterID, targetType, targetID, reason, details).Scan(
		&c.ID, &c.ReporterID, &c.TargetType, &c.TargetID, &c.Reason, &c.Details,
		&c.Status, &c.ReviewedBy, &c.Resolution, &c.CreatedAt, &c.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *StaffRepository) ListComplaints(ctx context.Context, status string, limit int) ([]model.UserComplaint, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, reporter_id, target_type, target_id, reason, details, status, reviewed_by, resolution, created_at, resolved_at
			FROM user_complaints WHERE status = $1 ORDER BY created_at DESC LIMIT $2
		`, status, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, reporter_id, target_type, target_id, reason, details, status, reviewed_by, resolution, created_at, resolved_at
			FROM user_complaints ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComplaints(rows)
}

func scanComplaints(rows pgx.Rows) ([]model.UserComplaint, error) {
	var list []model.UserComplaint
	for rows.Next() {
		var c model.UserComplaint
		if err := rows.Scan(
			&c.ID, &c.ReporterID, &c.TargetType, &c.TargetID, &c.Reason, &c.Details,
			&c.Status, &c.ReviewedBy, &c.Resolution, &c.CreatedAt, &c.ResolvedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *StaffRepository) GetComplaint(ctx context.Context, id uuid.UUID) (*model.UserComplaint, error) {
	var c model.UserComplaint
	err := r.pool.QueryRow(ctx, `
		SELECT id, reporter_id, target_type, target_id, reason, details, status, reviewed_by, resolution, created_at, resolved_at
		FROM user_complaints WHERE id = $1
	`, id).Scan(
		&c.ID, &c.ReporterID, &c.TargetType, &c.TargetID, &c.Reason, &c.Details,
		&c.Status, &c.ReviewedBy, &c.Resolution, &c.CreatedAt, &c.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *StaffRepository) ResolveComplaint(ctx context.Context, id, reviewerID uuid.UUID, status, resolution string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_complaints SET status = $2, reviewed_by = $3, resolution = $4, resolved_at = NOW()
		WHERE id = $1
	`, id, status, reviewerID, resolution)
	return err
}

func (r *StaffRepository) CreateStaffTask(ctx context.Context, createdBy uuid.UUID, title, description, entityType string, entityID *uuid.UUID) (*model.StaffTask, error) {
	var t model.StaffTask
	err := r.pool.QueryRow(ctx, `
		INSERT INTO staff_tasks (created_by, title, description, entity_type, entity_id, status)
		VALUES ($1, $2, $3, $4, $5, 'open')
		RETURNING id, created_by, assigned_to, title, description, entity_type, entity_id, status, resolution, resolved_by, created_at, resolved_at
	`, createdBy, title, description, entityType, entityID).Scan(
		&t.ID, &t.CreatedBy, &t.AssignedTo, &t.Title, &t.Description, &t.EntityType, &t.EntityID,
		&t.Status, &t.Resolution, &t.ResolvedBy, &t.CreatedAt, &t.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *StaffRepository) ListStaffTasks(ctx context.Context, status string, limit int) ([]model.StaffTask, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, created_by, assigned_to, title, description, entity_type, entity_id, status, resolution, resolved_by, created_at, resolved_at
			FROM staff_tasks WHERE status = $1 ORDER BY created_at DESC LIMIT $2
		`, status, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, created_by, assigned_to, title, description, entity_type, entity_id, status, resolution, resolved_by, created_at, resolved_at
			FROM staff_tasks ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.StaffTask
	for rows.Next() {
		var t model.StaffTask
		if err := rows.Scan(
			&t.ID, &t.CreatedBy, &t.AssignedTo, &t.Title, &t.Description, &t.EntityType, &t.EntityID,
			&t.Status, &t.Resolution, &t.ResolvedBy, &t.CreatedAt, &t.ResolvedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *StaffRepository) ResolveStaffTask(ctx context.Context, id, adminID uuid.UUID, resolution string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE staff_tasks SET status = 'resolved', resolution = $2, resolved_by = $3, resolved_at = NOW()
		WHERE id = $1 AND status = 'open'
	`, id, resolution, adminID)
	return err
}

func (r *StaffRepository) GetSetting(ctx context.Context, key string) (map[string]interface{}, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `SELECT value FROM platform_settings WHERE key = $1`, key).Scan(&raw)
	if err == pgx.ErrNoRows {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return out, nil
}

func (r *StaffRepository) UpdateSetting(ctx context.Context, key string, value map[string]interface{}, updatedBy uuid.UUID) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO platform_settings (key, value, updated_by, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_by = $3, updated_at = NOW()
	`, key, raw, updatedBy)
	return err
}

func (r *StaffRepository) ModeratorDashboardCounts(ctx context.Context) (vacancies, hackathons, freelance, complaints int, err error) {
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vacancies WHERE status = 'pending_review'`).Scan(&vacancies)
	if err != nil {
		return
	}
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM hackathons WHERE status = 'pending_review'`).Scan(&hackathons)
	if err != nil {
		return
	}
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM freelance_tasks WHERE status = 'pending_review'`).Scan(&freelance)
	if err != nil {
		return
	}
	err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_complaints WHERE status = 'open'`).Scan(&complaints)
	return
}

func ComplaintToMap(c *model.UserComplaint) map[string]interface{} {
	out := map[string]interface{}{
		"id": c.ID.String(), "reporter_id": c.ReporterID.String(),
		"target_type": c.TargetType, "target_id": c.TargetID.String(),
		"reason": c.Reason, "details": c.Details, "status": c.Status,
		"resolution": c.Resolution, "created_at": c.CreatedAt.Format(time.RFC3339),
	}
	if c.ReviewedBy != nil {
		out["reviewed_by"] = c.ReviewedBy.String()
	}
	if c.ResolvedAt != nil {
		out["resolved_at"] = c.ResolvedAt.Format(time.RFC3339)
	}
	return out
}

func StaffTaskToMap(t *model.StaffTask) map[string]interface{} {
	out := map[string]interface{}{
		"id": t.ID.String(), "created_by": t.CreatedBy.String(),
		"title": t.Title, "description": t.Description,
		"entity_type": t.EntityType, "status": t.Status,
		"resolution": t.Resolution, "created_at": t.CreatedAt.Format(time.RFC3339),
	}
	if t.AssignedTo != nil {
		out["assigned_to"] = t.AssignedTo.String()
	}
	if t.EntityID != nil {
		out["entity_id"] = t.EntityID.String()
	}
	if t.ResolvedBy != nil {
		out["resolved_by"] = t.ResolvedBy.String()
	}
	if t.ResolvedAt != nil {
		out["resolved_at"] = t.ResolvedAt.Format(time.RFC3339)
	}
	return out
}
