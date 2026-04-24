package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/uni-intern-organization/marketplace-backend/internal/ai"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

const (
	chunkMaxRunes = 1200
	chunkOverlap  = 150
)

// Indexer embeds vacancy text into rag_chunk rows. Safe to call with nil client (no-op).
type Indexer struct {
	Chunks     *repository.RagChunkRepository
	Client     *ai.Client
	AES        []byte
	EmbedModel string
}

// NewIndexer returns a non-nil indexer; Reindex is a no-op if Client is nil or has no API key.
func NewIndexer(chunks *repository.RagChunkRepository, client *ai.Client, aes []byte, embedModel string) *Indexer {
	return &Indexer{Chunks: chunks, Client: client, AES: aes, EmbedModel: embedModel}
}

// ScheduleReindex runs Reindex in a background goroutine (for HTTP handlers).
func (x *Indexer) ScheduleReindex(v *model.Vacancy) {
	if x == nil || v == nil {
		return
	}
	vv := *v
	go func() {
		_ = x.Reindex(context.Background(), &vv)
	}()
}

// Reindex replaces all chunks for one vacancy (decrypt title/description server-side only).
func (x *Indexer) Reindex(ctx context.Context, v *model.Vacancy) error {
	if x == nil || x.Chunks == nil || x.Client == nil || strings.TrimSpace(x.Client.APIKey) == "" {
		return nil
	}
	title, _ := crypto.Decrypt(v.TitleEnc, x.AES)
	desc, _ := crypto.Decrypt(v.DescriptionEnc, x.AES)
	doc := buildVacancyDocument(string(title), v, string(desc))
	parts := ai.SplitText(doc, chunkMaxRunes, chunkOverlap)
	if len(parts) == 0 {
		return x.Chunks.ReplaceVacancyChunks(ctx, v.ID, nil)
	}
	vec, err := x.Client.EmbedTexts(ctx, x.EmbedModel, parts)
	if err != nil {
		return err
	}
	if len(vec) != len(parts) {
		return fmt.Errorf("embeddings: got %d vectors for %d chunks", len(vec), len(parts))
	}
	rows := make([]repository.RagChunkRow, 0, len(parts))
	for i := range parts {
		if len(vec[i]) == 0 {
			return fmt.Errorf("embeddings: empty vector for chunk %d", i)
		}
		rows = append(rows, repository.RagChunkRow{
			Content:   parts[i],
			Embedding: vec[i],
		})
	}
	if len(rows) == 0 {
		return x.Chunks.ReplaceVacancyChunks(ctx, v.ID, nil)
	}
	return x.Chunks.ReplaceVacancyChunks(ctx, v.ID, rows)
}

func buildVacancyDocument(title string, v *model.Vacancy, description string) string {
	var b strings.Builder
	if t := strings.TrimSpace(title); t != "" {
		b.WriteString(t)
		b.WriteString("\n")
	}
	if v.CompanyName != "" {
		b.WriteString("Company: ")
		b.WriteString(v.CompanyName)
		b.WriteString("\n")
	}
	if v.RequiredSkills != "" {
		b.WriteString("Skills: ")
		b.WriteString(v.RequiredSkills)
		b.WriteString("\n")
	}
	if v.Location != "" {
		b.WriteString("Location: ")
		b.WriteString(v.Location)
		b.WriteString("\n")
	}
	if v.EmploymentType != "" {
		b.WriteString("Employment: ")
		b.WriteString(v.EmploymentType)
		b.WriteString("\n")
	}
	if v.MinExperienceYears > 0 {
		b.WriteString("Min years experience: ")
		b.WriteString(strconv.Itoa(v.MinExperienceYears))
		b.WriteString("\n")
	}
	if d := strings.TrimSpace(description); d != "" {
		b.WriteString(d)
	}
	return strings.TrimSpace(b.String())
}