package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client calls OpenAI-compatible /v1/embeddings and /v1/chat/completions.
type Client struct {
	APIKey         string
	BaseURL        string
	ChatModel      string
	EmbeddingModel string
	HTTP           *http.Client
}

func (c *Client) base() string {
	b := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if b == "" {
		b = "https://api.openai.com/v1"
	}
	return b
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 120 * time.Second}
}

// EmbedTexts returns one vector per input string, same order.
func (c *Client) EmbedTexts(ctx context.Context, model string, inputs []string) ([][]float64, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("openai: missing api key")
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	if model == "" {
		model = "text-embedding-3-small"
	}
	body, err := json.Marshal(map[string]any{
		"model": model,
		"input": inputs,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings: %d %s", res.StatusCode, string(raw))
	}
	var parsed struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	sort.Slice(parsed.Data, func(i, j int) bool { return parsed.Data[i].Index < parsed.Data[j].Index })
	out := make([][]float64, len(inputs))
	for _, d := range parsed.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, nil
}

// ChatJSONObject posts chat completions with response_format json_object and returns raw message content.
func (c *Client) ChatJSONObject(ctx context.Context, system, user string) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("openai: missing api key")
	}
	m := c.ChatModel
	if m == "" {
		m = "gpt-4o-mini"
	}
	body, err := json.Marshal(map[string]any{
		"model": m,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.35,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.http().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("chat: %d %s", res.StatusCode, string(raw))
	}
	var oai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &oai); err != nil {
		return "", err
	}
	if len(oai.Choices) == 0 {
		return "", fmt.Errorf("chat: empty choices")
	}
	return oai.Choices[0].Message.Content, nil
}

// ChatMessage is one turn for OpenAI chat completions.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletion sends a multi-turn conversation (must include optional system as first element).
func (c *Client) ChatCompletion(ctx context.Context, msgs []ChatMessage, temperature float64, maxTokens int) (string, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return "", fmt.Errorf("openai: missing api key")
	}
	if len(msgs) == 0 {
		return "", fmt.Errorf("chat: empty messages")
	}
	m := c.ChatModel
	if m == "" {
		m = "gpt-4o-mini"
	}
	if temperature <= 0 {
		temperature = 0.7
	}
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	rawMsgs := make([]map[string]string, 0, len(msgs))
	for _, x := range msgs {
		r := strings.TrimSpace(x.Role)
		if r != "system" && r != "user" && r != "assistant" {
			continue
		}
		rawMsgs = append(rawMsgs, map[string]string{"role": r, "content": x.Content})
	}
	if len(rawMsgs) == 0 {
		return "", fmt.Errorf("chat: no valid messages")
	}
	body, err := json.Marshal(map[string]any{
		"model":       m,
		"messages":    rawMsgs,
		"temperature": temperature,
		"max_tokens":  maxTokens,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	res, err := c.http().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", fmt.Errorf("chat: %d %s", res.StatusCode, string(raw))
	}
	var oai struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &oai); err != nil {
		return "", err
	}
	if len(oai.Choices) == 0 {
		return "", fmt.Errorf("chat: empty choices")
	}
	return oai.Choices[0].Message.Content, nil
}
