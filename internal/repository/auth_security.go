package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthSecurityRepository struct {
	pool *pgxpool.Pool
}

func NewAuthSecurityRepository(pool *pgxpool.Pool) *AuthSecurityRepository {
	return &AuthSecurityRepository{pool: pool}
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (r *AuthSecurityRepository) CreateRefreshToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashToken(token), expiresAt)
	return err
}

func (r *AuthSecurityRepository) GetRefreshTokenUserID(ctx context.Context, token string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
	`, hashToken(token)).Scan(&userID)
	return userID, err
}

func (r *AuthSecurityRepository) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1
	`, hashToken(token))
	return err
}

func (r *AuthSecurityRepository) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

func (r *AuthSecurityRepository) RecordLoginAttempt(ctx context.Context, email, ip string, success bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO login_attempts (email, ip_address, success) VALUES ($1, $2, $3)
	`, email, ip, success)
	return err
}

func (r *AuthSecurityRepository) FailedLoginCount(ctx context.Context, email string, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE email = $1 AND success = false AND created_at > $2
	`, email, since).Scan(&count)
	return count, err
}

func (r *AuthSecurityRepository) CreatePasswordResetToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)
	`, userID, hashToken(token), expiresAt)
	return err
}

func (r *AuthSecurityRepository) ConsumePasswordResetToken(ctx context.Context, token string) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		UPDATE password_reset_tokens SET used_at = NOW()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
		RETURNING user_id
	`, hashToken(token)).Scan(&userID)
	return userID, err
}

func (r *AuthSecurityRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1
	`, userID, passwordHash)
	return err
}

func (r *AuthSecurityRepository) SaveTOTPSecret(ctx context.Context, userID uuid.UUID, secretEnc []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_totp (user_id, secret_enc) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET secret_enc = $2, updated_at = NOW()
	`, userID, secretEnc)
	return err
}

func (r *AuthSecurityRepository) GetTOTPSecret(ctx context.Context, userID uuid.UUID) ([]byte, bool, error) {
	var secret []byte
	var enabled bool
	err := r.pool.QueryRow(ctx, `
		SELECT secret_enc, enabled FROM user_totp WHERE user_id = $1
	`, userID).Scan(&secret, &enabled)
	if err != nil {
		return nil, false, err
	}
	return secret, enabled, nil
}

func (r *AuthSecurityRepository) EnableTOTP(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE user_totp SET enabled = true, verified = true, updated_at = NOW() WHERE user_id = $1
	`, userID)
	return err
}

func (r *AuthSecurityRepository) IsTOTPEnabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	var enabled bool
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(enabled, false) FROM user_totp WHERE user_id = $1
	`, userID).Scan(&enabled)
	if err != nil {
		return false, nil
	}
	return enabled, nil
}

func (r *AuthSecurityRepository) SavePushSubscription(ctx context.Context, userID uuid.UUID, endpoint, p256dh, authKey string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh = $3, auth_key = $4
	`, userID, endpoint, p256dh, authKey)
	return err
}

func (r *AuthSecurityRepository) ListPushSubscriptions(ctx context.Context, userID uuid.UUID) ([]map[string]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT endpoint, p256dh, auth_key FROM push_subscriptions WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []map[string]string
	for rows.Next() {
		var ep, p256, ak string
		if err := rows.Scan(&ep, &p256, &ak); err != nil {
			return nil, err
		}
		list = append(list, map[string]string{"endpoint": ep, "p256dh": p256, "auth_key": ak})
	}
	return list, rows.Err()
}
