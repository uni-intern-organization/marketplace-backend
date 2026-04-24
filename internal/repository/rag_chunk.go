package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RagChunkRow is one indexed fragment with its embedding.
type RagChunkRow struct {
	VacancyID uuid.UUID
	Content   string
	Embedding []float64
}

// RagChunkRepository stores vacancy text chunks and vectors for RAG.
type RagChunkRepository struct {
	pool *pgxpool.Pool
}

func NewRagChunkRepository(pool *pgxpool.Pool) *RagChunkRepository {
	return &RagChunkRepository{pool: pool}
}

// ReplaceVacancyChunks deletes old rows and inserts new ones in a transaction.
func (r *RagChunkRepository) ReplaceVacancyChunks(ctx context.Context, vacancyID uuid.UUID, chunks []RagChunkRow) error {
	if len(chunks) == 0 {
		_, err := r.pool.Exec(ctx, `DELETE FROM vacancy_rag_chunks WHERE vacancy_id = $1`, vacancyID)
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM vacancy_rag_chunks WHERE vacancy_id = $1`, vacancyID); err != nil {
		return err
	}
	for i, c := range chunks {
		if len(c.Embedding) == 0 {
			return fmt.Errorf("empty embedding at chunk %d", i)
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO vacancy_rag_chunks (vacancy_id, chunk_index, content, embedding)
			VALUES ($1, $2, $3, $4::float8[])
		`, vacancyID, i, c.Content, c.Embedding)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ListAllEmbeddingRows loads all chunk rows (used for in-DB similarity over moderate corpora).
func (r *RagChunkRepository) ListAllEmbeddingRows(ctx context.Context, maxRows int) ([]RagChunkRow, error) {
	if maxRows <= 0 {
		maxRows = 20000
	}
	rows, err := r.pool.Query(ctx, `
		SELECT vacancy_id, content, embedding
		FROM vacancy_rag_chunks
		LIMIT $1
	`, maxRows)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RagChunkRow
	for rows.Next() {
		var row RagChunkRow
		if err := rows.Scan(&row.VacancyID, &row.Content, &row.Embedding); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *RagChunkRepository) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vacancy_rag_chunks`).Scan(&n)
	return n, err
}
