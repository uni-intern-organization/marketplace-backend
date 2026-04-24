package ai

import (
	"unicode/utf8"
)

// SplitText splits s into overlapping segments of at most maxRunes runes (rough size control for embedding limits).
func SplitText(s string, maxRunes, overlapRunes int) []string {
	if maxRunes < 200 {
		maxRunes = 200
	}
	if overlapRunes < 0 {
		overlapRunes = 0
	}
	if overlapRunes >= maxRunes {
		overlapRunes = maxRunes / 4
	}
	s = trimSpaceRunes(s)
	if s == "" {
		return nil
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return []string{s}
	}
	var out []string
	start := 0
	runes := []rune(s)
	n := len(runes)
	for start < n {
		end := start + maxRunes
		if end > n {
			end = n
		}
		chunk := string(runes[start:end])
		chunk = trimSpaceRunes(chunk)
		if chunk != "" {
			out = append(out, chunk)
		}
		if end >= n {
			break
		}
		advance := maxRunes - overlapRunes
		if advance < 1 {
			advance = 1
		}
		start += advance
	}
	return out
}

func trimSpaceRunes(s string) string {
	// fast path: trim common ASCII space
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c == ' ' || c == '\n' || c == '\r' || c == '\t' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}
