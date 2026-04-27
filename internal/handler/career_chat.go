package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/uni-intern-organization/marketplace-backend/internal/ai"
	"github.com/uni-intern-organization/marketplace-backend/internal/careerintel"
	"github.com/uni-intern-organization/marketplace-backend/internal/crypto"
	"github.com/uni-intern-organization/marketplace-backend/internal/middleware"
	"github.com/uni-intern-organization/marketplace-backend/internal/model"
	"github.com/uni-intern-organization/marketplace-backend/internal/repository"
)

const (
	maxCareerChatMessages = 24
	maxCareerContentRunes = 8000
	chatTopVacancies      = 10
	// Карточки под кнопками: только относительно сильные совпадения
	chatCardMinMatchPct = 38
	chatCardMaxItems    = 3
)

type scoredVacancy struct {
	v     model.Vacancy
	score int
}

// CareerChatHandler powers POST /api/chat/career (authenticated career assistant).
type CareerChatHandler struct {
	Client        *ai.Client
	UserRepo      *repository.UserRepository
	RecruiterRepo *repository.RecruiterProfileRepository
	VacancyRepo   *repository.VacancyRepository
	AES           []byte
}

// NewCareerChatHandler constructs handler; recruiterRepo may be used for RoleRecruiter context.
func NewCareerChatHandler(c *ai.Client, u *repository.UserRepository, rec *repository.RecruiterProfileRepository, v *repository.VacancyRepository, aes []byte) *CareerChatHandler {
	return &CareerChatHandler{Client: c, UserRepo: u, RecruiterRepo: rec, VacancyRepo: v, AES: aes}
}

type careerChatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	VacancyID string `json:"vacancy_id,omitempty"`
}

// careerVacancyCard is one actionable row for the SPA (open_path → /jobs/:id).
type careerVacancyCard struct {
	VacancyID       string   `json:"vacancy_id"`
	Title           string   `json:"title"`
	CompanyName     string   `json:"company_name"`
	EmploymentType  string   `json:"employment_type,omitempty"`
	Location        string   `json:"location,omitempty"`
	MatchPercent    int      `json:"match_percent"`      // относительно лучшего в каталоге (legacy)
	MatchScore      int      `json:"match_score"`        // сырой балл матчинга
	MatchScore100   int      `json:"match_score_100"`    // 0–100 тот же смысл что match_percent
	MatchedSkills   []string `json:"matched_skills,omitempty"`
	MissingSkills   []string `json:"missing_skills,omitempty"`
	LearnNext       []string `json:"learn_next,omitempty"` // что подтянуть (пробелы по навыкам)
	FitSummary      string   `json:"fit_summary,omitempty"`
	OpenPath        string   `json:"open_path"`
}

type careerChatResponse struct {
	Reply     string                `json:"reply"`
	Vacancies []careerVacancyCard   `json:"vacancies,omitempty"`
	Error     string                `json:"error,omitempty"`
}

type studentVacancyRanking struct {
	profile *model.StudentProfile
	scored  []scoredVacancy
	maxSc   int
}

