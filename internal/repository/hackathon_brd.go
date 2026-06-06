package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
)

type ListHackathonFilter struct {
	Theme        string
	Format       string
	PrizeType    string
	StartsAfter  *time.Time
	StartsBefore *time.Time
	Sort         string
	Limit        int
}

func (r *HackathonRepository) ListPublishedFiltered(ctx context.Context, f ListHackathonFilter) ([]model.Hackathon, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	order := "catalog_priority DESC, starts_at ASC"
	if f.Sort == "starts_at" {
		order = "starts_at ASC"
	}
	var b strings.Builder
	b.WriteString(`SELECT ` + hackathonCols + ` FROM hackathons WHERE listing_fee_paid = true AND status NOT IN ('draft','pending_review','rejected')`)
	args := []interface{}{}
	n := 1
	if f.Theme != "" {
		fmt.Fprintf(&b, " AND theme ILIKE $%d", n)
		args = append(args, "%"+f.Theme+"%")
		n++
	}
	if f.Format != "" {
		fmt.Fprintf(&b, " AND format = $%d", n)
		args = append(args, f.Format)
		n++
	}
	if f.PrizeType != "" {
		fmt.Fprintf(&b, " AND prize_type = $%d", n)
		args = append(args, f.PrizeType)
		n++
	}
	if f.StartsAfter != nil {
		fmt.Fprintf(&b, " AND starts_at >= $%d", n)
		args = append(args, *f.StartsAfter)
		n++
	}
	if f.StartsBefore != nil {
		fmt.Fprintf(&b, " AND starts_at <= $%d", n)
		args = append(args, *f.StartsBefore)
		n++
	}
	fmt.Fprintf(&b, " ORDER BY %s LIMIT $%d", order, n)
	args = append(args, limit)
	rows, err := r.pool.Query(ctx, b.String(), args...)
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

func (r *HackathonRepository) UpdateHackathon(ctx context.Context, id, organizerID uuid.UUID, in model.HackathonUpdateInput, titleEnc, descEnc, rulesEnc, taskEnc []byte, priority int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE hackathons SET
			title_enc=$3, description_enc=$4, rules_enc=$5, theme=$6, format=$7,
			prize_pool_kzt=$8, prize_type=$9, prize_breakdown=$10,
			min_participants=$11, max_participants=$12, team_min_size=$13, team_max_size=$14,
			starts_at=$15, ends_at=$16, registration_opens_at=$17, registration_deadline=$18,
			results_announced_at=$19, registration_mode=$20, task_reveal=$21, task_body_enc=$22,
			submission_schema=$23, blind_judging=$24, winner_mode=$25, public_submissions=$26,
			catalog_priority=$27, updated_at=NOW()
		WHERE id=$1 AND organizer_id=$2 AND status='draft'
	`, id, organizerID, titleEnc, descEnc, rulesEnc, in.Theme, in.Format,
		in.PrizePoolKZT, in.PrizeType, in.PrizeBreakdown, in.MinParticipants, in.MaxParticipants,
		in.TeamMinSize, in.TeamMaxSize, in.StartsAt, in.EndsAt, in.RegistrationOpensAt,
		in.RegistrationDeadline, in.ResultsAnnouncedAt, in.RegistrationMode, in.TaskReveal, taskEnc,
		in.SubmissionSchema, in.BlindJudging, in.WinnerMode, in.PublicSubmissions, priority)
	return err
}

func (r *HackathonRepository) PublishWithEscrow(ctx context.Context, id, organizerID uuid.UUID, priority int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathons SET listing_fee_paid=true, status='pending_review',
			catalog_priority=$3, prize_escrow_recorded=CASE WHEN prize_type='cash' THEN true ELSE prize_escrow_recorded END,
			updated_at=NOW()
		WHERE id=$1 AND organizer_id=$2`, id, organizerID, priority)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) OpenEvaluation(ctx context.Context, id, organizerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathons SET status='results_pending', updated_at=NOW()
		WHERE id=$1 AND organizer_id=$2 AND status IN ('evaluation','in_progress')`, id, organizerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) LockResults(ctx context.Context, id, organizerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathons SET results_locked=true, updated_at=NOW()
		WHERE id=$1 AND organizer_id=$2`, id, organizerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) ApprovedRegistrationCount(ctx context.Context, hackathonID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM hackathon_registrations WHERE hackathon_id=$1 AND status='approved'`, hackathonID).Scan(&n)
	return n, err
}

func (r *HackathonRepository) RegisterWithStatus(ctx context.Context, hackathonID, studentID uuid.UUID, teamID *uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO hackathon_registrations (hackathon_id, student_id, team_id, status)
		VALUES ($1,$2,$3,$4)`, hackathonID, studentID, teamID, status)
	return err
}

