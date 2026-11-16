package ai

import (
	"strings"
	"testing"
	"time"

	"news_panel/internal/triage"
)

func l1Input(content, title string) triage.Input {
	return triage.Input{
		SourceID: "x-twifork-main", SourceType: "x", ExternalID: "1",
		URL: "https://x.com/user/status/1", Title: title, Content: content,
		PublishedAt: time.Now().UTC(),
	}
}

func defaultL1() *l1Rules {
	return newL1Rules(Config{L1MinContentRunes: 10})
}

func assertValid(t *testing.T, r triage.Result) {
	t.Helper()
	if err := r.Validate(); err != nil {
		t.Fatalf("result failed Validate: %v (%+v)", err, r)
	}
	if r.AITags == nil {
		t.Errorf("AITags must be non-nil")
	}
}

func TestL1ShortContentNoise(t *testing.T) {
	r := defaultL1()
	cases := []struct {
		name    string
		in      triage.Input
		filtered bool
	}{
		{"too short no title no link", l1Input("hi", ""), true},
		{"short but has title", l1Input("hi", "Breaking: something"), false},
		{"short but valuable github link", l1Input("see https://github.com/foo/bar/releases", ""), false},
		{"short social link only", l1Input("look https://t.co/abc", ""), true},
		{"media-only placeholder", l1Input("(media-only tweet)", ""), true},
		{"long enough", l1Input("这是一条足够长度的正常内容信息", ""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.evaluate(tc.in)
			assertValid(t, got)
			if tc.filtered {
				if got.TriageStatus != "filtered" || got.Priority != "low" || got.Domain != "other" {
					t.Errorf("expected noise filtered/low/other, got %+v", got)
				}
				if got.AIScore != nil {
					t.Errorf("noise must not fabricate score")
				}
				if !containsTag(got.AITags, "l1-noise") {
					t.Errorf("noise tags must include l1-noise: %#v", got.AITags)
				}
			} else if got.TriageStatus != "kept" {
				t.Errorf("expected kept, got %+v", got)
			}
		})
	}
}

func TestL1MinRunesZeroDisablesShortFilter(t *testing.T) {
	r := newL1Rules(Config{L1MinContentRunes: 0})
	got := r.evaluate(l1Input("hi", ""))
	assertValid(t, got)
	if got.TriageStatus != "kept" {
		t.Errorf("min runes 0 should disable short filter: %+v", got)
	}
}

func TestL1NoiseKeywordsAndDealsProtection(t *testing.T) {
	r := defaultL1()
	cases := []struct {
		name     string
		content  string
		filtered bool
		domain   string
	}{
		{"follow bait", "互粉互关，回关必回！", true, "other"},
		{"giveaway spam", "转发抽奖关注抽奖送礼物", true, "other"},
		{"ad引流", "加微信领取网赚课程，日赚千元", true, "other"},
		{"deals protected despite 抽奖", "抽奖送一年VPS优惠，薅羊毛速来", false, "deals"},
		{"限免 signal", "限时免费领 Steam 游戏，喜加一", false, "deals"},
		{"折扣码 signal", "云主机折扣码优惠，空投规则更新", false, "deals"},
		{"normal long text", "今天市场整体表现平稳，没有特别的消息", false, "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := r.evaluate(l1Input(tc.content, ""))
			assertValid(t, got)
			if tc.filtered && got.TriageStatus != "filtered" {
				t.Errorf("expected filtered, got %+v", got)
			}
			if !tc.filtered {
				if got.TriageStatus != "kept" {
					t.Errorf("expected kept (deals protection), got %+v", got)
				}
				if got.Domain != tc.domain {
					t.Errorf("domain=%q want %q", got.Domain, tc.domain)
				}
			}
		})
	}
}

