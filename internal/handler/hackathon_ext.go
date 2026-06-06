package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/hackathoncert"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
	hacksvc "github.com/uni-intern-organization/marketplace-backend/internal/service/hackathon"
)

type hackathonRespFull struct {
	hackathonResp
	Rules                string          `json:"rules,omitempty"`
	PrizeType            string          `json:"prize_type"`
	PrizeBreakdown       json.RawMessage `json:"prize_breakdown,omitempty"`
	TeamMinSize          int             `json:"team_min_size"`
	TeamMaxSize          int             `json:"team_max_size"`
	MaxRegistrations     int             `json:"max_registrations"`
	RegistrationOpensAt  string          `json:"registration_opens_at,omitempty"`
	ResultsAnnouncedAt   string          `json:"results_announced_at,omitempty"`
	RegistrationMode     string          `json:"registration_mode"`
	TaskReveal           string          `json:"task_reveal"`
	TaskBody             string          `json:"task_body,omitempty"`
	SubmissionSchema     json.RawMessage `json:"submission_schema,omitempty"`
	BlindJudging         bool            `json:"blind_judging"`
	WinnerMode           string          `json:"winner_mode"`
	PublicSubmissions    bool            `json:"public_submissions"`
	SubmissionCount      int             `json:"submission_count,omitempty"`
	MyRegistrationStatus string          `json:"my_registration_status,omitempty"`
}

func hackToRespFull(h *model.Hackathon, key []byte, regCount, subCount int) hackathonRespFull {
	base := hackToResp(h, key, regCount)
	r := hackathonRespFull{
		hackathonResp:     base,
		PrizeType:         h.PrizeType,
		PrizeBreakdown:    h.PrizeBreakdown,
		TeamMinSize:       h.TeamMinSize,
		TeamMaxSize:       h.TeamMaxSize,
		MaxRegistrations:  h.MaxParticipants,
		RegistrationMode:  h.RegistrationMode,
		TaskReveal:        h.TaskReveal,
		SubmissionSchema:  h.SubmissionSchema,
		BlindJudging:      h.BlindJudging,
		WinnerMode:        h.WinnerMode,
		PublicSubmissions: h.PublicSubmissions,
		SubmissionCount:   subCount,
	}
	if len(h.RulesEnc) > 0 {
		b, _ := crypto.Decrypt(h.RulesEnc, key)
		r.Rules = string(b)
	}
	if h.RegistrationOpensAt != nil {
		r.RegistrationOpensAt = h.RegistrationOpensAt.Format(time.RFC3339)
	}
	if h.ResultsAnnouncedAt != nil {
		r.ResultsAnnouncedAt = h.ResultsAnnouncedAt.Format(time.RFC3339)
	}
	return r
}

func (h *HackathonHandler) ensureSvc() *hacksvc.Service {
	if h.svc == nil {
		h.svc = hacksvc.New(h.repo, h.billingRepo)
	}
	return h.svc
}

func (h *HackathonHandler) decryptField(enc []byte) string {
	if len(enc) == 0 {
		return ""
	}
	b, _ := crypto.Decrypt(enc, h.aesKey)
	return string(b)
}

