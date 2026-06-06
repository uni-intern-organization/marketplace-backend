package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uni-intern-organization/marketplace-backend/internal/notifier"
)

// StartScheduler runs periodic background jobs: vacancy expiry warnings, moderation SLA,
// hackathon status transitions, and weekly digest placeholder.
func StartScheduler(ctx context.Context, pool *pgxpool.Pool, n *notifier.Service) {
	go runTicker(ctx, 30*time.Minute, func() {
		vacancyExpiryWarnings(ctx, pool, n)
		moderationSLA(ctx, pool, n)
		hackathonStatusTransitions(ctx, pool)
		bannerMaintenance(ctx, pool, n)
		interviewReminders(ctx, pool, n)
	})
	go runTicker(ctx, 24*time.Hour, func() {
		weeklyDigestPlaceholder(ctx, pool, n)
	})
	log.Println("jobs: scheduler started")
}

func interviewReminders(ctx context.Context, pool *pgxpool.Pool, n *notifier.Service) {
	rows, err := pool.Query(ctx, `
		SELECT id, student_id, recruiter_id, interview_scheduled_at
		FROM applications
		WHERE status = 'interview_scheduled'
		  AND interview_reminder_sent = false
		  AND interview_scheduled_at > NOW()
		  AND interview_scheduled_at <= NOW() + interval '24 hours'
	`)
	if err != nil {
		log.Printf("scheduler interview reminders: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var appID, studentID, recruiterID uuid.UUID
		var scheduled time.Time
		if err := rows.Scan(&appID, &studentID, &recruiterID, &scheduled); err != nil {
			continue
		}
		body := fmt.Sprintf("Собеседование назначено на %s", scheduled.Format("02.01.2006 15:04"))
		if n != nil {
			payload := map[string]interface{}{"application_id": appID.String(), "scheduled_at": scheduled.Format(time.RFC3339)}
			n.Notify(ctx, studentID, "interview_reminder", "Собеседование уже скоро", body, payload)
			n.Notify(ctx, recruiterID, "interview_reminder", "Собеседование уже скоро", body, payload)
		}
		_, _ = pool.Exec(ctx, `UPDATE applications SET interview_reminder_sent=true WHERE id=$1`, appID)
	}
}

func runTicker(ctx context.Context, interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	fn()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fn()
		}
	}
}

func vacancyExpiryWarnings(ctx context.Context, pool *pgxpool.Pool, n *notifier.Service) {
	for _, days := range []int{7, 3} {
		rows, err := pool.Query(ctx, `
			SELECT id, recruiter_id
			FROM vacancies
			WHERE status = 'active' AND expires_at IS NOT NULL
			  AND expires_at > NOW()
			  AND expires_at <= NOW() + ($1::int * interval '1 day')
		`, days)
		if err != nil {
			log.Printf("scheduler vacancy expiry %dd: %v", days, err)
			continue
		}
		for rows.Next() {
			var vacID, recruiterID uuid.UUID
			if err := rows.Scan(&vacID, &recruiterID); err != nil {
				continue
			}
			if n != nil {
				title := fmt.Sprintf("Объявление истекает через %d дн.", days)
				body := fmt.Sprintf("Вакансия %s будет архивирована %d дней.", vacID.String(), days)
				n.Notify(ctx, recruiterID, "vacancy_expiry", title, body, map[string]interface{}{
					"vacancy_id": vacID.String(), "days_left": days,
				})
			}
		}
		rows.Close()
	}
}

