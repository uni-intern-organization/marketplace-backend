package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentSession struct {
	ID          uuid.UUID
	RecruiterID uuid.UUID
	Provider    string
	ExternalID  *string
	AmountKZT   int
	Currency    string
	Purpose     string
	Metadata    []byte
	Status      string
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type PaymentMethod struct {
	ID          uuid.UUID
	RecruiterID uuid.UUID
	Provider    string
	TokenRef    string
	Last4       string
	Brand       string
	CreatedAt   time.Time
}

type PaymentRepository struct {
	pool *pgxpool.Pool
}

func NewPaymentRepository(pool *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{pool: pool}
}

func (r *PaymentRepository) CreateSession(ctx context.Context, recruiterID uuid.UUID, provider, externalID string, amount int, purpose string, metadata map[string]interface{}) (*PaymentSession, error) {
	var meta []byte
	if metadata != nil {
		var err error
		meta, err = json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
	}
	var s PaymentSession
	err := r.pool.QueryRow(ctx, `
		INSERT INTO payment_sessions (recruiter_id, provider, external_id, amount_kzt, purpose, metadata, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING id, recruiter_id, provider, external_id, amount_kzt, COALESCE(currency,'KZT'), purpose, metadata, status, created_at, completed_at
	`, recruiterID, provider, externalID, amount, purpose, meta).Scan(
		&s.ID, &s.RecruiterID, &s.Provider, &s.ExternalID, &s.AmountKZT, &s.Currency, &s.Purpose, &s.Metadata, &s.Status, &s.CreatedAt, &s.CompletedAt,
	)
	return &s, err
}

func (r *PaymentRepository) GetSession(ctx context.Context, idStr string) (*PaymentSession, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}
	return r.getSessionByID(ctx, id)
}

