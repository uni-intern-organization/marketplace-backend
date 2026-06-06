package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

var ErrBannerSlotTaken = errors.New("banner slot taken for period")
var ErrBannerNotFound = errors.New("banner campaign not found")

type BannerRepository struct {
	pool *pgxpool.Pool
}

func NewBannerRepository(pool *pgxpool.Pool) *BannerRepository {
	return &BannerRepository{pool: pool}
}

const bannerCampaignCols = `id, placement_code, recruiter_id, created_by, image_key, link_url,
	starts_at, ends_at, status, payment_session_id, amount_kzt, reject_reason,
	impressions, clicks, priority, created_at, updated_at`

func scanBannerCampaign(row interface{ Scan(dest ...any) error }) (model.BannerCampaign, error) {
	var c model.BannerCampaign
	err := row.Scan(
		&c.ID, &c.PlacementCode, &c.RecruiterID, &c.CreatedBy, &c.ImageKey, &c.LinkURL,
		&c.StartsAt, &c.EndsAt, &c.Status, &c.PaymentSessionID, &c.AmountKZT, &c.RejectReason,
		&c.Impressions, &c.Clicks, &c.Priority, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

func (r *BannerRepository) ListPlacements(ctx context.Context) ([]model.BannerPlacement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT code, name, description, width, height, price_week_kzt, price_month_kzt, is_active
		FROM banner_placements WHERE is_active = true ORDER BY code
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.BannerPlacement, 0)
	for rows.Next() {
		var p model.BannerPlacement
		if err := rows.Scan(&p.Code, &p.Name, &p.Description, &p.Width, &p.Height, &p.PriceWeekKZT, &p.PriceMonthKZT, &p.IsActive); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *BannerRepository) GetPlacement(ctx context.Context, code string) (*model.BannerPlacement, error) {
	var p model.BannerPlacement
	err := r.pool.QueryRow(ctx, `
		SELECT code, name, description, width, height, price_week_kzt, price_month_kzt, is_active
		FROM banner_placements WHERE code = $1
	`, code).Scan(&p.Code, &p.Name, &p.Description, &p.Width, &p.Height, &p.PriceWeekKZT, &p.PriceMonthKZT, &p.IsActive)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *BannerRepository) UpdatePlacementPricing(ctx context.Context, code string, week, month int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE banner_placements SET price_week_kzt = $2, price_month_kzt = $3 WHERE code = $1
	`, code, week, month)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *BannerRepository) HasOverlap(ctx context.Context, placementCode string, starts, ends time.Time, excludeID *uuid.UUID) (bool, error) {
	var n int
	q := `
		SELECT COUNT(*) FROM banner_campaigns
		WHERE placement_code = $1 AND status IN ('pending_review', 'active')
		  AND starts_at < $3 AND ends_at > $2
	`
	args := []interface{}{placementCode, starts, ends}
	if excludeID != nil {
		q += ` AND id <> $4`
		args = append(args, *excludeID)
	}
	err := r.pool.QueryRow(ctx, q, args...).Scan(&n)
	return n > 0, err
}

func (r *BannerRepository) CreateCampaign(ctx context.Context, c *model.BannerCampaign) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO banner_campaigns (placement_code, recruiter_id, created_by, image_key, link_url,
			starts_at, ends_at, status, amount_kzt, priority)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+bannerCampaignCols,
		c.PlacementCode, c.RecruiterID, c.CreatedBy, c.ImageKey, c.LinkURL,
		c.StartsAt, c.EndsAt, c.Status, c.AmountKZT, c.Priority,
	)
	out, err := scanBannerCampaign(row)
	if err != nil {
		return err
	}
	*c = out
	return nil
}

func (r *BannerRepository) GetCampaign(ctx context.Context, id uuid.UUID) (*model.BannerCampaign, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+bannerCampaignCols+` FROM banner_campaigns WHERE id = $1`, id)
	c, err := scanBannerCampaign(row)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *BannerRepository) UpdateCampaignDraft(ctx context.Context, id uuid.UUID, imageKey, linkURL string, starts, ends time.Time, amount int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE banner_campaigns SET image_key=$2, link_url=$3, starts_at=$4, ends_at=$5, amount_kzt=$6, updated_at=NOW()
		WHERE id=$1 AND status IN ('draft','pending_payment')
	`, id, imageKey, linkURL, starts, ends, amount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *BannerRepository) UpdateCampaignPeriod(ctx context.Context, id uuid.UUID, starts, ends time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE banner_campaigns SET starts_at=$2, ends_at=$3, updated_at=NOW() WHERE id=$1
	`, id, starts, ends)
	return err
}

func (r *BannerRepository) GetRecruiterInfo(ctx context.Context, recruiterID uuid.UUID) (email string, err error) {
	err = r.pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, recruiterID).Scan(&email)
	return
}

func (r *BannerRepository) SetCampaignStatus(ctx context.Context, id uuid.UUID, status string, rejectReason *string) error {
	if rejectReason != nil {
		_, err := r.pool.Exec(ctx, `UPDATE banner_campaigns SET status=$2, reject_reason=$3, updated_at=NOW() WHERE id=$1`, id, status, *rejectReason)
		return err
	}
	_, err := r.pool.Exec(ctx, `UPDATE banner_campaigns SET status=$2, updated_at=NOW() WHERE id=$1`, id, status)
	return err
}

func (r *BannerRepository) SetPaymentSession(ctx context.Context, id, sessionID uuid.UUID, amount int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE banner_campaigns SET payment_session_id=$2, amount_kzt=$3, status='pending_review', updated_at=NOW()
		WHERE id=$1 AND status IN ('draft','pending_payment')
	`, id, sessionID, amount)
	return err
}

func (r *BannerRepository) ListByRecruiter(ctx context.Context, recruiterID uuid.UUID) ([]model.BannerCampaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+bannerCampaignCols+` FROM banner_campaigns WHERE recruiter_id=$1 ORDER BY created_at DESC
	`, recruiterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBannerList(rows)
}

func (r *BannerRepository) ListAll(ctx context.Context, status string, limit int) ([]model.BannerCampaign, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = r.pool.Query(ctx, `
			SELECT `+bannerCampaignCols+` FROM banner_campaigns WHERE status=$1 ORDER BY created_at DESC LIMIT $2
		`, status, limit)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT `+bannerCampaignCols+` FROM banner_campaigns ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBannerList(rows)
}

func scanBannerList(rows pgx.Rows) ([]model.BannerCampaign, error) {
	out := make([]model.BannerCampaign, 0)
	for rows.Next() {
		c, err := scanBannerCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *BannerRepository) GetActiveForPlacement(ctx context.Context, placementCode string) (*model.BannerCampaign, error) {
	list, err := r.ListActiveForPlacement(ctx, placementCode, 1)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &list[0], nil
}

func (r *BannerRepository) ListActiveForPlacement(ctx context.Context, placementCode string, limit int) ([]model.BannerCampaign, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+bannerCampaignCols+` FROM banner_campaigns
		WHERE placement_code=$1 AND status='active'
		  AND starts_at <= NOW() AND ends_at > NOW()
		ORDER BY priority DESC, created_at ASC
		LIMIT $2
	`, placementCode, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBannerList(rows)
}

func (r *BannerRepository) GetOccupiedUntil(ctx context.Context, placementCode string) (*time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT MAX(ends_at) FROM banner_campaigns
		WHERE placement_code=$1 AND status IN ('pending_review','active') AND ends_at > NOW()
	`, placementCode).Scan(&t)
	return t, err
}

func (r *BannerRepository) IncrementImpressions(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE banner_campaigns SET impressions = impressions + 1 WHERE id=$1`, id)
	return err
}

func (r *BannerRepository) IncrementClicks(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE banner_campaigns SET clicks = clicks + 1 WHERE id=$1`, id)
	return err
}

func (r *BannerRepository) ExpireCompleted(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE banner_campaigns SET status='completed', updated_at=NOW()
		WHERE status='active' AND ends_at <= NOW()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *BannerRepository) ListExpiringSoon(ctx context.Context, days int) ([]model.BannerCampaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+bannerCampaignCols+` FROM banner_campaigns
		WHERE status='active' AND expiring_notified=false
		  AND ends_at > NOW() AND ends_at <= NOW() + ($1::int * interval '1 day')
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBannerList(rows)
}

func (r *BannerRepository) MarkExpiringNotified(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE banner_campaigns SET expiring_notified=true WHERE id=$1`, id)
	return err
}

func (r *BannerRepository) ListPendingReviewSLA(ctx context.Context, hours int) ([]model.BannerCampaign, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+bannerCampaignCols+` FROM banner_campaigns
		WHERE status='pending_review' AND created_at <= NOW() - ($1::int * interval '1 hour')
	`, hours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBannerList(rows)
}

func CalcBannerPrice(placement *model.BannerPlacement, starts, ends time.Time) int {
	days := int(ends.Sub(starts).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	if days >= 28 {
		months := days / 30
		if months < 1 {
			months = 1
		}
		remainder := days % 30
		return months*placement.PriceMonthKZT + remainder*placement.PriceMonthKZT/30
	}
	return days * placement.PriceWeekKZT / 7
}

func (r *BannerRepository) ListAlternativePlacements(ctx context.Context, excludeCode string, targetWeekPrice int) ([]model.BannerPlacement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT code, name, description, width, height, price_week_kzt, price_month_kzt, is_active
		FROM banner_placements
		WHERE is_active=true AND code <> $1
		ORDER BY ABS(price_week_kzt - $2) ASC LIMIT 3
	`, excludeCode, targetWeekPrice)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.BannerPlacement, 0)
	for rows.Next() {
		var p model.BannerPlacement
		if err := rows.Scan(&p.Code, &p.Name, &p.Description, &p.Width, &p.Height, &p.PriceWeekKZT, &p.PriceMonthKZT, &p.IsActive); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func FormatBannerConflictMsg(code string) string {
	return fmt.Sprintf("slot %s occupied for selected period", code)
}
