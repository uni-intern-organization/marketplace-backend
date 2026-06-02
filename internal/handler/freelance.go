package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/uni-intern-organization/marketplace-backend/config"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type FreelanceHandler struct {
	repo        *repository.FreelanceRepository
	billingRepo *repository.BillingRepository
	cfg         *config.BillingConfig
	aesKey      []byte
}

func NewFreelanceHandler(repo *repository.FreelanceRepository, billingRepo *repository.BillingRepository, cfg *config.BillingConfig, aesKey []byte) *FreelanceHandler {
	return &FreelanceHandler{repo: repo, billingRepo: billingRepo, cfg: cfg, aesKey: aesKey}
}

type freelanceTaskResp struct {
	ID            string  `json:"id"`
	RecruiterID   string  `json:"recruiter_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	BudgetKZT     float64 `json:"budget_kzt"`
	Deadline      string  `json:"deadline"`
	RequiredSkills string `json:"required_skills"`
	Status        string  `json:"status"`
	EscrowStatus  string  `json:"escrow_status"`
	CreatedAt     string  `json:"created_at"`
}

func taskToResp(t *model.FreelanceTask, key []byte) freelanceTaskResp {
	r := freelanceTaskResp{
		ID: t.ID.String(), RecruiterID: t.RecruiterID.String(), Category: t.Category,
		BudgetKZT: t.BudgetKZT, Deadline: t.Deadline.Format(time.RFC3339),
		RequiredSkills: t.RequiredSkills, Status: t.Status, EscrowStatus: t.EscrowStatus,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	if len(t.TitleEnc) > 0 {
		b, _ := crypto.Decrypt(t.TitleEnc, key)
		r.Title = string(b)
	}
	if len(t.DescriptionEnc) > 0 {
		b, _ := crypto.Decrypt(t.DescriptionEnc, key)
		r.Description = string(b)
	}
	return r
}

func (h *FreelanceHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var req struct {
		Title          string  `json:"title"`
		Description    string  `json:"description"`
		Category       string  `json:"category"`
		BudgetKZT      float64 `json:"budget_kzt"`
		Deadline       string  `json:"deadline"`
		RequiredSkills string  `json:"required_skills"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" || req.BudgetKZT <= 0 {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	deadline, err := time.Parse(time.RFC3339, req.Deadline)
	if err != nil {
		deadline = time.Now().Add(7 * 24 * time.Hour)
	}
	titleEnc, _ := crypto.Encrypt([]byte(req.Title), h.aesKey)
	descEnc, _ := crypto.Encrypt([]byte(req.Description), h.aesKey)
	if req.Category == "" {
		req.Category = "general"
	}
	t, err := h.repo.CreateTask(r.Context(), claims.UserID, titleEnc, descEnc, req.Category, req.BudgetKZT, deadline, req.RequiredSkills)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "create failed", err)
		return
	}
	_ = h.billingRepo.CreateEscrow(r.Context(), claims.UserID, "freelance_task", t.ID, req.BudgetKZT)
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "freelance_escrow_hold", map[string]interface{}{
		"task_id": t.ID.String(), "amount": req.BudgetKZT, "demo": true,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskToResp(t, h.aesKey))
}

