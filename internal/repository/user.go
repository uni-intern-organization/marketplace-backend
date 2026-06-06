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

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string, role model.UserRole) (*model.User, error) {
	var u model.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, role, COALESCE(is_blocked, false), created_at, updated_at
	`, email, passwordHash, role).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.IsBlocked, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) scanUser(row interface{ Scan(dest ...any) error }) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.IsBlocked, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, COALESCE(is_blocked, false), created_at, updated_at
		FROM users WHERE id = $1
	`, id)
	return r.scanUser(row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, COALESCE(is_blocked, false), created_at, updated_at
		FROM users WHERE email = $1
	`, email)
	return r.scanUser(row)
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

func (r *UserRepository) Search(ctx context.Context, q string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows pgx.Rows
	var err error
	if q != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, email, role, COALESCE(is_blocked, false), created_at
			FROM users WHERE email ILIKE $1 OR id::text = $1 LIMIT $2
		`, "%"+q+"%", limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, email, role, COALESCE(is_blocked, false), created_at
			FROM users ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var email, role string
		var blocked bool
		var createdAt time.Time
		if err := rows.Scan(&id, &email, &role, &blocked, &createdAt); err != nil {
			return nil, err
		}
		list = append(list, map[string]interface{}{
			"id": id.String(), "email": email, "role": role, "is_blocked": blocked,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	return list, rows.Err()
}

func (r *UserRepository) UpdateRole(ctx context.Context, id uuid.UUID, role model.UserRole) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1`, id, role)
	return err
}

func (r *UserRepository) SetBlocked(ctx context.Context, id uuid.UUID, blocked bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET is_blocked = $2, updated_at = NOW() WHERE id = $1`, id, blocked)
	return err
}

func (r *UserRepository) CreateWithSource(ctx context.Context, email, passwordHash string, role model.UserRole, source string, meta map[string]interface{}) (*model.User, error) {
	if source == "" {
		source = "direct"
	}
	var metaJSON []byte
	if meta != nil {
		var err error
		metaJSON, err = json.Marshal(meta)
		if err != nil {
			return nil, err
		}
	}
	var u model.User
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, registration_source, registration_metadata)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, password_hash, role, COALESCE(is_blocked, false), created_at, updated_at
	`, email, passwordHash, role, source, metaJSON).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.IsBlocked, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) InsertUserBlock(ctx context.Context, userID, blockedBy uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_blocks (user_id, blocked_by, reason) VALUES ($1, $2, $3)
	`, userID, blockedBy, reason)
	return err
}

func (r *UserRepository) FindRelatedUsers(ctx context.Context, userID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		WITH target_ips AS (
			SELECT DISTINCT la.ip_address
			FROM login_attempts la
			WHERE la.email = $2 AND la.created_at > NOW() - interval '30 days' AND la.ip_address <> ''
		),
		ip_matches AS (
			SELECT DISTINCT u.id, u.email, u.role, COALESCE(u.is_blocked, false) AS is_blocked, 'shared_ip' AS match_reason
			FROM users u
			JOIN login_attempts la ON la.email = u.email
			JOIN target_ips ti ON ti.ip_address = la.ip_address
			WHERE u.id <> $1 AND la.created_at > NOW() - interval '30 days'
		),
		bin_matches AS (
			SELECT DISTINCT u.id, u.email, u.role, COALESCE(u.is_blocked, false) AS is_blocked, 'shared_bin' AS match_reason
			FROM users u
			JOIN recruiter_verifications rv ON rv.recruiter_id = u.id
			WHERE rv.bin IN (
				SELECT bin FROM recruiter_verifications WHERE recruiter_id = $1 AND status = 'approved'
			) AND u.id <> $1
		)
		SELECT id, email, role, is_blocked, match_reason FROM (
			SELECT * FROM ip_matches
			UNION
			SELECT * FROM bin_matches
		) combined
		LIMIT $3
	`, userID, user.Email, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var email, role, reason string
		var blocked bool
		if err := rows.Scan(&id, &email, &role, &blocked, &reason); err != nil {
			return nil, err
		}
		list = append(list, map[string]interface{}{
			"id": id.String(), "email": email, "role": role, "is_blocked": blocked, "match_reason": reason,
		})
	}
	return list, rows.Err()
}