func (r *HackathonRepository) GetRegistration(ctx context.Context, hackathonID, studentID uuid.UUID) (*model.HackathonRegistration, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, hackathon_id, student_id, team_id, status, created_at
		FROM hackathon_registrations WHERE hackathon_id=$1 AND student_id=$2`, hackathonID, studentID)
	var reg model.HackathonRegistration
	err := row.Scan(&reg.ID, &reg.HackathonID, &reg.StudentID, &reg.TeamID, &reg.Status, &reg.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

func (r *HackathonRepository) ListRegistrations(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, student_id, team_id, status, created_at
		FROM hackathon_registrations WHERE hackathon_id=$1 ORDER BY created_at DESC`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, sid uuid.UUID
		var tid *uuid.UUID
		var status string
		var created time.Time
		if err := rows.Scan(&id, &sid, &tid, &status, &created); err != nil {
			return nil, err
		}
		m := map[string]interface{}{
			"id": id.String(), "student_id": sid.String(), "status": status,
			"created_at": created.Format(time.RFC3339),
		}
		if tid != nil {
			m["team_id"] = tid.String()
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *HackathonRepository) PatchRegistrationStatus(ctx context.Context, regID, organizerID uuid.UUID, status string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathon_registrations r SET status=$3
		FROM hackathons h WHERE r.id=$1 AND r.hackathon_id=h.id AND h.organizer_id=$2
	`, regID, organizerID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) ApprovedStudentIDs(ctx context.Context, hackathonID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT student_id FROM hackathon_registrations WHERE hackathon_id=$1 AND status='approved'`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *HackathonRepository) SubmissionCount(ctx context.Context, hackathonID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM hackathon_submissions WHERE hackathon_id=$1`, hackathonID).Scan(&n)
	return n, err
}

func (r *HackathonRepository) UpsertSubmission(ctx context.Context, hackathonID uuid.UUID, teamID, studentID *uuid.UUID, p model.SubmissionPayload) (*model.HackathonSubmission, error) {
	var subID uuid.UUID
	var versionNo int
	err := r.pool.QueryRow(ctx, `
		SELECT id, version_no FROM hackathon_submissions
		WHERE hackathon_id=$1 AND (($2::uuid IS NOT NULL AND team_id=$2) OR ($3::uuid IS NOT NULL AND student_id=$3))
	`, hackathonID, teamID, studentID).Scan(&subID, &versionNo)
	if err == pgx.ErrNoRows {
		versionNo = 1
		err = r.pool.QueryRow(ctx, `
			INSERT INTO hackathon_submissions (hackathon_id, team_id, student_id, description, artifact_key,
				presentation_key, repo_url, video_url, version_no)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1) RETURNING id
		`, hackathonID, teamID, studentID, p.Description, nullStr(p.ArtifactKey), p.PresentationKey, p.RepoURL, p.VideoURL).Scan(&subID)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		versionNo++
		_, err = r.pool.Exec(ctx, `
			UPDATE hackathon_submissions SET description=$2, artifact_key=$3, presentation_key=$4,
				repo_url=$5, video_url=$6, version_no=$7, submitted_at=NOW()
			WHERE id=$1`, subID, p.Description, nullStr(p.ArtifactKey), p.PresentationKey, p.RepoURL, p.VideoURL, versionNo)
		if err != nil {
			return nil, err
		}
	}
	_, _ = r.pool.Exec(ctx, `
		INSERT INTO hackathon_submission_versions (submission_id, version_no, description, artifact_key,
			presentation_key, repo_url, video_url)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (submission_id, version_no) DO NOTHING
	`, subID, versionNo, p.Description, p.ArtifactKey, p.PresentationKey, p.RepoURL, p.VideoURL)
	return r.GetSubmission(ctx, subID)
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (r *HackathonRepository) GetSubmission(ctx context.Context, id uuid.UUID) (*model.HackathonSubmission, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, hackathon_id, team_id, student_id, description, artifact_key,
			presentation_key, repo_url, video_url, version_no, submitted_at
		FROM hackathon_submissions WHERE id=$1`, id)
	var s model.HackathonSubmission
	err := row.Scan(&s.ID, &s.HackathonID, &s.TeamID, &s.StudentID, &s.Description, &s.ArtifactKey,
		&s.PresentationKey, &s.RepoURL, &s.VideoURL, &s.VersionNo, &s.SubmittedAt)
	return &s, err
}

