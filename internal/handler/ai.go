package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type AIHandler struct {
	billingSvc  *billing.Service
	vacancyRepo *repository.VacancyRepository
	userRepo    *repository.UserRepository
}

func NewAIHandler(billingSvc *billing.Service, vacancyRepo *repository.VacancyRepository, userRepo *repository.UserRepository) *AIHandler {
	return &AIHandler{billingSvc: billingSvc, vacancyRepo: vacancyRepo, userRepo: userRepo}
}

func (h *AIHandler) callClaude(system, user string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "AI assistant is not configured. Set ANTHROPIC_API_KEY on the server.", nil
	}
	body := map[string]interface{}{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 1024,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return string(data), nil
	}
	if len(parsed.Content) > 0 {
		return parsed.Content[0].Text, nil
	}
	return string(data), nil
}

func (h *AIHandler) CareerChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
		Context string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, `{"error":"message required"}`, http.StatusBadRequest)
		return
	}
	text, err := h.callClaude("You are Steppy career assistant for students in Kazakhstan. Be concise and practical.", req.Context+"\n\n"+req.Message)
	if err != nil {
		http.Error(w, `{"error":"ai failed"}`, http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]string{"reply": text})
}

func (h *AIHandler) CoverLetter(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile  string `json:"profile"`
		Vacancy  string `json:"vacancy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	text, err := h.callClaude("Write a short cover letter in Russian for a student applying to a vacancy.", "Profile:\n"+req.Profile+"\n\nVacancy:\n"+req.Vacancy)
	if err != nil {
		http.Error(w, `{"error":"ai failed"}`, http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]string{"cover_letter": text})
}

func (h *AIHandler) ImproveVacancy(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims.Role == model.RoleRecruiter {
		ent, _ := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
		if !ent.IsPro && ent.Plan != model.RecruiterPlanBusiness && ent.Plan != model.RecruiterPlanCorporate && ent.Plan != model.RecruiterPlanStarter {
			billing.WriteError(w, http.StatusForbidden, "subscription_required", "requires Business subscription or higher")
			return
		}
	}
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	text, err := h.callClaude("Suggest improvements for a job posting targeting students in Kazakhstan.", req.Title+"\n"+req.Description)
	if err != nil {
		http.Error(w, `{"error":"ai failed"}`, http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]string{"suggestions": text})
}

func (h *AIHandler) AnalyzeResume(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ResumeText string `json:"resume_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	text, err := h.callClaude("Analyze resume and give bullet-point improvements for a junior in Kazakhstan.", req.ResumeText)
	if err != nil {
		http.Error(w, `{"error":"ai failed"}`, http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]string{"analysis": text})
}

func (h *AIHandler) InterviewPrep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Vacancy string `json:"vacancy"`
		Company string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	text, err := h.callClaude("Generate 5 likely interview questions with brief answer tips.", req.Company+" — "+req.Vacancy)
	if err != nil {
		http.Error(w, `{"error":"ai failed"}`, http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]string{"prep": text})
}

func (h *AIHandler) Recommendations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Skills   string `json:"skills"`
		Location string `json:"location"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	p, _ := h.userRepo.GetStudentProfileByUserID(r.Context(), claims.UserID)
	skills := req.Skills
	if skills == "" && p != nil {
		skills = p.Skills
	}
	list, err := h.vacancyRepo.List(r.Context(), repository.VacancyFilter{Skills: skills, Location: req.Location}, 10)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusInternalServerError)
		return
	}
	ids := make([]string, 0, len(list))
	for _, v := range list {
		ids = append(ids, v.ID.String())
	}
	text, _ := h.callClaude("Summarize why these vacancy IDs match the student profile.", strings.Join(ids, ", "))
	jsonOK(w, map[string]interface{}{"vacancy_ids": ids, "summary": text})
}

func (h *AIHandler) SuggestCandidates(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	ent, _ := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
	if !ent.IsPro && ent.Plan != model.RecruiterPlanBusiness && ent.Plan != model.RecruiterPlanCorporate {
		billing.WriteError(w, http.StatusForbidden, "subscription_required", "requires Business subscription or higher")
		return
	}
	var req struct {
		VacancyID string `json:"vacancy_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VacancyID == "" {
		http.Error(w, `{"error":"vacancy_id required"}`, http.StatusBadRequest)
		return
	}
	students, _ := h.userRepo.ListStudentProfilesForMatching(r.Context())
	var hints []string
	for _, s := range students {
		if s.Skills != "" {
			hints = append(hints, s.UserID.String()+":"+s.Skills)
		}
	}
	text, err := h.callClaude("Suggest top student IDs for this vacancy from the list user_id:skills.", req.VacancyID+"\n"+strings.Join(hints, "\n"))
	if err != nil {
		http.Error(w, `{"error":"ai failed"}`, http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]string{"suggestions": text})
}
