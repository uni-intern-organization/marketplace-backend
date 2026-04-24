package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/ai"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

// TopChunk keeps one retrieved fragment with a similarity score in [0,1].
type TopChunk struct {
	VacancyID uuid.UUID `json:"vacancy_id"`
	Score     float64   `json:"score"`
	Content   string    `json:"content"`
}

// AnswerResult is the RAG output before mapping to public JSON in the handler.
type AnswerResult struct {
	Narrative  string
	VacancyIDs []uuid.UUID
	Chunks     []TopChunk
}

// Engine runs embedding retrieval + one LLM call to summarize.
type Engine struct {
	Chunks     *repository.RagChunkRepository
	Client     *ai.Client
	EmbedModel string
}

// Answer executes vector search on stored chunks, then asks the model to return narrative + allowed vacancy ids.
func (e *Engine) Answer(ctx context.Context, resumeText string) (*AnswerResult, error) {
	if e == nil || e.Chunks == nil || e.Client == nil || strings.TrimSpace(e.Client.APIKey) == "" {
		return nil, fmt.Errorf("rag: not configured")
	}
	t := strings.TrimSpace(resumeText)
	if len(t) < 15 {
		return nil, fmt.Errorf("text too short")
	}
	if len(t) > 32000 {
		t = t[:32000]
	}
	qv, err := e.Client.EmbedTexts(ctx, e.EmbedModel, []string{t})
	if err != nil {
		return nil, err
	}
	if len(qv) == 0 || len(qv[0]) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}
	query := qv[0]

	rows, err := e.Chunks.ListAllEmbeddingRows(ctx, 20000)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return &AnswerResult{
			Narrative:  "Семантический индекс пуст. После старта сервера с OPENAI_API_KEY индексация может идти в фоне (подождите 1–2 минуты). Рекрутёр: POST /api/recruiter/rag/reindex-mine, админ: POST /api/admin/rag/reindex.",
			VacancyIDs: nil,
			Chunks:     nil,
		}, nil
	}

	type scored struct {
		row repository.RagChunkRow
		sim float64
	}
	var hits []scored
	for _, row := range rows {
		if len(row.Embedding) != len(query) {
			continue
		}
		sim := ai.CosineSimilarity(query, row.Embedding)
		if sim < 0.05 {
			continue
		}
		hits = append(hits, scored{row: row, sim: sim})
	}
	if len(hits) == 0 {
		return &AnswerResult{
			Narrative:  "Среди вакансий в индексе нет близких по смыслу к этому тексту. Попробуйте сформулировать опыт и навыки подробнее.",
			VacancyIDs: nil,
			Chunks:     nil,
		}, nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].sim > hits[j].sim })
	if len(hits) > 20 {
		hits = hits[:20]
	}

	// best score per vacancy
	seen := make(map[uuid.UUID]TopChunk)
	for _, h := range hits {
		prev, ok := seen[h.row.VacancyID]
		if !ok || h.sim > prev.Score {
			seen[h.row.VacancyID] = TopChunk{
				VacancyID: h.row.VacancyID,
				Score:     h.sim,
				Content:   h.row.Content,
			}
		}
	}
	var tops []TopChunk
	for _, v := range seen {
		tops = append(tops, v)
	}
	sort.Slice(tops, func(i, j int) bool { return tops[i].Score > tops[j].Score })
	if len(tops) > 5 {
		tops = tops[:5]
	}

	allowed := make(map[string]struct{})
	var b strings.Builder
	b.WriteString("The following are retrieved excerpts (vacancy_id is authoritative). Do not invent ids.\n\n")
	for _, tch := range tops {
		allowed[tch.VacancyID.String()] = struct{}{}
		b.WriteString(fmt.Sprintf("---\nvacancy_id: %s\nscore: %.4f\nexcerpt: %s\n", tch.VacancyID.String(), tch.Score, tch.Content))
	}
	system := `You help match resumes to real job postings. Output ONLY a JSON object with:
{"narrative":"2-4 sentences, same language as the resume, explain fit and gaps","vacancy_ids":["<uuid from context only>",...]}
Max 5 vacancy_ids, most relevant first. If nothing fits, use empty array and say so in narrative.`

	raw, err := e.Client.ChatJSONObject(ctx, system, "Resume text:\n"+t+"\n\nContext:\n"+b.String())
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Narrative  string   `json:"narrative"`
		VacancyIDs []string `json:"vacancy_ids"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("model json: %w", err)
	}
	var ids []uuid.UUID
	for _, s := range parsed.VacancyIDs {
		if _, ok := allowed[strings.TrimSpace(s)]; !ok {
			continue
		}
		parsedID, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		ids = append(ids, parsedID)
	}
	if len(ids) > 5 {
		ids = ids[:5]
	}
	return &AnswerResult{
		Narrative:  strings.TrimSpace(parsed.Narrative),
		VacancyIDs: ids,
		Chunks:     tops,
	}, nil
}