func (r *HackathonRepository) ListSubmissions(ctx context.Context, hackathonID uuid.UUID, blind bool) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, student_id, description, artifact_key, presentation_key, repo_url, video_url,
			version_no, submitted_at FROM hackathon_submissions WHERE hackathon_id=$1 ORDER BY submitted_at DESC
	`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var teamID, studentID *uuid.UUID
		var desc, pres, repo, video string
		var artifact *string
		var version int
		var submitted time.Time
		if err := rows.Scan(&id, &teamID, &studentID, &desc, &artifact, &pres, &repo, &video, &version, &submitted); err != nil {
			return nil, err
		}
		m := map[string]interface{}{
			"id": id.String(), "description": desc, "version_no": version,
			"submitted_at": submitted.Format(time.RFC3339),
		}
		if !blind {
			if teamID != nil {
				m["team_id"] = teamID.String()
			}
			if studentID != nil {
				m["student_id"] = studentID.String()
			}
		} else {
			m["label"] = fmt.Sprintf("Project-%s", id.String()[:8])
		}
		if artifact != nil {
			m["artifact_key"] = *artifact
		}
		if pres != "" {
			m["presentation_key"] = pres
		}
		if repo != "" {
			m["repo_url"] = repo
		}
		if video != "" {
			m["video_url"] = video
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *HackathonRepository) GetMySubmission(ctx context.Context, hackathonID, studentID uuid.UUID) (*model.HackathonSubmission, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT s.id, s.hackathon_id, s.team_id, s.student_id, s.description, s.artifact_key,
			s.presentation_key, s.repo_url, s.video_url, s.version_no, s.submitted_at
		FROM hackathon_submissions s
		LEFT JOIN hackathon_registrations r ON r.hackathon_id=s.hackathon_id AND r.student_id=$2
		WHERE s.hackathon_id=$1 AND (s.student_id=$2 OR s.team_id=r.team_id)
		ORDER BY s.submitted_at DESC LIMIT 1
	`, hackathonID, studentID)
	var s model.HackathonSubmission
	err := row.Scan(&s.ID, &s.HackathonID, &s.TeamID, &s.StudentID, &s.Description, &s.ArtifactKey,
		&s.PresentationKey, &s.RepoURL, &s.VideoURL, &s.VersionNo, &s.SubmittedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *HackathonRepository) ListSubmissionVersions(ctx context.Context, submissionID uuid.UUID) ([]model.HackathonSubmissionVersion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, submission_id, version_no, description, artifact_key, presentation_key, repo_url, video_url, created_at
		FROM hackathon_submission_versions WHERE submission_id=$1 ORDER BY version_no DESC`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.HackathonSubmissionVersion
	for rows.Next() {
		var v model.HackathonSubmissionVersion
		if err := rows.Scan(&v.ID, &v.SubmissionID, &v.VersionNo, &v.Description, &v.ArtifactKey,
			&v.PresentationKey, &v.RepoURL, &v.VideoURL, &v.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *HackathonRepository) ComputeRanking(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		WITH crit AS (
			SELECT id, weight_percent FROM hackathon_criteria WHERE hackathon_id=$1
		),
		jury_scores AS (
			SELECT s.submission_id,
				AVG(sc.score * c.weight_percent::float / 100.0) AS weighted
			FROM hackathon_scores sc
			JOIN crit c ON c.id = sc.criterion_id
			JOIN hackathon_submissions s ON s.id = sc.submission_id
			WHERE sc.hackathon_id=$1
			GROUP BY s.submission_id, sc.jury_member_id
		),
		agg AS (
			SELECT submission_id, AVG(weighted) AS total_score
			FROM jury_scores GROUP BY submission_id
		)
		SELECT a.submission_id, sub.team_id, a.total_score
		FROM agg a
		JOIN hackathon_submissions sub ON sub.id = a.submission_id
		ORDER BY a.total_score DESC NULLS LAST
	`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	rank := 1
	for rows.Next() {
		var subID uuid.UUID
		var teamID *uuid.UUID
		var score float64
		if err := rows.Scan(&subID, &teamID, &score); err != nil {
			return nil, err
		}
		m := map[string]interface{}{
			"rank": rank, "submission_id": subID.String(), "total_score": score,
		}
		if teamID != nil {
			m["team_id"] = teamID.String()
		}
		out = append(out, m)
		rank++
	}
	return out, rows.Err()
}

