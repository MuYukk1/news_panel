package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"news_panel/internal/triage"
)

var fenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

// verdict is the raw scoring model output before normalization. Score is float64 so
// "85.0"/85.5 style responses do not break decoding; Keep is optional and
// inferred from the score threshold when absent.
type verdict struct {
	Domain   string   `json:"domain"`
	Score    *float64 `json:"score"`
	Tags     []string `json:"tags"`
	Summary  string   `json:"summary"`
	Priority string   `json:"priority"`
	Keep     *bool    `json:"keep"`
}

// extractJSONObject finds the triage JSON object inside pure JSON, ```json
// fences, or prose mixed with JSON. Objects mentioning domain/score are
// preferred so unrelated JSON fragments in prose do not win.
func extractJSONObject(text string) (string, error) {
	return extractPreferredObject(text, `"domain"`, `"score"`)
}

func extractPreferredObject(text, field1, field2 string) (string, error) {
	sources := make([]string, 0, 4)
	for _, m := range fenceRe.FindAllStringSubmatch(text, -1) {
		sources = append(sources, m[1])
	}
	sources = append(sources, text)

	fallback := ""
	for _, src := range sources {
		for i := 0; i < len(src); i++ {
			if src[i] != '{' {
				continue
			}
			obj, ok := balancedObjectAt(src, i)
			if !ok || !json.Valid([]byte(obj)) {
				continue
			}
			if strings.Contains(obj, field1) || strings.Contains(obj, field2) {
				return obj, nil
			}
			if fallback == "" {
				fallback = obj
			}
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", errors.New("no JSON object found in completion")
}

// balancedObjectAt returns the substring from s[start] ('{') to its matching
// '}', respecting JSON string literals and escapes.
func balancedObjectAt(s string, start int) (string, bool) {
	depth := 0
	inString := false
	escaped := false
	for j := start; j < len(s); j++ {
		ch := s[j]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : j+1], true
			}
		}
	}
	return "", false
}

func parseVerdict(text string) (verdict, error) {
	obj, err := extractJSONObject(text)
	if err != nil {
		return verdict{}, err
	}
	var v verdict
	if err := json.Unmarshal([]byte(obj), &v); err != nil {
		return verdict{}, fmt.Errorf("invalid triage JSON: %w", err)
	}
	if v.Score == nil {
		return verdict{}, errors.New("triage JSON missing score")
	}
	return v, nil
}

// normalizeVerdict defensively clamps and canonicalizes every model field and
// applies the keep threshold. The returned Result always passes Validate().
func normalizeVerdict(v verdict, l1 triage.Result, keepMinScore int) triage.Result {
	score := int(math.Round(*v.Score))
	score = clampInt(score, 0, 100)

	domain := strings.ToLower(strings.TrimSpace(v.Domain))
	switch domain {
	case "investing", "ai", "deals", "other":
	default:
		domain = l1.Domain
	}

	priority := strings.ToLower(strings.TrimSpace(v.Priority))
	switch priority {
	case "critical", "high", "normal", "low":
	default:
		priority = "normal"
	}

	summary := strings.TrimSpace(v.Summary)
	if summary == "" {
		summary = l1.AISummary
	}
	summary = truncateRunes(summary, 500)

	keep := score >= keepMinScore
	if v.Keep != nil {
		keep = *v.Keep
	}
	status := "kept"
	if !keep || (score < keepMinScore && priority != "high" && priority != "critical") {
		status = "filtered"
	}

	return triage.Result{
		Domain:       domain,
		AIScore:      &score,
		AITags:       normalizeTags(v.Tags),
		AISummary:    summary,
		Priority:     priority,
		TriageStatus: status,
	}
}

// normalizeTags trims, deduplicates (case-insensitive), shortens and caps tags
// at 6; the result is always non-nil.
func normalizeTags(tags []string) []string {
	out := make([]string, 0, 6)
	seen := make(map[string]bool)
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		t = truncateRunes(t, 32)
		key := strings.ToLower(t)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
		if len(out) == 6 {
			break
		}
	}
	return out
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}
