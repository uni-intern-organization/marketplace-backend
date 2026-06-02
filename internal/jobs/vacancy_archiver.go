package jobs

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// StartVacancyArchiver periodically archives expired vacancies.
func StartVacancyArchiver(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		archiveOnce(ctx, pool)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				archiveOnce(ctx, pool)
			}
		}
	}()
}

func archiveOnce(ctx context.Context, pool *pgxpool.Pool) {
	tag, err := pool.Exec(ctx, `
		UPDATE vacancies
		SET status = 'archived', updated_at = NOW()
		WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at < NOW()
	`)
	if err != nil {
		log.Printf("vacancy archiver: %v", err)
		return
	}
	if tag.RowsAffected() > 0 {
		log.Printf("vacancy archiver: archived %d vacancies", tag.RowsAffected())
	}
}