func TestL1CustomBlockOverridesDealsProtection(t *testing.T) {
	r := newL1Rules(Config{L1MinContentRunes: 10, L1BlockKeywords: []string{"VPS"}})
	got := r.evaluate(l1Input("VPS优惠折扣码薅羊毛", ""))
	assertValid(t, got)
	if got.TriageStatus != "filtered" {
		t.Errorf("custom block must override deals protection: %+v", got)
	}
	if !containsTag(got.AITags, "l1-block-keyword") {
		t.Errorf("expected l1-block-keyword tag: %#v", got.AITags)
	}
}

func TestL1AllowOverridesBlock(t *testing.T) {
	r := newL1Rules(Config{L1MinContentRunes: 10, L1BlockKeywords: []string{"羊毛"}, L1AllowKeywords: []string{"薅羊毛"}})
	got := r.evaluate(l1Input("薅羊毛线报：限免领取", ""))
	assertValid(t, got)
	if got.TriageStatus != "kept" {
		t.Errorf("allow keyword must bypass block: %+v", got)
	}
}

func TestL1RetweetFilter(t *testing.T) {
	in := l1Input("这是一条足够长度的转推内容信息", "")
	in.RawMeta = map[string]any{"is_retweet": true}

	off := newL1Rules(Config{L1MinContentRunes: 10, L1FilterRetweets: false})
	if got := off.evaluate(in); got.TriageStatus != "kept" {
		t.Errorf("retweet filter off should keep: %+v", got)
	}
	on := newL1Rules(Config{L1MinContentRunes: 10, L1FilterRetweets: true})
	got := on.evaluate(in)
	assertValid(t, got)
	if got.TriageStatus != "filtered" || !containsTag(got.AITags, "l1-retweet") {
		t.Errorf("retweet filter on should filter with tag: %+v", got)
	}
}

func TestL1DomainGuess(t *testing.T) {
	r := defaultL1()
	cases := []struct {
		content string
		domain  string
	}{
		{"美联储宣布降息，CPI 数据超预期，财报季来临", "investing"},
		{"OpenAI 发布 GPT 新大模型，内测泄露", "ai"},
		{"VPS 优惠折扣码，云主机特价薅羊毛", "deals"},
		{"今天天气不错，适合出去散步的感觉呀", "other"},
	}
	for _, tc := range cases {
		got := r.evaluate(l1Input(tc.content, ""))
		assertValid(t, got)
		if got.Domain != tc.domain {
			t.Errorf("content %q: domain=%q want %q", tc.content, got.Domain, tc.domain)
		}
		if got.AIScore != nil {
			t.Errorf("L1 must not fabricate score")
		}
	}
}

func TestL1ASCIIWordBoundary(t *testing.T) {
	r := defaultL1()
	got := r.evaluate(l1Input("I sent an email about the chain details today", ""))
	assertValid(t, got)
	if got.Domain != "other" {
		t.Errorf("'ai' inside email/chain must not trigger ai domain: %q", got.Domain)
	}
	got = r.evaluate(l1Input("New AI model released by the lab today", ""))
	if got.Domain != "ai" {
		t.Errorf("standalone AI should trigger ai domain: %q", got.Domain)
	}
}

func TestL1UrgentPriorityHint(t *testing.T) {
	r := defaultL1()
	got := r.evaluate(l1Input("突发：美联储紧急降息，市场暴跌", ""))
	assertValid(t, got)
	if got.Priority != "high" {
		t.Errorf("urgent content should hint high priority: %q", got.Priority)
	}
	got = r.evaluate(l1Input("今天市场整体表现平稳，没有特别的消息", ""))
	if got.Priority != "normal" {
		t.Errorf("calm content should be normal priority: %q", got.Priority)
	}
}

func TestL1KeptSummaryNonEmpty(t *testing.T) {
	r := defaultL1()
	got := r.evaluate(l1Input("OpenAI 发布新模型，值得关注", ""))
	if strings.TrimSpace(got.AISummary) == "" {
		t.Errorf("kept L1 result must carry a summary")
	}
}

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}