func (r *PaymentRepository) getSessionByID(ctx context.Context, id uuid.UUID) (*PaymentSession, error) {
	var s PaymentSession
	err := r.pool.QueryRow(ctx, `
		SELECT id, recruiter_id, provider, external_id, amount_kzt, COALESCE(currency,'KZT'), purpose, metadata, status, created_at, completed_at
		FROM payment_sessions WHERE id = $1
	`, id).Scan(&s.ID, &s.RecruiterID, &s.Provider, &s.ExternalID, &s.AmountKZT, &s.Currency, &s.Purpose, &s.Metadata, &s.Status, &s.CreatedAt, &s.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PaymentRepository) GetSessionByExternalID(ctx context.Context, externalID string) (*PaymentSession, error) {
	var s PaymentSession
	err := r.pool.QueryRow(ctx, `
		SELECT id, recruiter_id, provider, external_id, amount_kzt, COALESCE(currency,'KZT'), purpose, metadata, status, created_at, completed_at
		FROM payment_sessions WHERE external_id = $1
	`, externalID).Scan(&s.ID, &s.RecruiterID, &s.Provider, &s.ExternalID, &s.AmountKZT, &s.Currency, &s.Purpose, &s.Metadata, &s.Status, &s.CreatedAt, &s.CompletedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PaymentRepository) CompleteSession(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE payment_sessions SET status = 'completed', completed_at = NOW() WHERE id = $1 AND status = 'pending'
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *PaymentRepository) ListMethods(ctx context.Context, recruiterID uuid.UUID) ([]PaymentMethod, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, recruiter_id, provider, token_ref, COALESCE(last4,''), COALESCE(brand,''), created_at
		FROM payment_methods WHERE recruiter_id = $1 ORDER BY created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []PaymentMethod
	for rows.Next() {
		var m PaymentMethod
		if err := rows.Scan(&m.ID, &m.RecruiterID, &m.Provider, &m.TokenRef, &m.Last4, &m.Brand, &m.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *PaymentRepository) AddMethod(ctx context.Context, recruiterID uuid.UUID, provider, tokenRef, last4, brand string) (*PaymentMethod, error) {
	var m PaymentMethod
	err := r.pool.QueryRow(ctx, `
		INSERT INTO payment_methods (recruiter_id, provider, token_ref, last4, brand)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, recruiter_id, provider, token_ref, last4, brand, created_at
	`, recruiterID, provider, tokenRef, last4, brand).Scan(
		&m.ID, &m.RecruiterID, &m.Provider, &m.TokenRef, &m.Last4, &m.Brand, &m.CreatedAt,
	)
	return &m, err
}

func (r *PaymentRepository) DeleteMethod(ctx context.Context, id, recruiterID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM payment_methods WHERE id = $1 AND recruiter_id = $2`, id, recruiterID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *PaymentRepository) ApplyPromo(ctx context.Context, code string) (discountPercent int, err error) {
	var maxUses, usesCount int
	var expiresAt *time.Time
	err = r.pool.QueryRow(ctx, `
		SELECT discount_percent, max_uses, uses_count, expires_at FROM promo_codes WHERE UPPER(code) = UPPER($1)
	`, code).Scan(&discountPercent, &maxUses, &usesCount, &expiresAt)
	if err != nil {
		return 0, err
	}
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		return 0, pgx.ErrNoRows
	}
	if usesCount >= maxUses {
		return 0, pgx.ErrNoRows
	}
	_, err = r.pool.Exec(ctx, `UPDATE promo_codes SET uses_count = uses_count + 1 WHERE UPPER(code) = UPPER($1)`, code)
	return discountPercent, err
}

func (r *PaymentRepository) ListAdminTransactions(ctx context.Context, limit int) ([]map[string]interface{}, float64, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, recruiter_id, amount_kzt, purpose, status, created_at
		FROM payment_sessions ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var list []map[string]interface{}
	for rows.Next() {
		var id, recruiterID uuid.UUID
		var amount int
		var purpose, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &recruiterID, &amount, &purpose, &status, &createdAt); err != nil {
			return nil, 0, err
		}
		list = append(list, map[string]interface{}{
			"id": id.String(), "recruiter_id": recruiterID.String(),
			"amount_kzt": amount, "purpose": purpose, "status": status,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}
	var escrow float64
	_ = r.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_kzt),0) FROM escrow_holds WHERE status = 'held'`).Scan(&escrow)
	return list, escrow, rows.Err()
}

func (r *PaymentRepository) AdminDashboardStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{}
	var n int64
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT u.id) FROM users u
		WHERE u.role = 'student' AND COALESCE(u.is_blocked, false) = false
		  AND u.updated_at > NOW() - interval '30 days'
	`).Scan(&n)
	stats["active_students_30d"] = n
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'recruiter' AND COALESCE(is_blocked,false)=false`).Scan(&n)
	stats["active_recruiters"] = n
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vacancies WHERE status = 'active'`).Scan(&n)
	stats["active_vacancies"] = n
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM freelance_tasks WHERE status IN ('open','in_progress')`).Scan(&n)
	stats["active_freelance_tasks"] = n
	var revenue float64
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_kzt),0) FROM payment_sessions
		WHERE status = 'completed' AND created_at > date_trunc('month', NOW())
	`).Scan(&revenue)
	stats["monthly_revenue_kzt"] = revenue
	var pub, sub, fee, hack float64
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM((metadata->>'price_kzt')::float),0) FROM billing_events
		WHERE event_type = 'vacancy_tier' AND created_at > date_trunc('month', NOW())
	`).Scan(&pub)
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM((metadata->>'price_kzt')::float),0) FROM billing_events
		WHERE event_type LIKE 'subscribe_%' AND created_at > date_trunc('month', NOW())
	`).Scan(&sub)
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM((metadata->>'fee_kzt')::float),0) FROM billing_events
		WHERE event_type = 'freelance_fee' AND created_at > date_trunc('month', NOW())
	`).Scan(&fee)
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM((metadata->>'fee_kzt')::float),0) FROM billing_events
		WHERE event_type = 'hackathon_listing' AND created_at > date_trunc('month', NOW())
	`).Scan(&hack)
	stats["revenue_publications"] = pub
	stats["revenue_subscriptions"] = sub
	stats["revenue_freelance_fee"] = fee
	stats["revenue_hackathons"] = hack
	return stats, nil
}

func (r *PaymentRepository) UpdateTariffPrices(ctx context.Context, basic, standard, premium int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO platform_stats (key, value, updated_at) VALUES
			('tariff_basic_kzt', $1, NOW()),
			('tariff_standard_kzt', $2, NOW()),
			('tariff_premium_kzt', $3, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, basic, standard, premium)
	return err
}

func (r *PaymentRepository) GetPublicStats(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	keys := []struct {
		key string
		sql string
	}{
		{"students", `SELECT COUNT(*) FROM users WHERE role = 'student'`},
		{"recruiters", `SELECT COUNT(*) FROM users WHERE role = 'recruiter'`},
		{"vacancies", `SELECT COUNT(*) FROM vacancies WHERE status = 'active'`},
		{"freelance_tasks", `SELECT COUNT(*) FROM freelance_tasks WHERE status = 'open'`},
		{"hackathons", `SELECT COUNT(*) FROM hackathons WHERE status IN ('registration_open','in_progress')`},
	}
	for _, k := range keys {
		var v int64
		if err := r.pool.QueryRow(ctx, k.sql).Scan(&v); err != nil {
			return nil, err
		}
		out[k.key] = v
	}
	rows, err := r.pool.Query(ctx, `SELECT key, value FROM platform_stats`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var key string
			var val int64
			if err := rows.Scan(&key, &val); err == nil {
				out[key] = val
			}
		}
	}
	return out, nil
}

func (r *PaymentRepository) UpsertPushSubscription(ctx context.Context, userID uuid.UUID, endpoint, p256dh, authKey string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh = $3, auth_key = $4
	`, userID, endpoint, p256dh, authKey)
	return err
}
