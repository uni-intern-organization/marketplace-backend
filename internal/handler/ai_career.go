package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/uni-intern-organization/marketplace-backend/internal/ai"
)

// AICareerHandler serves POST /api/ai/career/suggest (contract matches frontend VITE_CAREER_AI_API_URL).
type AICareerHandler struct {
	OpenAIKey   string
	OpenAIBase  string
	OpenAIModel string
}

func NewAICareerHandler(key, openAIBase, openAIModel string) *AICareerHandler {
	return &AICareerHandler{
		OpenAIKey:   key,
		OpenAIBase:  openAIBase,
		OpenAIModel: openAIModel,
	}
}

type careerSuggestRequest struct {
	Text string `json:"text"`
}

type careerSuggestResponse struct {
	Suggestions []ai.CareerSuggestionItem `json:"suggestions,omitempty"`
	Error       string                    `json:"error,omitempty"`
}

func (h *AICareerHandler) Suggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if strings.TrimSpace(h.OpenAIKey) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(careerSuggestResponse{Error: "ai not configured: set OPENAI_API_KEY on the server"})
		return
	}
	var in careerSuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(in.Text)
	if len(text) < 15 {
		http.Error(w, `{"error":"text too short"}`, http.StatusBadRequest)
		return
	}
	if len(text) > 32000 {
		text = text[:32000]
	}
	items, err := ai.SuggestCareerDirections(r.Context(), ai.OpenAIConfig{
		APIKey:  h.OpenAIKey,
		BaseURL: h.OpenAIBase,
		Model:   h.OpenAIModel,
	}, text)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(careerSuggestResponse{Error: "upstream: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(careerSuggestResponse{Suggestions: items})
}
