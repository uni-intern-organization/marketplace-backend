package rag

import "fmt"

// VacancyPick is one actionable row for the SPA (/jobs/:id).
type VacancyPick struct {
	VacancyID       string   `json:"vacancy_id"`
	Title           string   `json:"title"`
	CompanyName     string   `json:"company_name"`
	EmploymentType  string   `json:"employment_type,omitempty"`
	Location        string   `json:"location,omitempty"`
	MatchPercent    int      `json:"match_percent"`
	MatchScore100   int      `json:"match_score_100"`
	CombinedScore   float64  `json:"combined_score,omitempty"`
	MatchedSkills   []string `json:"matched_skills,omitempty"`
	MissingSkills   []string `json:"missing_skills,omitempty"`
	LearnNext       []string `json:"learn_next,omitempty"`
	FitSummary      string   `json:"fit_summary,omitempty"`
	OpenPath        string   `json:"open_path"` // always "/jobs/<uuid>"
}

// ragPickMinPct — в карточках под кнопками только достаточно сильный комбинированный матч.
const ragPickMinPct = 38

// VacancyPickFitSummary коротко объясняет, почему строка попала в подборку.
func VacancyPickFitSummary(keyword float64, matchPct int) string {
	if keyword >= 0.42 {
		return fmt.Sprintf("Подходит: заметное пересечение резюме с требуемыми навыками (матч ~%d%%).", matchPct)
	}
	if keyword >= 0.22 {
		return fmt.Sprintf("Подходит частично: смысл близок, по ключевым словам можно усилить резюме (матч ~%d%%).", matchPct)
	}
	return fmt.Sprintf("Смысловое сходство с вакансией; точных совпадений по навыкам мало (матч ~%d%%).", matchPct)
}