func trimRunes(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// Chat handles POST /api/chat/career.
func (h *CareerChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if h == nil || h.Client == nil || strings.TrimSpace(h.Client.APIKey) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(careerChatResponse{Error: "career chat unavailable (OPENAI_API_KEY)"})
		return
	}

	var in careerChatRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if len(in.Messages) == 0 {
		http.Error(w, `{"error":"messages required"}`, http.StatusBadRequest)
		return
	}
	if len(in.Messages) > maxCareerChatMessages {
		http.Error(w, `{"error":"too many messages"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var rank *studentVacancyRanking
	if claims.Role == model.RoleStudent {
		rank = h.computeStudentVacancyRanking(ctx, claims)
	}

	var vacancyCards []careerVacancyCard
	switch claims.Role {
	case model.RoleStudent:
		vacancyCards = h.studentVacancyCards(rank)
	case model.RoleRecruiter:
		vacancyCards = h.recruiterVacancyCards(ctx, claims)
	case model.RoleAdmin:
		vacancyCards = h.adminVacancyCards(ctx)
	}

	sys := h.buildSystemPrompt(r, claims, in.VacancyID, rank, vacancyCards)

	var conv []ai.ChatMessage
	conv = append(conv, ai.ChatMessage{Role: "system", Content: sys})
	for _, m := range in.Messages {
		role := strings.TrimSpace(strings.ToLower(m.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := trimRunes(strings.TrimSpace(m.Content), maxCareerContentRunes)
		if content == "" {
			continue
		}
		conv = append(conv, ai.ChatMessage{Role: role, Content: content})
	}
	if len(conv) < 2 {
		http.Error(w, `{"error":"no user messages"}`, http.StatusBadRequest)
		return
	}

	reply, err := h.Client.ChatCompletion(r.Context(), conv, 0.65, 2048)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(careerChatResponse{Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(careerChatResponse{
		Reply:     strings.TrimSpace(reply),
		Vacancies: vacancyCards,
	})
}

func (h *CareerChatHandler) buildSystemPrompt(r *http.Request, claims *middleware.ClaimsContext, vacancyIDStr string, rank *studentVacancyRanking, cards []careerVacancyCard) string {
	var b strings.Builder
	b.WriteString(`You are a smart job search assistant on an internship and job marketplace. The backend gives three layers:
Intelligence: match_score_100, matched_skills, missing_skills, learn_next per vacancy.
Decision: suitable vacancies appear as UI cards; other rows may still be listed above for context.
Explanation: your job is to explain fit and learning priorities using that data — never contradict the JSON cards.

LANGUAGE: reply in the same language the user writes (Russian, Kazakh, or English). Never mix languages.

STYLE: warm, direct, human career advisor — not a bot reading a list. No Markdown in replies. No numbered vacancy lists like 1) 2) 3). No vacancy UUIDs, paths, or URLs in the reply text. No hollow praise of the question. No “click here” or link talk.

QUALITY: judge fit honestly from the data; prioritize strong matches; mention weak ones as secondary with clear reasoning or skip them. If nothing fits well, say so and ask what the user is looking for.

When the user asks about jobs, internships, or fit, ground advice in “Matching vacancies from platform” and the intelligence JSON: title, company, scores, matched and missing skills. The app already shows open buttons — do not paste links or ids.

If there are no matches or scores are all zero, explain likely gaps and suggest completing the profile and using the job catalog without sounding generic.

Never invent employers or titles not in the context below. Refuse legal or medical advice.

