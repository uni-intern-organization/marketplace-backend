package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type HackathonRepository struct {
	pool *pgxpool.Pool
}

func NewHackathonRepository(pool *pgxpool.Pool) *HackathonRepository {
	return &HackathonRepository{pool: pool}
}

const hackathonCols = `id, organizer_id, title_enc, description_enc, theme, format, prize_pool_kzt,
	min_participants, max_participants, starts_at, ends_at, registration_deadline,
	listing_fee_paid, status, created_at, updated_at`

func scanHackathon(row interface{ Scan(dest ...any) error }) (model.Hackathon, error) {
	var h model.Hackathon
	err := row.Scan(&h.ID, &h.OrganizerID, &h.TitleEnc, &h.DescriptionEnc, &h.Theme, &h.Format,
		&h.PrizePoolKZT, &h.MinParticipants, &h.MaxParticipants, &h.StartsAt, &h.EndsAt,
		&h.RegistrationDeadline, &h.ListingFeePaid, &h.Status, &h.CreatedAt, &h.UpdatedAt)
	return h, err
}

func (r *HackathonRepository) Create(ctx context.Context, organizerID uuid.UUID, titleEnc, descEnc []byte, theme, format string, prize float64, minP, maxP int, starts, ends, regDeadline time.Time) (*model.Hackathon, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO hackathons (organizer_id, title_enc, description_enc, theme, format, prize_pool_kzt,
			min_participants, max_participants, starts_at, ends_at, registration_deadline, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'draft') RETURNING `+hackathonCols,
		organizerID, titleEnc, descEnc, theme, format, prize, minP, maxP, starts, ends, regDeadline)
	h, err := scanHackathon(row)
	return &h, err
}

func (r *HackathonRepository) Get(ctx context.Context, id uuid.UUID) (*model.Hackathon, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+hackathonCols+` FROM hackathons WHERE id=$1`, id)
	h, err := scanHackathon(row)
	return &h, err
}

func (r *HackathonRepository) ListPublished(ctx context.Context, limit int) ([]model.Hackathon, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+hackathonCols+` FROM hackathons
		WHERE status IN ('published','active','registration_open','registration_closed','in_progress','evaluation') AND listing_fee_paid = true
		ORDER BY starts_at ASC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Hackathon
	for rows.Next() {
		h, err := scanHackathon(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

func (r *HackathonRepository) ListByOrganizer(ctx context.Context, organizerID uuid.UUID) ([]model.Hackathon, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+hackathonCols+` FROM hackathons WHERE organizer_id=$1 ORDER BY created_at DESC`, organizerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Hackathon
	for rows.Next() {
		h, err := scanHackathon(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

func (r *HackathonRepository) Publish(ctx context.Context, id, organizerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathons SET listing_fee_paid=true, status='pending_review', updated_at=NOW()
		WHERE id=$1 AND organizer_id=$2`, id, organizerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) Register(ctx context.Context, hackathonID, studentID uuid.UUID, teamID *uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO hackathon_registrations (hackathon_id, student_id, team_id) VALUES ($1,$2,$3)
	`, hackathonID, studentID, teamID)
	return err
}

func (r *HackathonRepository) CreateTeam(ctx context.Context, hackathonID, captainID uuid.UUID, name string) (*model.HackathonTeam, error) {
	code, _ := randomCode(8)
	row := r.pool.QueryRow(ctx, `
		INSERT INTO hackathon_teams (hackathon_id, name, captain_id, invite_code)
		VALUES ($1,$2,$3,$4)
		RETURNING id, hackathon_id, name, captain_id, invite_code, created_at
	`, hackathonID, name, captainID, code)
	var t model.HackathonTeam
	err := row.Scan(&t.ID, &t.HackathonID, &t.Name, &t.CaptainID, &t.InviteCode, &t.CreatedAt)
	return &t, err
}

func randomCode(n int) (string, error) {
	b := make([]byte, n/2+1)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%08d", time.Now().UnixNano()%100000000), nil
	}
	return hex.EncodeToString(b)[:n], nil
}

func (r *HackathonRepository) JoinTeamByCode(ctx context.Context, hackathonID, studentID uuid.UUID, inviteCode string) (*uuid.UUID, error) {
	var teamID uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM hackathon_teams WHERE hackathon_id=$1 AND invite_code=$2`, hackathonID, inviteCode).Scan(&teamID)
	if err != nil {
		return nil, err
	}
	return &teamID, r.Register(ctx, hackathonID, studentID, &teamID)
}

func (r *HackathonRepository) CreateSubmission(ctx context.Context, hackathonID uuid.UUID, teamID, studentID *uuid.UUID, artifactKey string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO hackathon_submissions (hackathon_id, team_id, student_id, artifact_key)
		VALUES ($1,$2,$3,$4)`, hackathonID, teamID, studentID, artifactKey)
	return err
}

func (r *HackathonRepository) PublishResults(ctx context.Context, hackathonID uuid.UUID, results []model.HackathonResult) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, res := range results {
		_, err = tx.Exec(ctx, `
			INSERT INTO hackathon_results (hackathon_id, team_id, student_id, place, prize_amount_kzt, internship_offer)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			hackathonID, res.TeamID, res.StudentID, res.Place, res.PrizeAmountKZT, res.InternshipOffer)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE hackathons SET status='completed', updated_at=NOW() WHERE id=$1`, hackathonID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO hackathon_certificates (hackathon_id, student_id, certificate_url)
		SELECT $1, student_id, 'demo://certificate/' || hackathon_id::text || '/' || student_id::text
		FROM hackathon_registrations WHERE hackathon_id=$1
		ON CONFLICT (hackathon_id, student_id) DO NOTHING`, hackathonID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *HackathonRepository) Leaderboard(ctx context.Context, hackathonID uuid.UUID) ([]model.HackathonResult, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, hackathon_id, team_id, student_id, place, prize_amount_kzt, internship_offer, created_at
		FROM hackathon_results WHERE hackathon_id=$1 ORDER BY place ASC`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.HackathonResult
	for rows.Next() {
		var res model.HackathonResult
		if err := rows.Scan(&res.ID, &res.HackathonID, &res.TeamID, &res.StudentID, &res.Place, &res.PrizeAmountKZT, &res.InternshipOffer, &res.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, res)
	}
	return list, rows.Err()
}

func (r *HackathonRepository) PortfolioForStudent(ctx context.Context, studentID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT h.id, h.theme, hr.place, hr.prize_amount_kzt, hr.internship_offer
		FROM hackathon_results hr
		JOIN hackathons h ON h.id = hr.hackathon_id
		WHERE hr.student_id=$1 OR hr.team_id IN (
			SELECT team_id FROM hackathon_registrations WHERE student_id=$1 AND team_id IS NOT NULL
		)
		ORDER BY hr.place ASC LIMIT 20
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var theme string
		var place int
		var prize float64
		var offer bool
		if err := rows.Scan(&id, &theme, &place, &prize, &offer); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"hackathon_id": id.String(), "theme": theme, "place": place,
			"prize_amount_kzt": prize, "internship_offer": offer,
		})
	}
	return out, rows.Err()
}

func (r *HackathonRepository) AddAnnouncement(ctx context.Context, hackathonID uuid.UUID, title, body string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO hackathon_announcements (hackathon_id, title, body) VALUES ($1,$2,$3)
	`, hackathonID, title, body)
	return err
}

func (r *HackathonRepository) ListAnnouncements(ctx context.Context, hackathonID uuid.UUID) ([]model.HackathonAnnouncement, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, hackathon_id, title, body, created_at FROM hackathon_announcements
		WHERE hackathon_id=$1 ORDER BY created_at DESC`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.HackathonAnnouncement
	for rows.Next() {
		var a model.HackathonAnnouncement
		if err := rows.Scan(&a.ID, &a.HackathonID, &a.Title, &a.Body, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (r *HackathonRepository) RegistrationCount(ctx context.Context, hackathonID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM hackathon_registrations WHERE hackathon_id=$1`, hackathonID).Scan(&n)
	return n, err
}

func (r *HackathonRepository) ListCriteria(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, weight_percent, sort_order FROM hackathon_criteria
		WHERE hackathon_id=$1 ORDER BY sort_order ASC
	`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		var weight, sort int
		if err := rows.Scan(&id, &name, &weight, &sort); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id.String(), "name": name, "weight_percent": weight, "sort_order": sort,
		})
	}
	return out, rows.Err()
}

func (r *HackathonRepository) CreateCriterion(ctx context.Context, hackathonID uuid.UUID, name string, weight, sort int) (map[string]interface{}, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO hackathon_criteria (hackathon_id, name, weight_percent, sort_order)
		VALUES ($1,$2,$3,$4) RETURNING id
	`, hackathonID, name, weight, sort).Scan(&id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"id": id.String(), "name": name, "weight_percent": weight, "sort_order": sort}, nil
}

func (r *HackathonRepository) UpdateCriterion(ctx context.Context, criterionID, organizerID uuid.UUID, name string, weight, sort int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathon_criteria c SET name=COALESCE(NULLIF($3,''), name),
			weight_percent=CASE WHEN $4>0 THEN $4 ELSE weight_percent END,
			sort_order=CASE WHEN $5>=0 THEN $5 ELSE sort_order END
		FROM hackathons h WHERE c.id=$1 AND c.hackathon_id=h.id AND h.organizer_id=$2
	`, criterionID, organizerID, name, weight, sort)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) DeleteCriterion(ctx context.Context, criterionID, organizerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM hackathon_criteria c USING hackathons h
		WHERE c.id=$1 AND c.hackathon_id=h.id AND h.organizer_id=$2
	`, criterionID, organizerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) UpsertScore(ctx context.Context, hackathonID, submissionID, criterionID uuid.UUID, score float64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO hackathon_scores (hackathon_id, submission_id, criterion_id, score)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (submission_id, criterion_id) DO UPDATE SET score=$4
	`, hackathonID, submissionID, criterionID, score)
	return err
}

