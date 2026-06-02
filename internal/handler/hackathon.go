package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type HackathonHandler struct {
	repo        *repository.HackathonRepository
	billingRepo *repository.BillingRepository
	cfg         *config.BillingConfig
	aesKey      []byte
}

func NewHackathonHandler(repo *repository.HackathonRepository, billingRepo *repository.BillingRepository, cfg *config.BillingConfig, aesKey []byte) *HackathonHandler {
	return &HackathonHandler{repo: repo, billingRepo: billingRepo, cfg: cfg, aesKey: aesKey}
}

type hackathonResp struct {
	ID                   string  `json:"id"`
	OrganizerID          string  `json:"organizer_id"`
	Title                string  `json:"title"`
	Description          string  `json:"description"`
	Theme                string  `json:"theme"`
	Format               string  `json:"format"`
	PrizePoolKZT         float64 `json:"prize_pool_kzt"`
	MinParticipants      int     `json:"min_participants"`
	MaxParticipants      int     `json:"max_participants"`
	StartsAt             string  `json:"starts_at"`
	EndsAt               string  `json:"ends_at"`
	RegistrationDeadline string  `json:"registration_deadline"`
	ListingFeePaid       bool    `json:"listing_fee_paid"`
	Status               string  `json:"status"`
	RegistrationCount    int     `json:"registration_count,omitempty"`
}

func hackToResp(h *model.Hackathon, key []byte, regCount int) hackathonResp {
	r := hackathonResp{
		ID: h.ID.String(), OrganizerID: h.OrganizerID.String(), Theme: h.Theme, Format: h.Format,
		PrizePoolKZT: h.PrizePoolKZT, MinParticipants: h.MinParticipants, MaxParticipants: h.MaxParticipants,
		StartsAt: h.StartsAt.Format(time.RFC3339), EndsAt: h.EndsAt.Format(time.RFC3339),
		RegistrationDeadline: h.RegistrationDeadline.Format(time.RFC3339),
		ListingFeePaid: h.ListingFeePaid, Status: h.Status, RegistrationCount: regCount,
	}
	if len(h.TitleEnc) > 0 {
		b, _ := crypto.Decrypt(h.TitleEnc, key)
		r.Title = string(b)
	}
	if len(h.DescriptionEnc) > 0 {
		b, _ := crypto.Decrypt(h.DescriptionEnc, key)
		r.Description = string(b)
	}
	return r
}

func hackathonListingFee(prize float64) int {
	switch {
	case prize >= 500000:
		return 50000
	case prize >= 100000:
		return 25000
	default:
		return 10000
	}
}