func (h *HackathonHandler) parseHackathonBody(r *http.Request) (model.HackathonUpdateInput, error) {
	var req struct {
		Title                string          `json:"title"`
		Description          string          `json:"description"`
		Rules                string          `json:"rules"`
		Theme                string          `json:"theme"`
		Format               string          `json:"format"`
		PrizePoolKZT         float64         `json:"prize_pool_kzt"`
		PrizeType            string          `json:"prize_type"`
		PrizeBreakdown       json.RawMessage `json:"prize_breakdown"`
		MinParticipants      int             `json:"min_participants"`
		MaxParticipants      int             `json:"max_participants"`
		MaxRegistrations     int             `json:"max_registrations"`
		TeamMinSize          int             `json:"team_min_size"`
		TeamMaxSize          int             `json:"team_max_size"`
		StartsAt             string          `json:"starts_at"`
		EndsAt               string          `json:"ends_at"`
		RegistrationOpensAt  string          `json:"registration_opens_at"`
		RegistrationDeadline string          `json:"registration_deadline"`
		ResultsAnnouncedAt   string          `json:"results_announced_at"`
		RegistrationMode     string          `json:"registration_mode"`
		TaskReveal           string          `json:"task_reveal"`
		TaskBody             string          `json:"task_body"`
		SubmissionSchema     json.RawMessage `json:"submission_schema"`
		BlindJudging         bool            `json:"blind_judging"`
		WinnerMode           string          `json:"winner_mode"`
		PublicSubmissions    bool            `json:"public_submissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return model.HackathonUpdateInput{}, err
	}
	parseT := func(s string, def time.Time) time.Time {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return def
		}
		return t
	}
	now := time.Now()
	maxReg := req.MaxParticipants
	if req.MaxRegistrations > 0 {
		maxReg = req.MaxRegistrations
	}
	if maxReg <= 0 {
		maxReg = 100
	}
	teamMin, teamMax := req.TeamMinSize, req.TeamMaxSize
	if teamMin <= 0 {
		teamMin = 1
	}
	if teamMax <= 0 {
		teamMax = 5
	}
	starts := parseT(req.StartsAt, now.Add(7*24*time.Hour))
	ends := parseT(req.EndsAt, starts.Add(48*time.Hour))
	regOpen := parseT(req.RegistrationOpensAt, now)
	regDeadline := parseT(req.RegistrationDeadline, starts)
	resultsAt := parseT(req.ResultsAnnouncedAt, ends.Add(7*24*time.Hour))
	prizeType := req.PrizeType
	if prizeType == "" {
		prizeType = model.HackathonPrizeNone
	}
	regMode := req.RegistrationMode
	if regMode == "" {
		regMode = model.HackathonRegAuto
	}
	taskReveal := req.TaskReveal
	if taskReveal == "" {
		taskReveal = model.HackathonTaskAtStart
	}
	winnerMode := req.WinnerMode
	if winnerMode == "" {
		winnerMode = model.HackathonWinnerAuto
	}
	schema := req.SubmissionSchema
	if len(schema) == 0 {
		schema = json.RawMessage(`{"artifact":true,"description":true}`)
	}
	breakdown := req.PrizeBreakdown
	if len(breakdown) == 0 {
		breakdown = json.RawMessage(`{"first":0,"second":0,"third":0,"nominations":[]}`)
	}
	return model.HackathonUpdateInput{
		Title: req.Title, Description: req.Description, Rules: req.Rules, Theme: req.Theme,
		Format: req.Format, PrizePoolKZT: req.PrizePoolKZT, PrizeType: prizeType, PrizeBreakdown: breakdown,
		MinParticipants: req.MinParticipants, MaxParticipants: maxReg, TeamMinSize: teamMin, TeamMaxSize: teamMax,
		StartsAt: starts, EndsAt: ends, RegistrationOpensAt: regOpen, RegistrationDeadline: regDeadline,
		ResultsAnnouncedAt: resultsAt, RegistrationMode: regMode, TaskReveal: taskReveal, TaskBody: req.TaskBody,
		SubmissionSchema: schema, BlindJudging: req.BlindJudging, WinnerMode: winnerMode, PublicSubmissions: req.PublicSubmissions,
	}, nil
}

func (h *HackathonHandler) Patch(w http.ResponseWriter, r *http.Request) {
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
	in, err := h.parseHackathonBody(r)
	if err != nil || in.Title == "" {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	titleEnc, _ := crypto.Encrypt([]byte(in.Title), h.aesKey)
	descEnc, _ := crypto.Encrypt([]byte(in.Description), h.aesKey)
	rulesEnc, _ := crypto.Encrypt([]byte(in.Rules), h.aesKey)
	taskEnc, _ := crypto.Encrypt([]byte(in.TaskBody), h.aesKey)
	priority := hacksvc.CatalogPriority(in.PrizeType, in.PrizePoolKZT)
	if err := h.repo.UpdateHackathon(r.Context(), id, claims.UserID, in, titleEnc, descEnc, rulesEnc, taskEnc, priority); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	hc, _ := h.repo.Get(r.Context(), id)
	n, _ := h.repo.ApprovedRegistrationCount(r.Context(), id)
	jsonOK(w, hackToRespFull(hc, h.aesKey, n, 0))
}

func (h *HackathonHandler) Preview(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, err := h.repo.Get(r.Context(), id)
	if err != nil || (claims != nil && hc.OrganizerID != claims.UserID) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	criteria, _ := h.repo.ListCriteria(r.Context(), id)
	jury, _ := h.repo.ListJuryMembers(r.Context(), id)
	n, _ := h.repo.ApprovedRegistrationCount(r.Context(), id)
	resp := hackToRespFull(hc, h.aesKey, n, 0)
	if h.ensureSvc().TaskVisible(hc) {
		resp.TaskBody = h.decryptField(hc.TaskBodyEnc)
	}
	jsonOK(w, map[string]interface{}{
		"hackathon": resp, "criteria": criteria, "jury": jury,
	})
}

func (h *HackathonHandler) ListFiltered(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	f := repository.ListHackathonFilter{Theme: q.Get("theme"), Format: q.Get("format"), PrizeType: q.Get("prize_type"), Sort: q.Get("sort"), Limit: limit}
	if s := q.Get("starts_after"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.StartsAfter = &t
		}
	}
	if s := q.Get("starts_before"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.StartsBefore = &t
		}
	}
	list, err := h.repo.ListPublishedFiltered(r.Context(), f)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	resp := make([]hackathonRespFull, 0, len(list))
	for i := range list {
		n, _ := h.repo.ApprovedRegistrationCount(r.Context(), list[i].ID)
		subs, _ := h.repo.SubmissionCount(r.Context(), list[i].ID)
		resp = append(resp, hackToRespFull(&list[i], h.aesKey, n, subs))
	}
	jsonOK(w, resp)
}

func (h *HackathonHandler) GetTask(w http.ResponseWriter, r *http.Request) {
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
	svc := h.ensureSvc()
	if !svc.TaskVisible(hc) {
		http.Error(w, `{"error":"task not available yet"}`, http.StatusForbidden)
		return
	}
	body := h.decryptField(hc.TaskBodyEnc)
	if body == "" {
		body = h.decryptField(hc.DescriptionEnc)
	}
	jsonOK(w, map[string]string{"task_body": body})
}

func (h *HackathonHandler) ListRegistrations(w http.ResponseWriter, r *http.Request) {
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
	list, err := h.repo.ListRegistrations(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"registrations": list})
}

func (h *HackathonHandler) PatchRegistration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	regID, err := uuid.Parse(r.URL.Query().Get("registration_id"))
	if err != nil {
		http.Error(w, `{"error":"registration_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.PatchRegistrationStatus(r.Context(), regID, claims.UserID, req.Status); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": req.Status})
}

