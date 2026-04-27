package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/ai"
	"github.com/uni-intern-organization/marketplace-backend/internal/careerintel"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

const (
	ragHitScanLimit      = 55
	ragMaxDedupVacancies = 22
	ragGoodFitForPrompt  = 6
	ragMaxIDsOut         = 5
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
	Picks      []VacancyPick
}

// Engine runs embedding retrieval + one LLM call to summarize.
type Engine struct {
	Chunks     *repository.RagChunkRepository
	Vacancies  *repository.VacancyRepository // optional: hybrid rank + richer prompts
	AESKey     []byte                        // decrypt vacancy titles for ranking/prompt
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
			Picks:      nil,
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
			Picks:      nil,
		}, nil
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].sim > hits[j].sim })
	if len(hits) > ragHitScanLimit {
		hits = hits[:ragHitScanLimit]
	}

	// best cosine score per vacancy (semantic recall)
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
	if len(tops) > ragMaxDedupVacancies {
		tops = tops[:ragMaxDedupVacancies]
	}

	var ranked []RankedVacancy
	if e.Vacancies != nil {
		ranked = EnrichAndRank(ctx, e.Vacancies, e.AESKey, t, tops)
	}
	if len(ranked) == 0 {
		ranked = RankSemanticOnly(tops)
	}
	if len(ranked) == 0 {
		return &AnswerResult{
			Narrative:  "Не удалось построить список вакансий для ответа.",
			VacancyIDs: nil,
			Chunks:     nil,
			Picks:      nil,
		}, nil
	}

	nGood := ragGoodFitForPrompt
	if nGood > len(ranked) {
		nGood = len(ranked)
	}
	good := ranked[:nGood]

	var contrast *RankedVacancy
	if e.Vacancies != nil && len(ranked) > 0 {
		contrast = PickContrast(ranked)
		ex := make(map[uuid.UUID]struct{})
		for _, rv := range good {
			ex[rv.VacancyID] = struct{}{}
		}
		if contrast != nil {
			if _, dup := ex[contrast.VacancyID]; dup {
				contrast = nil
			}
		}
		if contrast == nil {
			contrast = WeakSemanticCandidate(ranked, ex)
			if contrast != nil {
				if _, dup := ex[contrast.VacancyID]; dup {
					contrast = nil
				}
			}
		}
	}

	goodSet := make(map[uuid.UUID]struct{}, len(good))
	var pb strings.Builder
	pb.WriteString("Resume text:\n")
	pb.WriteString(t)
	pb.WriteString("\n\n=== Vacancies for GOOD FIT (trust match_percent — server-computed hybrid: semantics + skill overlap) ===\n")
	for i := range good {
		rv := &good[i]
		goodSet[rv.VacancyID] = struct{}{}
		kwPct := int(rv.Keyword * 100)
		if kwPct > 100 {
			kwPct = 100
		}
		pb.WriteString(fmt.Sprintf("%d) vacancy_id=%s\n   title=%s\n   company=%s\n   employment=%s | location=%s\n   match_percent=%d | semantic_cosine≈%.3f | keyword_cover≈%d%%\n   required_skills=%s\n   chunk_excerpt: %s\n\n",
			i+1, rv.VacancyID.String(), rv.Title, rv.Company, rv.EmploymentType, rv.Location,
			rv.MatchPct, rv.Semantic, kwPct, rv.RequiredSkills, trimExcerpt(rv.ChunkExcerpt, 900)))
	}
	if contrast != nil {
		kwPct := int(contrast.Keyword * 100)
		pb.WriteString("=== CONTRAST ROW (often misleading semantic match — explain mismatch, e.g. frontend vs backend) ===\n")
		pb.WriteString(fmt.Sprintf("vacancy_id=%s\n   title=%s\n   company=%s\n   employment=%s | location=%s\n   match_percent=%d keyword_cover≈%d%%\n   required_skills=%s\n   chunk_excerpt: %s\n\n",
			contrast.VacancyID.String(), contrast.Title, contrast.Company, contrast.EmploymentType, contrast.Location,
			contrast.MatchPct, kwPct, contrast.RequiredSkills, trimExcerpt(contrast.ChunkExcerpt, 700)))
	}

	system := `You are a smart, helpful job search assistant integrated into a recruitment platform.
You receive structured vacancy data from retrieval; your job is to present it clearly, naturally, and in a human-like way.

You MUST output ONLY valid JSON:
{"narrative":"<plain text string>","vacancy_ids":["uuid",...]}

LANGUAGE:
- Write narrative in the SAME language as the resume text (Russian ↔ Russian, Kazakh ↔ Kazakh, English ↔ English).
- Never mix languages in one narrative.

THINKING BEFORE WRITING:
- Judge which vacancies actually fit the resume; match_percent and keyword_cover from CONTEXT are authoritative.
- Prefer quality over quantity; skip weak rows or describe them briefly as secondary with honest reasoning.

RESPONSE STYLE FOR narrative:
- Sound like a capable career advisor speaking to one person: direct, warm, honest.
- No filler openers like praising the question or announcing a list.
- Plain text only in narrative: no Markdown, no bold or headings, no numbered lists like 1) 2) 3), no bullet markdown.
- Separate vacancy blocks with blank lines only.
- Do not put vacancy_id, UUIDs, URL paths, or technical slugs inside narrative.
- Do not invite clicks or mention links in narrative.

STRUCTURE EACH VACANCY BLOCK up to four short sentences:
First: job title at company and location or employment format when relevant.
Second: which skills from the resume align with required_skills and why that matters for this role.
Third: honest note — what is interesting about the role or what to watch for; if match is weak, say so plainly.
Optional fourth: one concrete action tip if it helps.

CONTRAST ROW:
If CONTRAST ROW exists in CONTEXT, you may add one short paragraph explaining why it is weaker or misleading versus the resume — without copying ids into narrative.

vacancy_ids: UUID strings for the BEST openings only from GOOD FIT, descending match_percent, maximum %d. Omit poor-fit contrast ids.

If GOOD FIT is empty or useless, say so briefly in narrative and use vacancy_ids [].`

	system = fmt.Sprintf(system, ragMaxIDsOut)

	raw, err := e.Client.ChatJSONObject(ctx, system, pb.String())
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
	orderIdx := make(map[uuid.UUID]int, len(ranked))
	for i := range ranked {
		orderIdx[ranked[i].VacancyID] = i
	}
	var preferred []uuid.UUID
	for i := range ranked {
		if len(preferred) >= ragMaxIDsOut {
			break
		}
		preferred = append(preferred, ranked[i].VacancyID)
	}

	var ids []uuid.UUID
	for _, s := range parsed.VacancyIDs {
		parsedID, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		if _, ok := goodSet[parsedID]; !ok {
			continue
		}
		if contrast != nil && parsedID == contrast.VacancyID {
			continue
		}
		ids = append(ids, parsedID)
	}
	if len(ids) > ragMaxIDsOut {
		ids = ids[:ragMaxIDsOut]
	}
	sort.Slice(ids, func(i, j int) bool { return orderIdx[ids[i]] < orderIdx[ids[j]] })
	if len(ids) == 0 && len(preferred) > 0 {
		ids = preferred
	}

	chunksOut := make([]TopChunk, 0, min(8, len(ranked)))
	picks := make([]VacancyPick, 0, min(10, len(ranked)))
	for i := range ranked {
		if len(chunksOut) >= 8 {
			break
		}
		rv := ranked[i]
		chunksOut = append(chunksOut, TopChunk{
			VacancyID: rv.VacancyID,
			Score:     rv.Combined,
			Content:   rv.ChunkExcerpt,
		})
	}
	resumeHay := strings.ToLower(t)
	for i := range ranked {
		if len(picks) >= 5 {
			break
		}
		rv := ranked[i]
		if rv.MatchPct < ragPickMinPct {
			continue
		}
		matched, missing := careerintel.SkillGap(rv.RequiredSkills, resumeHay)
		learn := careerintel.LearnNext(missing, 8)
		picks = append(picks, VacancyPick{
			VacancyID:       rv.VacancyID.String(),
			Title:           rv.Title,
			CompanyName:     rv.Company,
			EmploymentType:  rv.EmploymentType,
			Location:        rv.Location,
			MatchPercent:    rv.MatchPct,
			MatchScore100:   rv.MatchPct,
			CombinedScore:   rv.Combined,
			MatchedSkills:   matched,
			MissingSkills:   missing,
			LearnNext:       learn,
			FitSummary:      VacancyPickFitSummary(rv.Keyword, rv.MatchPct),
			OpenPath:        "/jobs/" + rv.VacancyID.String(),
		})
	}

	narr := strings.TrimSpace(parsed.Narrative)
	if narr == "" && len(ranked) > 0 {
		narr = "Не удалось сформулировать текст ответа модели. Смотрите retrieval_top и vacancy_ids."
	}

	return &AnswerResult{
		Narrative:  narr,
		VacancyIDs: ids,
		Chunks:     chunksOut,
		Picks:      picks,
	}, nil
}

func trimExcerpt(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