func (h *HackathonHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Title                string  `json:"title"`
		Description          string  `json:"description"`
		Theme                string  `json:"theme"`
		Format               string  `json:"format"`
		PrizePoolKZT         float64 `json:"prize_pool_kzt"`
		MinParticipants      int     `json:"min_participants"`
		MaxParticipants      int     `json:"max_participants"`
		StartsAt             string  `json:"starts_at"`
		EndsAt               string  `json:"ends_at"`
		RegistrationDeadline string  `json:"registration_deadline"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	parseT := func(s string, def time.Time) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return def
		}
		return t
	}
	now := time.Now()
	starts := parseT(req.StartsAt, now.Add(7*24*time.Hour))
	ends := parseT(req.EndsAt, starts.Add(48*time.Hour))
	reg := parseT(req.RegistrationDeadline, starts)
	if req.Format == "" {
		req.Format = "team"
	}
	if req.MaxParticipants <= 0 {
		req.MaxParticipants = 100
	}
	titleEnc, _ := crypto.Encrypt([]byte(req.Title), h.aesKey)
	descEnc, _ := crypto.Encrypt([]byte(req.Description), h.aesKey)
	hc, err := h.repo.Create(r.Context(), claims.UserID, titleEnc, descEnc, req.Theme, req.Format, req.PrizePoolKZT, req.MinParticipants, req.MaxParticipants, starts, ends, reg)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hackToResp(hc, h.aesKey, 0))
}

func (h *HackathonHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	list, err := h.repo.ListPublished(r.Context(), limit)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	resp := make([]hackathonResp, 0, len(list))
	for i := range list {
		n, _ := h.repo.RegistrationCount(r.Context(), list[i].ID)
		resp = append(resp, hackToResp(&list[i], h.aesKey, n))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HackathonHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	list, err := h.repo.ListByOrganizer(r.Context(), claims.UserID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	resp := make([]hackathonResp, 0, len(list))
	for i := range list {
		n, _ := h.repo.RegistrationCount(r.Context(), list[i].ID)
		resp = append(resp, hackToResp(&list[i], h.aesKey, n))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HackathonHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, err := h.repo.Get(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	n, _ := h.repo.RegistrationCount(r.Context(), id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hackToResp(hc, h.aesKey, n))
}

func (h *HackathonHandler) Publish(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, err := h.repo.Get(r.Context(), id)
	if err != nil || hc.OrganizerID != claims.UserID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	fee := hackathonListingFee(hc.PrizePoolKZT)
	if err := h.repo.Publish(r.Context(), id, claims.UserID); err != nil {
		RespondError(w, http.StatusInternalServerError, "publish failed", err)
		return
	}
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "hackathon_listing", map[string]interface{}{
		"hackathon_id": id.String(), "fee_kzt": fee, "demo": true,
	})
	_ = h.billingRepo.LogNotification(r.Context(), claims.UserID, "email", "Hackathon published", "Demo: students notified about new hackathon")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "published", "listing_fee_kzt": fee})
}

func (h *HackathonHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		InviteCode string `json:"invite_code"`
		TeamID     string `json:"team_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var teamID *uuid.UUID
	if req.InviteCode != "" {
		teamID, err = h.repo.JoinTeamByCode(r.Context(), id, claims.UserID, req.InviteCode)
	} else if req.TeamID != "" {
		tid, _ := uuid.Parse(req.TeamID)
		teamID = &tid
		err = h.repo.Register(r.Context(), id, claims.UserID, teamID)
	} else {
		err = h.repo.Register(r.Context(), id, claims.UserID, nil)
	}
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "register failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
}