`)
	b.WriteString("User context:\n")
	b.WriteString("- Role: ")
	b.WriteString(string(claims.Role))
	b.WriteString("\n- Email: ")
	b.WriteString(claims.Email)
	b.WriteString("\n")

	switch claims.Role {
	case model.RoleStudent:
		p, err := h.UserRepo.GetStudentProfileByUserID(r.Context(), claims.UserID)
		if err == nil {
			if len(p.FullNameEnc) > 0 {
				x, _ := crypto.Decrypt(p.FullNameEnc, h.AES)
				b.WriteString("- Name: ")
				b.WriteString(string(x))
				b.WriteString("\n")
			}
			if len(p.BioEnc) > 0 {
				x, _ := crypto.Decrypt(p.BioEnc, h.AES)
				b.WriteString("- Bio: ")
				b.WriteString(string(x))
				b.WriteString("\n")
			}
			b.WriteString("- Skills: ")
			b.WriteString(p.Skills)
			b.WriteString("\n- Education: ")
			b.WriteString(p.Education)
			b.WriteString("\n- Experience years: ")
			b.WriteString(strconv.Itoa(p.ExperienceYears))
			b.WriteString("\n- Location: ")
			b.WriteString(p.Location)
			b.WriteString("\n- Availability: ")
			b.WriteString(p.Availability)
			b.WriteString("\n")
		}
	case model.RoleRecruiter:
		p, err := h.RecruiterRepo.GetByUserID(r.Context(), claims.UserID)
		if err == nil {
			if len(p.CompanyNameEnc) > 0 {
				x, _ := crypto.Decrypt(p.CompanyNameEnc, h.AES)
				b.WriteString("- Company: ")
				b.WriteString(string(x))
				b.WriteString("\n")
			}
			if len(p.FullNameEnc) > 0 {
				x, _ := crypto.Decrypt(p.FullNameEnc, h.AES)
				b.WriteString("- Name: ")
				b.WriteString(string(x))
				b.WriteString("\n")
			}
		}
	case model.RoleAdmin:
		b.WriteString("- Platform administrator.\n")
	}

	switch claims.Role {
	case model.RoleStudent:
		h.appendStudentVacancyMatchesFromRanking(&b, rank)
	case model.RoleRecruiter:
		h.appendRecruiterOwnVacancies(r.Context(), &b, claims)
	case model.RoleAdmin:
		h.appendAdminVacancySnapshot(r.Context(), &b)
	}

	if claims.Role == model.RoleStudent && len(cards) > 0 {
		h.appendIntelligenceDecisionJSON(&b, cards)
	}

	vacancyIDStr = strings.TrimSpace(vacancyIDStr)
	if vacancyIDStr != "" && h.VacancyRepo != nil {
		id, err := uuid.Parse(vacancyIDStr)
		if err == nil {
			v, err := h.VacancyRepo.GetByID(r.Context(), id)
			if err == nil {
				title := ""
				desc := ""
				if len(v.TitleEnc) > 0 {
					t, _ := crypto.Decrypt(v.TitleEnc, h.AES)
					title = string(t)
				}
				if len(v.DescriptionEnc) > 0 {
					d, _ := crypto.Decrypt(v.DescriptionEnc, h.AES)
					desc = string(d)
				}
				b.WriteString(fmt.Sprintf("\nFocused vacancy (id=%s):\nTitle: %s\nCompany: %s\nSkills: %s\nLocation: %s\nEmployment: %s\nDescription excerpt: %s\n",
					id.String(), title, v.CompanyName, v.RequiredSkills, v.Location, v.EmploymentType, trimRunes(desc, 4000)))
			}
		}
	}

	return b.String()
}

func (h *CareerChatHandler) computeStudentVacancyRanking(ctx context.Context, claims *middleware.ClaimsContext) *studentVacancyRanking {
	if h.VacancyRepo == nil || h.UserRepo == nil {
		return nil
	}
	profile, err := h.UserRepo.GetStudentProfileByUserID(ctx, claims.UserID)
	if err != nil {
		return nil
	}
	list, err := h.VacancyRepo.List(ctx, repository.VacancyFilter{}, 100)
	if err != nil || len(list) == 0 {
		return &studentVacancyRanking{profile: profile, scored: nil, maxSc: 0}
	}
	scored := make([]scoredVacancy, 0, len(list))
	for _, v := range list {
		scored = append(scored, scoredVacancy{
			v: v,
			score: matchScore(v.RequiredSkills, profile.Skills, v.Location, profile.Location, v.EmploymentType, profile.Availability,
				v.MinExperienceYears, profile.ExperienceYears),
		})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	maxSc := 0
	if len(scored) > 0 {
		maxSc = scored[0].score
	}
	return &studentVacancyRanking{profile: profile, scored: scored, maxSc: maxSc}
}

func (h *CareerChatHandler) appendStudentVacancyMatchesFromRanking(b *strings.Builder, rank *studentVacancyRanking) {
	if rank == nil || rank.profile == nil {
		b.WriteString("\nMatching vacancies from platform:\n(no student profile — fill profile for personalized matches)\n")
		return
	}
	if len(rank.scored) == 0 {
		b.WriteString("\nMatching vacancies from platform:\n(no vacancies in catalog)\n")
		return
	}
	profile := rank.profile
	scored := rank.scored
	maxSc := rank.maxSc
	n := chatTopVacancies
	if n > len(scored) {
		n = len(scored)
	}
	haystack := h.studentProfileHaystack(profile)
	b.WriteString("\nMatching vacancies from platform (score = same heuristic as GET /api/match/recommendations; UI opens /jobs/<id>):\n")
	for i := 0; i < n; i++ {
		s := scored[i]
		vr := vacancyToResponse(&s.v, h.AES)
		pct := careerintel.Score0to100Relative(s.score, maxSc)
		matched, missing := careerintel.SkillGap(s.v.RequiredSkills, haystack)
		overlapStr := strings.Join(matched, ", ")
		if overlapStr == "" {
			overlapStr = "(no substring match skills/bio ↔ required_skills)"
		}
		gapStr := strings.Join(missing, ", ")
		if len([]rune(gapStr)) > 120 {
			gapStr = string([]rune(gapStr)[:117]) + "…"
		}
		locNote := ""
		if s.v.Location != "" && profile.Location != "" &&
			strings.EqualFold(strings.TrimSpace(s.v.Location), strings.TrimSpace(profile.Location)) {
			locNote = " | location matches profile"
		}
		b.WriteString(fmt.Sprintf("%d) id=%s | %s — %s | match_score=%d (~%d%% vs best) | matched: %s | skill_gaps: %s%s | employment=%s\n",
			i+1, vr.ID, vr.Title, vr.CompanyName, s.score, pct, overlapStr, gapStr, locNote, vr.EmploymentType))
	}
	if maxSc == 0 {
		b.WriteString("(All scores are 0 — strengthen Skills/location/availability to align with postings, then revisit.)\n")
	}
}

func (h *CareerChatHandler) studentProfileHaystack(p *model.StudentProfile) string {
	if p == nil {
		return ""
	}
	var parts []string
	parts = append(parts, p.Skills, p.Education)
	if len(p.BioEnc) > 0 && len(h.AES) > 0 {
		if b, err := crypto.Decrypt(p.BioEnc, h.AES); err == nil {
			parts = append(parts, string(b))
		}
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

func (h *CareerChatHandler) appendIntelligenceDecisionJSON(b *strings.Builder, cards []careerVacancyCard) {
	type row struct {
		VacancyID     string   `json:"vacancy_id"`
		MatchScore100 int      `json:"match_score_100"`
		MatchedSkills []string `json:"matched_skills"`
		MissingSkills []string `json:"missing_skills"`
		LearnNext     []string `json:"learn_next"`
		FitSummary    string   `json:"fit_summary,omitempty"`
	}
	rows := make([]row, 0, len(cards))
	for _, c := range cards {
		rows = append(rows, row{
			VacancyID:     c.VacancyID,
			MatchScore100: c.MatchScore100,
			MatchedSkills: c.MatchedSkills,
			MissingSkills: c.MissingSkills,
			LearnNext:     c.LearnNext,
			FitSummary:    c.FitSummary,
		})
	}
	raw, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return
	}
	b.WriteString("\n### INTELLIGENCE_JSON (UI cards = decision layer; use for explanations)\n")
	b.Write(raw)
	b.WriteString("\n")
}

func studentVacancyFitSummary(profile *model.StudentProfile, v *model.Vacancy, pct int, matchedTokens []string, locMatch bool) string {
	if profile == nil || v == nil {
		return ""
	}
	if len(matchedTokens) > 0 {
		s := strings.Join(matchedTokens, ", ")
		if len([]rune(s)) > 100 {
			r := []rune(s)
			s = string(r[:97]) + "…"
		}
		return fmt.Sprintf("Подходит: есть пересечения по навыкам (%s). Относительный матч ~%d%% к лучшему в каталоге.", s, pct)
	}
	if locMatch {
		return fmt.Sprintf("Подходит частично: локация совпадает; добавьте в профиль ключевые слова из «требуемых навыков» (~%d%% к лучшему).", pct)
	}
	return fmt.Sprintf("Из доступных позиций — один из более удачных по профилю (~%d%% к лучшему); усильте навыки под эту вакансию.", pct)
}

func (h *CareerChatHandler) studentVacancyCards(rank *studentVacancyRanking) []careerVacancyCard {
	if rank == nil || rank.profile == nil || len(rank.scored) == 0 {
		return nil
	}
	profile := rank.profile
	haystack := h.studentProfileHaystack(profile)
	out := make([]careerVacancyCard, 0, chatCardMaxItems)
	for i := 0; i < len(rank.scored); i++ {
		if len(out) >= chatCardMaxItems {
			break
		}
		s := rank.scored[i]
		pct := careerintel.Score0to100Relative(s.score, rank.maxSc)
		if pct < chatCardMinMatchPct {
			continue
		}
		vr := vacancyToResponse(&s.v, h.AES)
		matched, missing := careerintel.SkillGap(s.v.RequiredSkills, haystack)
		learn := careerintel.LearnNext(missing, 8)
		locMatch := s.v.Location != "" && profile.Location != "" &&
			strings.EqualFold(strings.TrimSpace(s.v.Location), strings.TrimSpace(profile.Location))
		fit := studentVacancyFitSummary(profile, &s.v, pct, matched, locMatch)
		out = append(out, careerVacancyCard{
			VacancyID:       vr.ID,
			Title:           vr.Title,
			CompanyName:     vr.CompanyName,
			EmploymentType:  vr.EmploymentType,
			Location:        vr.Location,
			MatchPercent:    pct,
			MatchScore:      s.score,
			MatchScore100:   pct,
			MatchedSkills:   matched,
			MissingSkills:   missing,
			LearnNext:       learn,
			FitSummary:      fit,
			OpenPath:        "/jobs/" + vr.ID,
		})
	}
	return out
}

func (h *CareerChatHandler) recruiterVacancyCards(ctx context.Context, claims *middleware.ClaimsContext) []careerVacancyCard {
	if h.VacancyRepo == nil {
		return nil
	}
	list, err := h.VacancyRepo.ListByRecruiter(ctx, claims.UserID)
	if err != nil || len(list) == 0 {
		return nil
	}
	cap := chatTopVacancies
	if len(list) < cap {
		cap = len(list)
	}
	out := make([]careerVacancyCard, 0, cap)
	for i := range list {
		if len(out) >= chatCardMaxItems {
			break
		}
		v := list[i]
		vr := vacancyToResponse(&v, h.AES)
		out = append(out, careerVacancyCard{
			VacancyID:      vr.ID,
			Title:          vr.Title,
			CompanyName:    vr.CompanyName,
			EmploymentType: vr.EmploymentType,
			Location:       vr.Location,
			MatchPercent:   0,
			MatchScore:     0,
			OpenPath:       "/jobs/" + vr.ID,
		})
	}
	return out
}

func (h *CareerChatHandler) adminVacancyCards(ctx context.Context) []careerVacancyCard {
	if h.VacancyRepo == nil {
		return nil
	}
	list, err := h.VacancyRepo.List(ctx, repository.VacancyFilter{}, 15)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]careerVacancyCard, 0, chatCardMaxItems)
	for i := range list {
		if len(out) >= chatCardMaxItems {
			break
		}
		v := list[i]
		vr := vacancyToResponse(&v, h.AES)
		out = append(out, careerVacancyCard{
			VacancyID:      vr.ID,
			Title:          vr.Title,
			CompanyName:    vr.CompanyName,
			EmploymentType: vr.EmploymentType,
			Location:       vr.Location,
			MatchPercent:   0,
			MatchScore:     0,
			OpenPath:       "/jobs/" + vr.ID,
		})
	}
	return out
}

func (h *CareerChatHandler) appendRecruiterOwnVacancies(ctx context.Context, b *strings.Builder, claims *middleware.ClaimsContext) {
	if h.VacancyRepo == nil {
		return
	}
	list, err := h.VacancyRepo.ListByRecruiter(ctx, claims.UserID)
	if err != nil || len(list) == 0 {
		b.WriteString("\nYour company's vacancies on the platform:\n(none yet — create one from recruiter dashboard)\n")
		return
	}
	b.WriteString("\nYour company's vacancies on the platform (reference by id when advising candidates):\n")
	for i, v := range list {
		if i >= chatTopVacancies {
			break
		}
		vr := vacancyToResponse(&v, h.AES)
		b.WriteString(fmt.Sprintf("%d) id=%s | %s | %s | skills=%s | location=%s\n",
			i+1, vr.ID, vr.Title, vr.CompanyName, vr.RequiredSkills, vr.Location))
	}
}

func (h *CareerChatHandler) appendAdminVacancySnapshot(ctx context.Context, b *strings.Builder) {
	if h.VacancyRepo == nil {
		return
	}
	list, err := h.VacancyRepo.List(ctx, repository.VacancyFilter{}, 15)
	if err != nil || len(list) == 0 {
		b.WriteString("\nRecent vacancies (catalog snapshot):\n(empty)\n")
		return
	}
	b.WriteString("\nRecent vacancies (catalog snapshot — use only these ids/titles when discussing openings):\n")
	for i, v := range list {
		vr := vacancyToResponse(&v, h.AES)
		b.WriteString(fmt.Sprintf("%d) id=%s | %s @ %s\n", i+1, vr.ID, vr.Title, vr.CompanyName))
	}
}
