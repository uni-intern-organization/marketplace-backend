package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Career IDs must stay in sync with the frontend (resumeCareerSuggestions.ts).
var allowedCareerIDs = map[string]struct{}{
	"frontend":  {},
	"backend":   {},
	"fullstack": {},
	"mobile":    {},
	"data":      {},
	"ml":        {},
	"devops":    {},
	"qa":        {},
	"security":  {},
	"product":   {},
	"design":    {},
	"marketing": {},
	"ba":        {},
	"hr":        {},
	"finance":   {},
}

// CareerSuggestionItem is the JSON shape expected by the SPA.
type CareerSuggestionItem struct {
	ID       string   `json:"id"`
	Score    int      `json:"score"`
	Keywords []string `json:"keywords"`
}

// OpenAIConfig holds non-secret and secret settings for a single request path.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string // e.g. https://api.openai.com/v1
	Model   string
	HTTP    *http.Client
}

// SuggestCareerDirections calls OpenAI-compatible chat completions and parses JSON.
func SuggestCareerDirections(ctx context.Context, cfg OpenAIConfig, resumeText string) ([]CareerSuggestionItem, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("openai: api key not configured")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	idsList := `frontend, backend, fullstack, mobile, data, ml, devops, qa, security, product, design, marketing, ba, hr, finance`
	system := fmt.Sprintf(`You are a career advisor. Given a resume in any language, output ONLY valid JSON, no markdown.
The JSON must have shape: {"suggestions":[{"id":"<id>","score":<0-100>,"keywords":["word1",...]}]}
Use only these id values: %s.
"score" reflects how well the resume matches that direction. Include 1-8 items, sort by score descending.
"keywords" are 2-6 short terms from the resume that support the match (any language).`, idsList)

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": "Resume text:\n\n" + resumeText},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.3,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("openai: status %d: %s", res.StatusCode, string(b))
	}

	var oai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &oai); err != nil {
		return nil, err
	}
	if len(oai.Choices) == 0 {
		return nil, errors.New("openai: empty choices")
	}
	var parsed struct {
		Suggestions []struct {
			ID       string   `json:"id"`
			Score    any      `json:"score"`
			Keywords any      `json:"keywords"`
		} `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(oai.Choices[0].Message.Content), &parsed); err != nil {
		return nil, fmt.Errorf("parse model json: %w", err)
	}

	var out []CareerSuggestionItem
	for _, s := range parsed.Suggestions {
		if _, ok := allowedCareerIDs[s.ID]; !ok {
			continue
		}
		score := 1
		switch v := s.Score.(type) {
		case float64:
			score = int(v)
		case int:
			score = v
		}
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		var kws []string
		if arr, ok := s.Keywords.([]any); ok {
			for _, x := range arr {
				if t, ok := x.(string); ok && t != "" {
					kws = append(kws, t)
				}
			}
		}
		out = append(out, CareerSuggestionItem{ID: s.ID, Score: score, Keywords: kws})
	}
	if len(out) == 0 {
		return nil, errors.New("no valid career ids in model output")
	}
	return out, nil
}
