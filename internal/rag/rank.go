package rag

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

// RankedVacancy merges semantic retrieval with keyword overlap vs resume text.
type RankedVacancy struct {
	VacancyID      uuid.UUID
	Semantic       float64 // raw cosine similarity [0,1]
	Keyword        float64 // fraction of required skill tokens found in resume [0,1]
	Combined       float64 // weighted mix after normalizing semantics in batch [0,1]
	MatchPct       int     // display 0–100
	Title          string
	Company        string
	Location       string
	EmploymentType string
	RequiredSkills string
	ChunkExcerpt   string
}

func splitSkillTokens(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		t := strings.ToLower(strings.TrimSpace(p))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// keywordCoverage returns share of required skill tokens that appear as substrings in resumeText.
func keywordCoverage(resumeLower, requiredSkills string) float64 {
	parts := splitSkillTokens(requiredSkills)
	if len(parts) == 0 {
		return 0.35
	}
	hit := 0
	for _, p := range parts {
		if p != "" && strings.Contains(resumeLower, p) {
			hit++
		}
	}
	return float64(hit) / float64(len(parts))
}

const (
	semanticWeight = 0.42
	keywordWeight  = 0.58
)

func normalizeSemantics(rows []RankedVacancy) {
	if len(rows) == 0 {
		return
	}
	minS, maxS := rows[0].Semantic, rows[0].Semantic
	for _, r := range rows[1:] {
		if r.Semantic < minS {
			minS = r.Semantic
		}
		if r.Semantic > maxS {
			maxS = r.Semantic
		}
	}
	for i := range rows {
		var norm float64
		if maxS > minS {
			norm = (rows[i].Semantic - minS) / (maxS - minS)
		} else {
			norm = 1
		}
		comb := semanticWeight*norm + keywordWeight*rows[i].Keyword
		if comb > 1 {
			comb = 1
		}
		if comb < 0 {
			comb = 0
		}
		rows[i].Combined = comb
		rows[i].MatchPct = int(math.Round(100 * comb))
		if rows[i].MatchPct > 100 {
			rows[i].MatchPct = 100
		}
	}
}

// EnrichAndRank loads vacancy rows, computes hybrid scores, sorts by Combined descending.
func EnrichAndRank(ctx context.Context, vac *repository.VacancyRepository, aes []byte, resumeText string, chunks []TopChunk) []RankedVacancy {
	if vac == nil || len(chunks) == 0 {
		return nil
	}
	resumeLower := strings.ToLower(resumeText)
	var out []RankedVacancy
	for _, ch := range chunks {
		v, err := vac.GetByID(ctx, ch.VacancyID)
		if err != nil || v == nil {
			continue
		}
		title := ""
		if len(v.TitleEnc) > 0 && len(aes) > 0 {
			if b, err := crypto.Decrypt(v.TitleEnc, aes); err == nil {
				title = string(b)
			}
		}
		if strings.TrimSpace(title) == "" && v.CompanyName != "" {
			title = v.CompanyName
		}
		kw := keywordCoverage(resumeLower, v.RequiredSkills)
		out = append(out, RankedVacancy{
			VacancyID:      v.ID,
			Semantic:       ch.Score,
			Keyword:        kw,
			Title:          title,
			Company:        v.CompanyName,
			Location:       v.Location,
			EmploymentType: v.EmploymentType,
			RequiredSkills: v.RequiredSkills,
			ChunkExcerpt:   ch.Content,
		})
	}
	if len(out) == 0 {
		return nil
	}
	normalizeSemantics(out)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Combined == out[j].Combined {
			return out[i].Semantic > out[j].Semantic
		}
		return out[i].Combined > out[j].Combined
	})
	return out
}

// PickContrast finds a high-semantic but low-keyword row (misleading embedding match).
func PickContrast(ranked []RankedVacancy) *RankedVacancy {
	tmp := append([]RankedVacancy(nil), ranked...)
	sort.Slice(tmp, func(i, j int) bool { return tmp[i].Semantic > tmp[j].Semantic })
	for i := range tmp {
		if tmp[i].Semantic >= 0.32 && tmp[i].Keyword < 0.28 && tmp[i].MatchPct <= 55 {
			return &tmp[i]
		}
	}
	return nil
}

// WeakSemanticCandidate picks a lower-ranked opening (borderline fit) for contrast narrative.
func WeakSemanticCandidate(ranked []RankedVacancy, exclude map[uuid.UUID]struct{}) *RankedVacancy {
	n := len(ranked)
	if n == 0 {
		return nil
	}
	start := n - 4
	if start < 0 {
		start = 0
	}
	for i := n - 1; i >= start; i-- {
		rv := ranked[i]
		if _, skip := exclude[rv.VacancyID]; skip {
			continue
		}
		if rv.MatchPct >= 22 && rv.MatchPct <= 58 {
			return &ranked[i]
		}
	}
	return nil
}

// RankSemanticOnly is used when VacancyRepository is not wired (keyword overlap unavailable).
func RankSemanticOnly(chunks []TopChunk) []RankedVacancy {
	out := make([]RankedVacancy, 0, len(chunks))
	for _, ch := range chunks {
		out = append(out, RankedVacancy{
			VacancyID:    ch.VacancyID,
			Semantic:     ch.Score,
			Keyword:      0.45,
			ChunkExcerpt: ch.Content,
		})
	}
	normalizeSemantics(out)
	sort.Slice(out, func(i, j int) bool { return out[i].Combined > out[j].Combined })
	return out
}