func (r *HackathonRepository) UpsertJuryScore(ctx context.Context, hackathonID, submissionID, criterionID uuid.UUID, juryMemberID *uuid.UUID, score float64, comment string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM hackathon_scores WHERE submission_id=$1 AND criterion_id=$2
		  AND COALESCE(jury_member_id, '00000000-0000-0000-0000-000000000000'::uuid) =
		      COALESCE($3, '00000000-0000-0000-0000-000000000000'::uuid)
	`, submissionID, criterionID, juryMemberID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO hackathon_scores (hackathon_id, submission_id, criterion_id, jury_member_id, score, comment)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, hackathonID, submissionID, criterionID, juryMemberID, score, comment)
	return err
}

func (r *HackathonRepository) IsJuryMember(ctx context.Context, hackathonID, userID uuid.UUID) (*uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM hackathon_jury_members WHERE hackathon_id=$1 AND user_id=$2`, hackathonID, userID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *HackathonRepository) CreateJuryMember(ctx context.Context, hackathonID uuid.UUID, name, title string, userID *uuid.UUID, sort int) (map[string]interface{}, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO hackathon_jury_members (hackathon_id, display_name, title, user_id, sort_order)
		VALUES ($1,$2,$3,$4,$5) RETURNING id
	`, hackathonID, name, title, userID, sort).Scan(&id)
	if err != nil {
		return nil, err
	}
	m := map[string]interface{}{"id": id.String(), "display_name": name, "title": title, "sort_order": sort}
	if userID != nil {
		m["user_id"] = userID.String()
	}
	return m, nil
}

func (r *HackathonRepository) ListJuryMembers(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, display_name, title, user_id, sort_order FROM hackathon_jury_members
		WHERE hackathon_id=$1 ORDER BY sort_order ASC`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var name, title string
		var uid *uuid.UUID
		var sort int
		if err := rows.Scan(&id, &name, &title, &uid, &sort); err != nil {
			return nil, err
		}
		m := map[string]interface{}{"id": id.String(), "display_name": name, "title": title, "sort_order": sort}
		if uid != nil {
			m["user_id"] = uid.String()
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *HackathonRepository) DeleteJuryMember(ctx context.Context, juryID, organizerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM hackathon_jury_members j USING hackathons h
		WHERE j.id=$1 AND j.hackathon_id=h.id AND h.organizer_id=$2`, juryID, organizerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) CreateMaterial(ctx context.Context, hackathonID uuid.UUID, title, key string, sort int) (map[string]interface{}, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO hackathon_materials (hackathon_id, title, storage_key, sort_order)
		VALUES ($1,$2,$3,$4) RETURNING id
	`, hackathonID, title, key, sort).Scan(&id)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"id": id.String(), "title": title, "storage_key": key, "sort_order": sort}, nil
}