func (h *FreelanceHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			limit = n
		}
	}
	list, err := h.repo.ListOpen(r.Context(), category, limit)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	resp := make([]freelanceTaskResp, 0, len(list))
	for i := range list {
		resp = append(resp, taskToResp(&list[i], h.aesKey))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *FreelanceHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var list []model.FreelanceTask
	var err error
	if claims.Role == model.RoleRecruiter {
		list, err = h.repo.ListByRecruiter(r.Context(), claims.UserID)
	} else if claims.Role == model.RoleStudent {
		list, err = h.repo.ListByStudent(r.Context(), claims.UserID)
	} else {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	resp := make([]freelanceTaskResp, 0, len(list))
	for i := range list {
		resp = append(resp, taskToResp(&list[i], h.aesKey))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *FreelanceHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	t, err := h.repo.GetTask(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(taskToResp(t, h.aesKey))
}

func (h *FreelanceHandler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	taskID, err := uuid.Parse(r.URL.Query().Get("task_id"))
	if err != nil {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct{ Message string `json:"message"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	msgEnc, _ := crypto.Encrypt([]byte(req.Message), h.aesKey)
	p, err := h.repo.CreateProposal(r.Context(), taskID, claims.UserID, msgEnc)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "proposal failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": p.ID.String(), "status": p.Status})
}

func (h *FreelanceHandler) UpdateProposal(w http.ResponseWriter, r *http.Request) {
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
	var req struct{ Status string `json:"status"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	p, err := h.repo.GetProposal(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	t, err := h.repo.GetTask(r.Context(), p.TaskID)
	if err != nil || t.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if req.Status == "accepted" {
		if err := h.repo.AcceptProposal(r.Context(), p.TaskID, p.StudentID); err != nil {
			RespondError(w, http.StatusInternalServerError, "accept failed", err)
			return
		}
	} else {
		_ = h.repo.UpdateProposalStatus(r.Context(), id, req.Status)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *FreelanceHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	taskID, err := uuid.Parse(r.URL.Query().Get("task_id"))
	if err != nil {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		DeliverableKey string `json:"deliverable_key"`
		StudentNote    string `json:"student_note"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, err := h.repo.CreateSubmission(r.Context(), taskID, claims.UserID, req.DeliverableKey, req.StudentNote)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "submission failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": s.ID.String(), "status": s.Status})
}

func (h *FreelanceHandler) UpdateSubmission(w http.ResponseWriter, r *http.Request) {
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
	var req struct{ Status string `json:"status"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	s, err := h.repo.GetSubmission(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	t, _ := h.repo.GetTask(r.Context(), s.TaskID)
	if t == nil || t.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if req.Status == "revision_requested" {
		if err := h.repo.RequestRevision(r.Context(), id); err != nil {
			http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
			return
		}
		_ = h.repo.UpdateTaskStatus(r.Context(), s.TaskID, "in_progress")
	} else {
		if err := h.repo.UpdateSubmissionStatus(r.Context(), id, req.Status); err != nil {
			http.Error(w, `{"error":"update failed"}`, http.StatusInternalServerError)
			return
		}
	}
	if req.Status == "accepted" {
		_ = h.repo.UpdateTaskStatus(r.Context(), s.TaskID, "completed")
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *FreelanceHandler) CompleteTask(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || claims.Role != model.RoleRecruiter {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	taskID, err := uuid.Parse(r.URL.Query().Get("task_id"))
	if err != nil {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}
	t, err := h.repo.GetTask(r.Context(), taskID)
	if err != nil || t.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusNotFound)
		return
	}
	feePct := 10
	if h.cfg != nil && h.cfg.FreelancePlatformFeePercent > 0 {
		feePct = h.cfg.FreelancePlatformFeePercent
	}
	fee := t.BudgetKZT * float64(feePct) / 100
	payout := t.BudgetKZT - fee
	if t.AcceptedStudentID != nil {
		_ = h.billingRepo.CreditWallet(r.Context(), *t.AcceptedStudentID, payout)
	}
	_ = h.billingRepo.ReleaseEscrow(r.Context(), "freelance_task", taskID)
	_ = h.repo.UpdateTaskStatus(r.Context(), taskID, "completed")
	_ = h.billingRepo.InsertEvent(r.Context(), claims.UserID, "freelance_fee", map[string]interface{}{
		"task_id": taskID.String(), "fee_kzt": fee, "payout_kzt": payout, "demo": true,
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "completed", "payout_kzt": payout, "fee_kzt": fee})
}

func (h *FreelanceHandler) CreateDispute(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	taskID, err := uuid.Parse(r.URL.Query().Get("task_id"))
	if err != nil {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct{ Reason string `json:"reason"` }
	_ = json.NewDecoder(r.Body).Decode(&req)
	d, err := h.repo.CreateDispute(r.Context(), taskID, claims.UserID, req.Reason)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "dispute failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": d.ID.String(), "status": d.Status})
}

func (h *FreelanceHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || (claims.Role != model.RoleAdmin && claims.Role != model.RoleModerator) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	id, err := uuid.Parse(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Resolution   string `json:"resolution"`
		FavorStudent bool   `json:"favor_student"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.repo.ResolveDispute(r.Context(), id, claims.UserID, req.Resolution, req.FavorStudent); err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		RespondError(w, http.StatusInternalServerError, "resolve failed", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

func (h *FreelanceHandler) ListDisputes(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil || (claims.Role != model.RoleModerator && claims.Role != model.RoleAdmin) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	list, err := h.repo.ListDisputes(r.Context(), 100)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	jsonOK(w, map[string]interface{}{"disputes": list})
}

func (h *FreelanceHandler) ListProposals(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	taskID, err := uuid.Parse(r.URL.Query().Get("task_id"))
	if err != nil {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}
	t, err := h.repo.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if claims.Role == model.RoleRecruiter && t.RecruiterID != claims.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if claims.Role == model.RoleStudent {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	list, err := h.repo.ListProposalsByTask(r.Context(), taskID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "list failed", err)
		return
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, p := range list {
		msg := ""
		if len(p.MessageEnc) > 0 {
			b, _ := crypto.Decrypt(p.MessageEnc, h.aesKey)
			msg = string(b)
		}
		out = append(out, map[string]interface{}{
			"id": p.ID.String(), "task_id": p.TaskID.String(), "student_id": p.StudentID.String(),
			"message": msg, "status": p.Status, "created_at": p.CreatedAt.Format(time.RFC3339),
		})
	}
	jsonOK(w, map[string]interface{}{"proposals": out})
}

func (h *FreelanceHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	taskID, err := uuid.Parse(r.URL.Query().Get("task_id"))
	if err != nil {
		http.Error(w, `{"error":"task_id required"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Rating < 1 || req.Rating > 5 {
		http.Error(w, `{"error":"rating 1-5 required"}`, http.StatusBadRequest)
		return
	}
	t, err := h.repo.GetTask(r.Context(), taskID)
	if err != nil || t.Status != "completed" {
		http.Error(w, `{"error":"task not completed"}`, http.StatusBadRequest)
		return
	}
	if claims.UserID != t.RecruiterID && (t.AcceptedStudentID == nil || claims.UserID != *t.AcceptedStudentID) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if err := h.repo.CreateReview(r.Context(), taskID, claims.UserID, req.Rating, req.Comment); err != nil {
		RespondError(w, http.StatusInternalServerError, "review failed", err)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