func (h *HackathonHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListTeams(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"teams": list})
}

func (h *HackathonHandler) LeaveTeam(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, _ := h.repo.Get(r.Context(), hackID)
	if hc != nil && time.Now().After(hc.StartsAt) {
		http.Error(w, `{"error":"cannot leave after start"}`, http.StatusForbidden)
		return
	}
	if err := h.repo.LeaveTeam(r.Context(), hackID, claims.UserID); err != nil {
		RespondError(w, http.StatusInternalServerError, "leave failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) TransferCaptain(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	var req struct {
		TeamID       string `json:"team_id"`
		NewCaptainID string `json:"new_captain_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	teamID, _ := uuid.Parse(req.TeamID)
	newCap, _ := uuid.Parse(req.NewCaptainID)
	if err := h.repo.TransferCaptain(r.Context(), teamID, claims.UserID, newCap); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusForbidden)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) ListMaterials(w http.ResponseWriter, r *http.Request) {
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListMaterials(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"materials": list})
}

func (h *HackathonHandler) CreateMaterial(w http.ResponseWriter, r *http.Request) {
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
		Title      string `json:"title"`
		StorageKey string `json:"storage_key"`
		SortOrder  int    `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	m, err := h.repo.CreateMaterial(r.Context(), hackID, req.Title, req.StorageKey, req.SortOrder)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	jsonOK(w, m)
}

func (h *HackathonHandler) DeleteMaterial(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	mid, err := uuid.Parse(r.URL.Query().Get("material_id"))
	if err != nil {
		http.Error(w, `{"error":"material_id required"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteMaterial(r.Context(), mid, claims.UserID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) ListJury(w http.ResponseWriter, r *http.Request) {
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ListJuryMembers(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"jury": list})
}

func (h *HackathonHandler) CreateJury(w http.ResponseWriter, r *http.Request) {
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
		DisplayName string `json:"display_name"`
		Title       string `json:"title"`
		UserID      string `json:"user_id"`
		SortOrder   int    `json:"sort_order"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	var uid *uuid.UUID
	if req.UserID != "" {
		id, _ := uuid.Parse(req.UserID)
		uid = &id
	}
	m, err := h.repo.CreateJuryMember(r.Context(), hackID, req.DisplayName, req.Title, uid, req.SortOrder)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	jsonOK(w, m)
}

func (h *HackathonHandler) DeleteJury(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	jid, err := uuid.Parse(r.URL.Query().Get("jury_id"))
	if err != nil {
		http.Error(w, `{"error":"jury_id required"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.DeleteJuryMember(r.Context(), jid, claims.UserID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, _ := h.repo.Get(r.Context(), hackID)
	blind := hc != nil && hc.BlindJudging && hc.Status != "completed" && hc.Status != "archived"
	if claims != nil {
		if hc != nil && hc.OrganizerID == claims.UserID {
			blind = false
		}
		if jid, err := h.repo.IsJuryMember(r.Context(), hackID, claims.UserID); err == nil && jid != nil {
			blind = hc.BlindJudging && hc.Status != "completed"
		}
	}
	list, err := h.repo.ListSubmissions(r.Context(), hackID, blind)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"submissions": list})
}

func (h *HackathonHandler) MySubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	sub, err := h.repo.GetMySubmission(r.Context(), hackID, claims.UserID)
	if err != nil {
		jsonOK(w, map[string]interface{}{"submission": nil})
		return
	}
	jsonOK(w, map[string]interface{}{"submission": sub})
}

func (h *HackathonHandler) Ranking(w http.ResponseWriter, r *http.Request) {
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	list, err := h.repo.ComputeRanking(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "ranking failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"ranking": list})
}

func (h *HackathonHandler) WinnersPreview(w http.ResponseWriter, r *http.Request) {
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
	svc := h.ensureSvc()
	results, err := svc.AutoWinners(r.Context(), hackID, 3)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "preview failed", err)
		return
	}
	ranking, _ := h.repo.ComputeRanking(r.Context(), hackID)
	jsonOK(w, map[string]interface{}{"winners": results, "ranking": ranking})
}

func (h *HackathonHandler) ConfirmResults(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	hc, _ := h.repo.Get(r.Context(), hackID)
	if hc == nil || hc.OrganizerID != claims.UserID || hc.ResultsLocked {
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
	if err := h.repo.LockResults(r.Context(), hackID, claims.UserID); err != nil {
		RespondError(w, http.StatusInternalServerError, "confirm failed", err)
		return
	}
	certURLs, _ := h.generateCertificates(r.Context(), hc, results)
	if err := h.repo.PublishResultsFinal(r.Context(), hackID, results, certURLs); err != nil {
		RespondError(w, http.StatusInternalServerError, "publish failed", err)
		return
	}
	for _, res := range results {
		recipients := []uuid.UUID{}
		if res.StudentID != nil {
			recipients = append(recipients, *res.StudentID)
		} else if res.TeamID != nil {
			recipients, _ = h.repo.TeamMemberIDs(r.Context(), *res.TeamID)
		}
		share := res.PrizeAmountKZT
		if len(recipients) > 0 && res.TeamID != nil {
			share = res.PrizeAmountKZT / float64(len(recipients))
		}
		for _, studentID := range recipients {
			if share > 0 {
				_ = h.billingRepo.CreditWallet(r.Context(), studentID, share)
			}
			h.notifier.Notify(r.Context(), studentID, "hackathon_result", fmt.Sprintf("%d место на хакатоне", res.Place),
				fmt.Sprintf("Ваш результат опубликован. Приз: %.0f ₸", share), map[string]interface{}{"hackathon_id": hackID.String(), "place": res.Place})
			if res.InternshipOffer {
				h.notifier.Notify(r.Context(), studentID, "hackathon_internship_offer", "Приглашение на стажировку",
					"Организатор хакатона приглашает вас рассмотреть стажировку", map[string]interface{}{"hackathon_id": hackID.String()})
			}
		}
	}
	jsonOK(w, map[string]string{"status": "completed"})
}

func (h *HackathonHandler) OpenEvaluation(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	if err := h.repo.OpenEvaluation(r.Context(), hackID, claims.UserID); err != nil {
		http.Error(w, `{"error":"failed"}`, http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "results_pending"})
}

func (h *HackathonHandler) Stats(w http.ResponseWriter, r *http.Request) {
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
	st, err := h.repo.OrganizerStats(r.Context(), hackID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "stats failed", err)
		return
	}
	jsonOK(w, st)
}

func (h *HackathonHandler) SubmitJuryScores(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	hackID, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	juryID, err := h.repo.IsJuryMember(r.Context(), hackID, claims.UserID)
	isOrganizer := false
	hc, _ := h.repo.Get(r.Context(), hackID)
	if hc != nil && hc.OrganizerID == claims.UserID {
		isOrganizer = true
	}
	if err != nil && !isOrganizer {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Scores []struct {
			SubmissionID string  `json:"submission_id"`
			CriterionID  string  `json:"criterion_id"`
			Score        float64 `json:"score"`
			Comment      string  `json:"comment"`
		} `json:"scores"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	for _, s := range req.Scores {
		subID, _ := uuid.Parse(s.SubmissionID)
		critID, _ := uuid.Parse(s.CriterionID)
		_ = h.repo.UpsertJuryScore(r.Context(), hackID, subID, critID, juryID, s.Score, s.Comment)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *HackathonHandler) generateCertificates(ctx context.Context, hc *model.Hackathon, results []model.HackathonResult) (map[uuid.UUID]string, error) {
	urls := make(map[uuid.UUID]string)
	if h.storage == nil {
		return urls, nil
	}
	title := h.decryptField(hc.TitleEnc)
	winnerPlaces := map[uuid.UUID]int{}
	for _, res := range results {
		if res.StudentID != nil {
			winnerPlaces[*res.StudentID] = res.Place
		}
	}
	ids, _ := h.repo.ApprovedStudentIDs(ctx, hc.ID)
	for _, sid := range ids {
		label := "Certificate of Participation"
		if place, ok := winnerPlaces[sid]; ok {
			label = fmt.Sprintf("Winner Diploma — Place %d", place)
		}
		pdf, err := hackathoncert.GenerateSimplePDF(sid.String(), title, hc.OrganizerID.String(), label)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("certificates/%s/%s.pdf", hc.ID, sid)
		if err := h.storage.Upload(ctx, key, bytes.NewReader(pdf), "application/pdf"); err != nil {
			continue
		}
		if u, err := h.storage.GetPresignedURL(ctx, key); err == nil {
			urls[sid] = u
		} else {
			urls[sid] = "s3://" + key
		}
	}
	return urls, nil
}