func (r *HackathonRepository) ListMaterials(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, storage_key, sort_order FROM hackathon_materials
		WHERE hackathon_id=$1 ORDER BY sort_order ASC`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id uuid.UUID
		var title, key string
		var sort int
		if err := rows.Scan(&id, &title, &key, &sort); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id.String(), "title": title, "storage_key": key, "sort_order": sort,
		})
	}
	return out, rows.Err()
}

func (r *HackathonRepository) DeleteMaterial(ctx context.Context, materialID, organizerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM hackathon_materials m USING hackathons h
		WHERE m.id=$1 AND m.hackathon_id=h.id AND h.organizer_id=$2`, materialID, organizerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) ListTeams(ctx context.Context, hackathonID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.name, t.captain_id, t.invite_code, t.recruiting,
			(SELECT COUNT(*) FROM hackathon_registrations r WHERE r.team_id=t.id AND r.status='approved') AS members
		FROM hackathon_teams t WHERE t.hackathon_id=$1 ORDER BY t.created_at DESC`, hackathonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]interface{}
	for rows.Next() {
		var id, captain uuid.UUID
		var name, code string
		var recruiting bool
		var members int
		if err := rows.Scan(&id, &name, &captain, &code, &recruiting, &members); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"id": id.String(), "name": name, "captain_id": captain.String(),
			"invite_code": code, "recruiting": recruiting, "member_count": members,
		})
	}
	return out, rows.Err()
}

func (r *HackathonRepository) TeamMemberCount(ctx context.Context, teamID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM hackathon_registrations WHERE team_id=$1 AND status='approved'`, teamID).Scan(&n)
	return n, err
}

func (r *HackathonRepository) TeamMemberIDs(ctx context.Context, teamID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT student_id FROM hackathon_registrations WHERE team_id=$1 AND status='approved'`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *HackathonRepository) LeaveTeam(ctx context.Context, hackathonID, studentID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE hackathon_registrations SET team_id=NULL WHERE hackathon_id=$1 AND student_id=$2`, hackathonID, studentID)
	return err
}