func moderationSLA(ctx context.Context, pool *pgxpool.Pool, n *notifier.Service) {
	rows, err := pool.Query(ctx, `
		SELECT id, recruiter_id
		FROM vacancies
		WHERE status = 'pending_review'
		  AND COALESCE(moderation_submitted_at, created_at) < NOW() - interval '4 hours'
	`)
	if err != nil {
		log.Printf("scheduler moderation SLA: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var vacID, recruiterID uuid.UUID
		if err := rows.Scan(&vacID, &recruiterID); err != nil {
			continue
		}
		if n != nil {
			n.Notify(ctx, recruiterID, "moderation_sla", "Модерация задерживается", "Ваше объявление ожидает проверки более 4 часов", map[string]interface{}{
				"vacancy_id": vacID.String(),
			})
		}
	}
}

func hackathonStatusTransitions(ctx context.Context, pool *pgxpool.Pool) {
	_, err := pool.Exec(ctx, `
		UPDATE hackathons SET status = 'registration_open', updated_at = NOW()
		WHERE status IN ('published','registration_open') AND registration_deadline > NOW() AND starts_at > NOW()
	`)
	if err != nil {
		log.Printf("scheduler hackathon reg open: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE hackathons SET status = 'registration_closed', updated_at = NOW()
		WHERE status IN ('registration_open','published') AND registration_deadline <= NOW() AND starts_at > NOW()
	`)
	if err != nil {
		log.Printf("scheduler hackathon reg closed: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE hackathons SET status = 'in_progress', updated_at = NOW()
		WHERE status IN ('registration_closed','registration_open') AND starts_at <= NOW() AND ends_at > NOW()
	`)
	if err != nil {
		log.Printf("scheduler hackathon in progress: %v", err)
	}
	_, err = pool.Exec(ctx, `
		UPDATE hackathons SET status = 'evaluation', updated_at = NOW()
		WHERE status = 'in_progress' AND ends_at <= NOW()
	`)
	if err != nil {
		log.Printf("scheduler hackathon evaluation: %v", err)
	}
}

func weeklyDigestPlaceholder(ctx context.Context, pool *pgxpool.Pool, n *notifier.Service) {
	rows, err := pool.Query(ctx, `
		SELECT id FROM users WHERE role = 'student' AND COALESCE(is_blocked, false) = false LIMIT 500
	`)
	if err != nil {
		log.Printf("scheduler weekly digest: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			continue
		}
		if n != nil {
			n.Notify(ctx, userID, "weekly_digest", "Еженедельная подборка", "Новые вакансии и задачи на этой неделе (placeholder)", nil)
		}
	}
	log.Println("scheduler: weekly digest placeholder sent")
}

func bannerMaintenance(ctx context.Context, pool *pgxpool.Pool, n *notifier.Service) {
	tag, err := pool.Exec(ctx, `
		UPDATE banner_campaigns SET status='completed', updated_at=NOW()
		WHERE status='active' AND ends_at <= NOW()
	`)
	if err != nil {
		log.Printf("scheduler banner expire: %v", err)
	} else if tag.RowsAffected() > 0 {
		log.Printf("scheduler: expired %d banner campaigns", tag.RowsAffected())
	}

	rows, err := pool.Query(ctx, `
		SELECT id, recruiter_id FROM banner_campaigns
		WHERE status='active' AND expiring_notified=false AND recruiter_id IS NOT NULL
		  AND ends_at > NOW() AND ends_at <= NOW() + interval '3 days'
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, recruiterID uuid.UUID
			if err := rows.Scan(&id, &recruiterID); err != nil {
				continue
			}
			if n != nil {
				n.Notify(ctx, recruiterID, "banner_expiring", "Баннер скоро завершится",
					"Продлите размещение в разделе «Продвижение»", map[string]interface{}{"campaign_id": id.String()})
			}
			_, _ = pool.Exec(ctx, `UPDATE banner_campaigns SET expiring_notified=true WHERE id=$1`, id)
		}
	}

	slaRows, err := pool.Query(ctx, `
		SELECT bc.id, u.id FROM banner_campaigns bc
		CROSS JOIN users u
		WHERE bc.status='pending_review' AND bc.created_at <= NOW() - interval '24 hours'
		  AND u.role='admin' LIMIT 5
	`)
	if err == nil {
		defer slaRows.Close()
		for slaRows.Next() {
			var campaignID, adminID uuid.UUID
			if err := slaRows.Scan(&campaignID, &adminID); err != nil {
				continue
			}
			if n != nil {
				n.Notify(ctx, adminID, "banner_moderation_sla", "Баннер ожидает проверки",
					"Заявка на баннер ожидает решения более 24 часов", map[string]interface{}{"campaign_id": campaignID.String()})
			}
		}
	}
}