func (h *HackathonHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct{ Name string `json:"name"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "Team"
	}
	t, err := h.repo.CreateTeam(r.Context(), hackID, claims.UserID, req.Name)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "team failed", err)
		return
	}
	_ = h.repo.Register(r.Context(), hackID, claims.UserID, &t.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"team_id": t.ID.String(), "invite_code": t.InviteCode})
}

func (h *HackathonHandler) Submit(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		ArtifactKey string `json:"artifact_key"`
		TeamID      string `json:"team_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var teamID *uuid.UUID
	if req.TeamID != "" {
		tid, _ := uuid.Parse(req.TeamID)
		teamID = &tid
	}
	sid := claims.UserID
	if teamID != nil {
		sid = uuid.Nil
	}
	var studentID *uuid.UUID
	if sid != uuid.Nil {
		studentID = &sid
	}
	if err := h.repo.CreateSubmission(r.Context(), hackID, teamID, studentID, req.ArtifactKey); err != nil {
		RespondError(w, http.StatusInternalServerError, "submit failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
}

func (h *HackathonHandler) PublishResults(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, _ := h.repo.Get(r.Context(), hackID)
	if hc == nil || hc.OrganizerID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Results []struct {
			StudentID       string  `json:"student_id"`
			TeamID          string  `json:"team_id"`
			Place           int     `json:"place"`
			PrizeAmountKZT  float64 `json:"prize_amount_kzt"`
			InternshipOffer bool    `json:"internship_offer"`
		} `json:"results"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	var results []model.HackathonResult
	for _, row := range req.Results {
		res := model.HackathonResult{HackathonID: hackID, Place: row.Place, PrizeAmountKZT: row.PrizeAmountKZT, InternshipOffer: row.InternshipOffer}
		if row.StudentID != "" {
			sid, _ := uuid.Parse(row.StudentID)
			res.StudentID = &sid
		}
		if row.TeamID != "" {
			tid, _ := uuid.Parse(row.TeamID)
			res.TeamID = &tid
		}
		results = append(results, res)
	}
	if err := h.repo.PublishResults(r.Context(), hackID, results); err != nil {
		RespondError(w, http.StatusInternalServerError, "results failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *HackathonHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.Leaderboard(r.Context(), id)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "leaderboard failed", err)
		return
	}
	type row struct {
		Place          int     `json:"place"`
		PrizeAmountKZT float64 `json:"prize_amount_kzt"`
		InternshipOffer bool   `json:"internship_offer"`
	}
	resp := make([]row, 0, len(list))
	for _, res := range list {
		resp = append(resp, row{Place: res.Place, PrizeAmountKZT: res.PrizeAmountKZT, InternshipOffer: res.InternshipOffer})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *HackathonHandler) AddAnnouncement(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.repo.AddAnnouncement(r.Context(), hackID, req.Title, req.Body); err != nil {
		RespondError(w, http.StatusInternalServerError, "announcement failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) ListCriteria(w http.ResponseWriter, r *http.Request) {
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListCriteria(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"criteria": list})
}

func (h *HackathonHandler) CreateCriterion(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, _ := h.repo.Get(r.Context(), hackID)
	if hc == nil || hc.OrganizerID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Name          string `json:"name"`
		WeightPercent int    `json:"weight_percent"`
		SortOrder     int    `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, `{"error":"name required"}`, http.StatusBadRequest)
		return
	}
	c, err := h.repo.CreateCriterion(r.Context(), hackID, req.Name, req.WeightPercent, req.SortOrder)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	jsonOK(w, c)
}

func (h *HackathonHandler) UpdateCriterion(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	criterionID, err := uuid.Parse(r.URL.Query().Get("criterion_id"))
	if err != nil {
		http.Error(w, `{"error":"criterion_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Name          string `json:"name"`
		WeightPercent int    `json:"weight_percent"`
		SortOrder     int    `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.repo.UpdateCriterion(r.Context(), criterionID, claims.UserID, req.Name, req.WeightPercent, req.SortOrder); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) DeleteCriterion(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	criterionID, err := uuid.Parse(r.URL.Query().Get("criterion_id"))
	if err != nil {
		http.Error(w, `{"error":"criterion_id required"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteCriterion(r.Context(), criterionID, claims.UserID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) SubmitScores(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, _ := h.repo.Get(r.Context(), hackID)
	if hc == nil || hc.OrganizerID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Scores []struct {
			SubmissionID string  `json:"submission_id"`
			CriterionID  string  `json:"criterion_id"`
			Score        float64 `json:"score"`
		} `json:"scores"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	for _, s := range req.Scores {
		subID, _ := uuid.Parse(s.SubmissionID)
		critID, _ := uuid.Parse(s.CriterionID)
		_ = h.repo.UpsertScore(r.Context(), hackID, subID, critID, s.Score)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) ListTeamRequests(w http.ResponseWriter, r *http.Request) {
	teamID, err := uuid.Parse(r.URL.Query().Get("team_id"))
	if err != nil {
		http.Error(w, `{"error":"team_id required"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListTeamRequests(r.Context(), teamID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"requests": list})
}

func (h *HackathonHandler) CreateTeamRequest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	teamID, err := uuid.Parse(r.URL.Query().Get("team_id"))
	if err != nil {
		http.Error(w, `{"error":"team_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	tr, err := h.repo.CreateTeamRequest(r.Context(), teamID, claims.UserID, req.Message)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "request failed", err)
		return
	}
	jsonOK(w, tr)
}

func (h *HackathonHandler) PatchTeamRequest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	requestID, err := uuid.Parse(r.URL.Query().Get("request_id"))
	if err != nil {
		http.Error(w, `{"error":"request_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Status == "" {
		http.Error(w, `{"error":"status required"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.UpdateTeamRequest(r.Context(), requestID, claims.UserID, req.Status); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusForbidden)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListAnnouncements(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, a := range list {
		out = append(out, map[string]interface{}{
			"id": a.ID.String(), "title": a.Title, "body": a.Body,
			"created_at": a.CreatedAt.Format(time.RFC3339),
		})
	}
	jsonOK(w, map[string]interface{}{"announcements": out})
}

func (h *HackathonHandler) MyCertificates(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	list, err := h.repo.ListCertificatesForStudent(r.Context(), claims.UserID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"certificates": list})
}
