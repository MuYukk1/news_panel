package ai

import (
	"strings"
	"testing"

	"news_panel/internal/triage"
)

func TestExtractJSONObject(t *testing.T) {
	triageJSON := `{"domain":"ai","score":80,"tags":["x"],"summary":"摘要","priority":"normal","keep":true}`
	cases := []struct {
		name string
		text string
		want string
	}{
		{"pure", triageJSON, triageJSON},
		{"fenced json", "```json\n" + triageJSON + "\n```", triageJSON},
		{"fenced plain", "```\n" + triageJSON + "\n```", triageJSON},
		{"prose wrapped", "好的，这是结果：\n" + triageJSON + "\n希望有帮助！", triageJSON},
		{"braces inside string", `{"domain":"ai","score":80,"summary":"use {braces} here","priority":"normal","keep":true}`, `{"domain":"ai","score":80,"summary":"use {braces} here","priority":"normal","keep":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractJSONObject(tc.text)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestExtractJSONPrefersTriageObject(t *testing.T) {
	text := `metadata {"foo":1} then verdict {"domain":"deals","score":60,"tags":[],"summary":"s","priority":"normal","keep":true}`
	got, err := extractJSONObject(text)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !strings.Contains(got, `"domain"`) {
		t.Errorf("should prefer object with domain/score: %q", got)
	}
}

func TestExtractJSONInvalid(t *testing.T) {
	for _, text := range []string{"", "no json here", "{broken", "plain text only"} {
		if _, err := extractJSONObject(text); err == nil {
			t.Errorf("expected error for %q", text)
		}
	}
}

func TestParseVerdictMissingScore(t *testing.T) {
	if _, err := parseVerdict(`{"domain":"ai","keep":true}`); err == nil {
		t.Errorf("missing score should be a parse failure (triggers fallback)")
	}
}

func TestParseVerdictFloatScore(t *testing.T) {
	v, err := parseVerdict(`{"domain":"ai","score":85.6,"keep":true}`)
	if err != nil {
		t.Fatalf("float score should decode: %v", err)
	}
	if v.Score == nil || *v.Score != 85.6 {
		t.Errorf("score: %+v", v.Score)
	}
}

func l1Base() triage.Result {
	return triage.Result{Domain: "investing", AITags: []string{}, AISummary: "L1 兜底摘要", Priority: "normal", TriageStatus: "kept"}
}

func TestNormalizeVerdictClampAndDefaults(t *testing.T) {
	cases := []struct {
		name         string
		score        float64
		domain       string
		priority     string
		wantScore    int
		wantDomain   string
		wantPriority string
	}{
		{"negative clamp", -20, "ai", "normal", 0, "ai", "normal"},
		{"over clamp", 250, "deals", "high", 100, "deals", "high"},
		{"round float", 85.6, "ai", "normal", 86, "ai", "normal"},
		{"unknown domain", 50, "crypto", "normal", 50, "investing", "normal"},
		{"unknown priority", 50, "ai", "urgent", 50, "ai", "normal"},
		{"case normalize", 50, "AI", "HIGH", 50, "ai", "high"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.score
			v := verdict{Domain: tc.domain, Score: &s, Priority: tc.priority}
			got := normalizeVerdict(v, l1Base(), 45)
			assertValid(t, got)
			if got.AIScore == nil || *got.AIScore != tc.wantScore {
				t.Errorf("score=%v want %d", got.AIScore, tc.wantScore)
			}
			if got.Domain != tc.wantDomain || got.Priority != tc.wantPriority {
				t.Errorf("domain/priority=%q/%q want %q/%q", got.Domain, got.Priority, tc.wantDomain, tc.wantPriority)
			}
		})
	}
}

func TestNormalizeVerdictKeepThreshold(t *testing.T) {
	mk := func(score float64, keep *bool, priority string) verdict {
		return verdict{Domain: "ai", Score: &score, Keep: keep, Priority: priority}
	}
	yes, no := true, false
	cases := []struct {
		name       string
		v          verdict
		wantStatus string
	}{
		{"keep false high score", mk(90, &no, "normal"), "filtered"},
		{"below threshold normal", mk(30, &yes, "normal"), "filtered"},
		{"below threshold high kept", mk(30, &yes, "high"), "kept"},
		{"below threshold critical kept", mk(30, &yes, "critical"), "kept"},
		{"above threshold kept", mk(60, &yes, "normal"), "kept"},
		{"keep absent infer above", mk(60, nil, "normal"), "kept"},
		{"keep absent infer below", mk(20, nil, "normal"), "filtered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeVerdict(tc.v, l1Base(), 45)
			assertValid(t, got)
			if got.TriageStatus != tc.wantStatus {
				t.Errorf("status=%q want %q", got.TriageStatus, tc.wantStatus)
			}
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	got := normalizeTags([]string{" AI ", "ai", "OpenAI", "", "  ", "t1", "t2", "t3", "t4", "t5", "t6", "t7"})
	if len(got) > 6 {
		t.Errorf("tags must cap at 6: %#v", got)
	}
	if got[0] != "AI" {
		t.Errorf("first tag trimmed: %q", got[0])
	}
	seen := map[string]bool{}
	for _, tag := range got {
		k := strings.ToLower(tag)
		if seen[k] {
			t.Errorf("duplicate tag: %#v", got)
		}
		seen[k] = true
	}
	if normalizeTags(nil) == nil {
		t.Errorf("nil tags must normalize to non-nil slice")
	}
	long := strings.Repeat("长", 50)
	got = normalizeTags([]string{long})
	if len([]rune(got[0])) > 33 {
		t.Errorf("long tag must truncate: %d runes", len([]rune(got[0])))
	}
}

func TestNormalizeVerdictSummaryFallback(t *testing.T) {
	s := 50.0
	got := normalizeVerdict(verdict{Domain: "ai", Score: &s, Summary: "  "}, l1Base(), 45)
	if got.AISummary != "L1 兜底摘要" {
		t.Errorf("empty summary should fall back to L1: %q", got.AISummary)
	}
	got = normalizeVerdict(verdict{Domain: "ai", Score: &s, Summary: strings.Repeat("摘", 600)}, l1Base(), 45)
	if len([]rune(got.AISummary)) > 501 {
		t.Errorf("summary must truncate to ~500 runes, got %d", len([]rune(got.AISummary)))
	}
	threeHundred := strings.Repeat("摘", 300)
	got = normalizeVerdict(verdict{Domain: "ai", Score: &s, Summary: threeHundred}, l1Base(), 45)
	if got.AISummary != threeHundred {
		t.Errorf("300-rune translated summary should remain intact")
	}
}