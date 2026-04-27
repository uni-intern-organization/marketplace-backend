// Package careerintel implements the intelligence layer: normalized scores (0–100) and skill gap analysis.
package careerintel

import (
	"strings"
)

// SplitSkillTokens splits comma-separated skills into trimmed non-empty tokens (original casing preserved per token).
func SplitSkillTokens(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// SkillGap compares required_skills tokens against a haystack (student skills field or full resume lowercased).
// matched = required tokens found as substrings in haystack; missing = required tokens not found.
func SkillGap(requiredSkills, haystackLower string) (matched, missing []string) {
	haystackLower = strings.ToLower(strings.TrimSpace(haystackLower))
	var reqTokens []string
	for _, p := range SplitSkillTokens(requiredSkills) {
		reqTokens = append(reqTokens, strings.TrimSpace(p))
	}
	if len(reqTokens) == 0 {
		return nil, nil
	}
	found := make(map[string]struct{})
	for _, tok := range reqTokens {
		tl := strings.ToLower(tok)
		if tl == "" {
			continue
		}
		if strings.Contains(haystackLower, tl) {
			matched = append(matched, tok)
			found[tl] = struct{}{}
		}
	}
	for _, tok := range reqTokens {
		tl := strings.ToLower(tok)
		if tl == "" {
			continue
		}
		if _, ok := found[tl]; !ok {
			missing = append(missing, tok)
		}
	}
	return matched, missing
}

// Score0to100Relative maps batch-relative match to 0–100 (same as UI “% vs best in catalog”).
func Score0to100Relative(score, maxScoreInBatch int) int {
	if maxScoreInBatch <= 0 {
		return 0
	}
	p := (100 * score) / maxScoreInBatch
	if p > 100 {
		return 100
	}
	if p < 0 {
		return 0
	}
	return p
}

// LearnNext caps missing skills for API payloads (prompt + UI).
func LearnNext(missing []string, maxN int) []string {
	if maxN <= 0 || len(missing) == 0 {
		return nil
	}
	if len(missing) <= maxN {
		out := make([]string, len(missing))
		copy(out, missing)
		return out
	}
	return append([]string(nil), missing[:maxN]...)
}