func (r *HackathonRepository) ListTeamRequests(ctx context.Context, teamID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, student_id, COALESCE(message,''), status, created_at
		FROM hackathon_team_requests WHERE team_id=$1 ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, tid, sid uuid.UUID
		var msg, status string
		var createdAt time.Time
		if err := rows.Scan(&id, &tid, &sid, &msg, &status, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id.String(), "team_id": tid.String(), "student_id": sid.String(),
			"message": msg, "status": status, "created_at": createdAt.Format(time.RFC3339),
		})
	}
	return out, rows.Err()
}

func (r *HackathonRepository) CreateTeamRequest(ctx context.Context, teamID, studentID uuid.UUID, message string) (map[string]interface{}, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO hackathon_team_requests (team_id, student_id, message, status)
		VALUES ($1,$2,$3,'pending') RETURNING id
	`, teamID, studentID, message).Scan(&id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"id": id.String(), "status": "pending"}, nil
}

func (r *HackathonRepository) UpdateTeamRequest(ctx context.Context, requestID, captainID uuid.UUID, status string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathon_team_requests r SET status=$3
		FROM hackathon_teams t
		WHERE r.id=$1 AND r.team_id=t.id AND t.captain_id=$2
	`, requestID, captainID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	if status == "accepted" {
		var teamID, studentID uuid.UUID
		_ = r.pool.QueryRow(ctx, `SELECT team_id, student_id FROM hackathon_team_requests WHERE id=$1`, requestID).Scan(&teamID, &studentID)
		var hackID uuid.UUID
		_ = r.pool.QueryRow(ctx, `SELECT hackathon_id FROM hackathon_teams WHERE id=$1`, teamID).Scan(&hackID)
		_ = r.Register(ctx, hackID, studentID, &teamID)
	}
	return nil
}

func (r *HackathonRepository) ListCertificatesForStudent(ctx context.Context, studentID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.hackathon_id, c.certificate_url, h.theme
		FROM hackathon_certificates c
		JOIN hackathons h ON h.id = c.hackathon_id
		WHERE c.student_id=$1
	`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var hackID uuid.UUID
		var url, theme string
		if err := rows.Scan(&hackID, &url, &theme); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"hackathon_id": hackID.String(), "certificate_url": url, "theme": theme,
		})
	}
	return out, rows.Err()
}
