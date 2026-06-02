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
	})
	go runTicker(ctx, 24*time.Hour, func() {
		weeklyDigestPlaceholder(ctx, pool, n)
	})
	log.Println("jobs: scheduler started")
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
