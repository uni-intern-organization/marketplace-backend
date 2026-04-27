package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/rag"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

// RAGHandler serves POST /api/career/rag (RAG on indexed vacancies).
type RAGHandler struct {
	Engine *rag.Engine
}

func NewRAGHandler(e *rag.Engine) *RAGHandler {
	return &RAGHandler{Engine: e}
}

type careerRAGRequest struct {
	Text string `json:"text"`
}

type careerRAGResponse struct {
	Narrative      string           `json:"narrative"`
	VacancyIDs     []string         `json:"vacancy_ids"`
	Retrieval      []rag.TopChunk   `json:"retrieval_top,omitempty"`
	VacancyCards   []rag.VacancyPick `json:"vacancy_cards,omitempty"`
	Error          string           `json:"error,omitempty"`
}

// CareerRAG is open to the same users as the career page: no auth, text-only (length limits).
func (h *RAGHandler) CareerRAG(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if h == nil || h.Engine == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(careerRAGResponse{Error: "RAG not configured (OPENAI_API_KEY, migrations)"})
		return
	}
	var in careerRAGRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	out, err := h.Engine.Answer(r.Context(), in.Text)
	if err != nil {
		if strings.Contains(err.Error(), "not configured") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(careerRAGResponse{Error: err.Error()})
			return
		}
		if strings.Contains(err.Error(), "too short") {
			http.Error(w, `{"error":"text too short"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(careerRAGResponse{Error: err.Error()})
		return
	}
	ids := make([]string, 0, len(out.VacancyIDs))
	for _, id := range out.VacancyIDs {
		ids = append(ids, id.String())
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(careerRAGResponse{
		Narrative:    out.Narrative,
		VacancyIDs:   ids,
		Retrieval:    out.Chunks,
		VacancyCards: out.Picks,
	})
}

// AdminRAGHandler runs full reindex (admin only).
type AdminRAGHandler struct {
	Vacancy *repository.VacancyRepository
	Indexer *rag.Indexer
}

func NewAdminRAGHandler(vac *repository.VacancyRepository, idx *rag.Indexer) *AdminRAGHandler {
	return &AdminRAGHandler{Vacancy: vac, Indexer: idx}
}

type adminReindexResponse struct {
	Indexed int    `json:"vacancies_queued"`
	Note    string `json:"note"`
	Error   string `json:"error,omitempty"`
}

// ReindexAll re-embeds all vacancies. Accepts the request, then processes in the background.
func (h *AdminRAGHandler) ReindexAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleAdmin {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if h == nil || h.Vacancy == nil || h.Indexer == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(adminReindexResponse{Error: "RAG index not configured"})
		return
	}
	list, err := h.Vacancy.List(r.Context(), repository.VacancyFilter{}, 10000)
	if err != nil {
		http.Error(w, `{"error":"list failed"}`, http.StatusInternalServerError)
		return
	}
	// copy slice for the goroutine
	queued := make([]model.Vacancy, len(list))
	copy(queued, list)
	go func() {
		for i := range queued {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			_ = h.Indexer.Reindex(ctx, &queued[i])
			cancel()
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(adminReindexResponse{
		Indexed: len(list),
		Note:    "reindex running in background (one vacancy at a time with timeout per job)",
	})
}

// ReindexMine re-embeds only the current recruiter’s vacancies (no admin role).
func (h *AdminRAGHandler) ReindexMine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if h == nil || h.Vacancy == nil || h.Indexer == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(adminReindexResponse{Error: "RAG index not configured"})
		return
	}
	list, err := h.Vacancy.ListByRecruiter(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, `{"error":"list failed"}`, http.StatusInternalServerError)
		return
	}
	queued := make([]model.Vacancy, len(list))
	copy(queued, list)
	go func() {
		for i := range queued {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			_ = h.Indexer.Reindex(ctx, &queued[i])
			cancel()
		}
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(adminReindexResponse{
		Indexed: len(list),
		Note:    "reindex for your vacancies running in background",
	})
}