func (r *HackathonRepository) TransferCaptain(ctx context.Context, teamID, captainID, newCaptainID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathon_teams SET captain_id=$3
		WHERE id=$1 AND captain_id=$2`, teamID, captainID, newCaptainID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) RemoveTeamMember(ctx context.Context, teamID, captainID, studentID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE hackathon_registrations r SET team_id=NULL
		FROM hackathon_teams t
		WHERE r.team_id=$1 AND r.student_id=$3 AND t.id=$1 AND t.captain_id=$2
	`, teamID, captainID, studentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *HackathonRepository) PublishResultsFinal(ctx context.Context, hackathonID uuid.UUID, results []model.HackathonResult, certURLs map[uuid.UUID]string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, _ = tx.Exec(ctx, `DELETE FROM hackathon_results WHERE hackathon_id=$1`, hackathonID)
	for _, res := range results {
		_, err = tx.Exec(ctx, `
			INSERT INTO hackathon_results (hackathon_id, team_id, student_id, place, prize_amount_kzt, internship_offer)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			hackathonID, res.TeamID, res.StudentID, res.Place, res.PrizeAmountKZT, res.InternshipOffer)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE hackathons SET status='completed', results_locked=true, updated_at=NOW() WHERE id=$1`, hackathonID)
	if err != nil {
		return err
	}
	rows, _ := tx.Query(ctx, `
		SELECT student_id FROM hackathon_registrations WHERE hackathon_id=$1 AND status='approved'`, hackathonID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var sid uuid.UUID
			if err := rows.Scan(&sid); err != nil {
				continue
			}
			url := certURLs[sid]
			if url == "" {
				url = "demo://certificate/participant/" + hackathonID.String() + "/" + sid.String()
			}
			_, _ = tx.Exec(ctx, `
				INSERT INTO hackathon_certificates (hackathon_id, student_id, certificate_url, cert_type, place)
				VALUES ($1,$2,$3,'participant',0)
				ON CONFLICT (hackathon_id, student_id) DO UPDATE SET certificate_url=EXCLUDED.certificate_url
			`, hackathonID, sid, url)
		}
	}
	for _, res := range results {
		if res.StudentID == nil {
			continue
		}
		url := certURLs[*res.StudentID]
		if url == "" {
			url = fmt.Sprintf("demo://certificate/winner/%s/%s", hackathonID, res.StudentID)
		}
		_, _ = tx.Exec(ctx, `
			INSERT INTO hackathon_certificates (hackathon_id, student_id, certificate_url, cert_type, place)
			VALUES ($1,$2,$3,'winner',$4)
			ON CONFLICT (hackathon_id, student_id) DO UPDATE SET certificate_url=EXCLUDED.certificate_url, cert_type='winner', place=$4
		`, hackathonID, res.StudentID, url, res.Place)
	}
	return tx.Commit(ctx)
}

func (r *HackathonRepository) OrganizerStats(ctx context.Context, hackathonID uuid.UUID) (map[string]interface{}, error) {
	reg, _ := r.ApprovedRegistrationCount(ctx, hackathonID)
	subs, _ := r.SubmissionCount(ctx, hackathonID)
	var avgScore float64
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(score),0) FROM hackathon_scores WHERE hackathon_id=$1`, hackathonID).Scan(&avgScore)
	return map[string]interface{}{
		"approved_registrations": reg,
		"submissions_count":      subs,
		"avg_score":              avgScore,
	}, nil
}

func (r *HackathonRepository) CriteriaWeightSum(ctx context.Context, hackathonID uuid.UUID) (int, error) {
	var sum int
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(weight_percent),0) FROM hackathon_criteria WHERE hackathon_id=$1`, hackathonID).Scan(&sum)
	return sum, err
}

func (r *HackathonRepository) ListCertificatesForStudent(ctx context.Context, studentID uuid.UUID) ([]map[string]interface{}, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.hackathon_id, c.certificate_url, c.cert_type, c.place, h.theme
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
		var url, theme, certType string
		var place int
		if err := rows.Scan(&hackID, &url, &certType, &place, &theme); err != nil {
			return nil, err
		}
		out = append(out, map[string]interface{}{
			"hackathon_id": hackID.String(), "certificate_url": url, "theme": theme,
			"cert_type": certType, "place": place,
		})
	}
	return out, rows.Err()
}

func (r *HackathonRepository) UpdateTeamRequestWithLimits(ctx context.Context, requestID, captainID uuid.UUID, status string, maxSize int) error {
	if status != "accepted" {
		return r.UpdateTeamRequest(ctx, requestID, captainID, status)
	}
	var teamID uuid.UUID
	if err := r.pool.QueryRow(ctx, `SELECT team_id FROM hackathon_team_requests WHERE id=$1`, requestID).Scan(&teamID); err != nil {
		return err
	}
	n, _ := r.TeamMemberCount(ctx, teamID)
	if n >= maxSize {
		return fmt.Errorf("team is full")
	}
	return r.UpdateTeamRequest(ctx, requestID, captainID, status)
}
