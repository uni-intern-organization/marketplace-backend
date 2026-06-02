package handler

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/billing"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

type PortfolioHandler struct {
	appRepo       *repository.ApplicationRepository
	freelanceRepo *repository.FreelanceRepository
	hackathonRepo *repository.HackathonRepository
	billingSvc    *billing.Service
}

func NewPortfolioHandler(
	appRepo *repository.ApplicationRepository,
	freelanceRepo *repository.FreelanceRepository,
	hackathonRepo *repository.HackathonRepository,
	billingSvc *billing.Service,
) *PortfolioHandler {
	return &PortfolioHandler{
		appRepo: appRepo, freelanceRepo: freelanceRepo, hackathonRepo: hackathonRepo, billingSvc: billingSvc,
	}
}

func (h *PortfolioHandler) GetStudentPortfolio(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	studentIDStr := r.PathValue("id")
	if studentIDStr == "" {
		studentIDStr = r.URL.Query().Get("id")
	}
	studentID, err := uuid.Parse(studentIDStr)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	if claims != nil && claims.Role == model.RoleStudent && claims.UserID != studentID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	if claims != nil && (claims.Role == model.RoleRecruiter) {
		ent, err := h.billingSvc.GetRecruiterEntitlements(r.Context(), claims.UserID, claims.Role)
		if err != nil || !ent.CanSearch {
			billing.WriteError(w, http.StatusForbidden, "subscription_required", "portfolio view for recruiters requires Pro")
			return
		}
	}

	apps, _ := h.appRepo.ListByStudent(r.Context(), studentID)
	internships := make([]map[string]interface{}, 0)
	for _, a := range apps {
		if a.Status == "hired" || a.Status == "accepted" || a.Status == "completed" {
			internships = append(internships, map[string]interface{}{
				"application_id": a.ID.String(),
				"status":         a.Status,
				"vacancy_id":     nilStringUUID(a.VacancyID),
			})
		}
	}
	freelance, _ := h.freelanceRepo.PortfolioForStudent(r.Context(), studentID)
	achievements, _ := h.hackathonRepo.PortfolioForStudent(r.Context(), studentID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"student_id":  studentID.String(),
		"internships": internships,
		"freelance":   freelance,
		"achievements": achievements,
	})
}

func nilStringUUID(id *uuid.UUID) interface{} {
	if id == nil {
		return nil
	}
	return id.String()
}
