package ai

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"news_panel/internal/triage"
)

// L1 is a pure-local, sub-millisecond coarse filter. It blocks obvious noise,
// protects real deals signals from keyword false positives, and produces a
// domain/priority guess used both as the no-LLM fallback and as an L2 hint.

var (
	urlRe     = regexp.MustCompile(`https?://\S+|www\.\S+`)
	mentionRe = regexp.MustCompile(`@[A-Za-z0-9_]+`)
)

// socialHosts are low-information link targets for the short-content guard.
var socialHosts = map[string]bool{
	"x.com": true, "twitter.com": true, "mobile.twitter.com": true, "t.co": true,
	"facebook.com": true, "fb.com": true, "instagram.com": true, "tiktok.com": true,
	"weibo.com": true, "weibo.cn": true,
}

var investingKeywords = []string{
	"美股", "a股", "港股", "沪深", "沪指", "创业板", "科创板", "恒生", "纳指", "标普", "道指",
	"大盘", "涨停", "跌停", "熔断", "停牌", "复牌", "财报", "季报", "年报", "业绩", "营收",
	"净利润", "毛利率", "指引", "研报", "目标价", "评级", "增持", "减持", "回购", "分红",
	"派息", "并购", "收购", "要约", "上市", "退市", "证监会", "美联储", "鲍威尔", "非农",
	"失业率", "利率", "加息", "降息", "缩表", "通胀", "通缩", "宏观", "关税", "贸易战",
	"制裁", "出口管制", "催化剂", "牛市", "熊市", "做多", "做空", "期权", "期货", "爆仓",
	"比特币", "以太坊", "加密", "股价", "股票", "股市", "券商", "投行", "高盛", "摩根",
	"巴菲特", "估值", "市盈率", "清算", "违约", "破产",
	"ipo", "sec", "fed", "fomc", "cpi", "ppi", "pce", "gdp", "pmi", "earnings", "eps",
	"revenue", "guidance", "outlook", "upgrade", "downgrade", "buyback", "dividend",
	"merger", "acquisition", "tariff", "sanction", "catalyst", "crypto", "btc", "eth",
	"solana", "tsla", "nvda", "aapl", "msft", "googl", "amzn", "ticker",
}

var aiKeywords = []string{
	"人工智能", "大模型", "通义", "千问", "文心", "豆包", "开源模型", "微调", "蒸馏",
	"量化", "算力", "英伟达", "特斯拉", "机器人", "宇树", "世界模型", "语音模型", "向量",
	"幻觉", "基准", "打榜", "刷榜", "内测", "灰度", "泄露", "提示词", "上下文窗口",
	"智能体", "机器学习", "深度学习", "神经网络", "推理模型", "多模态", "强化学习",
	"扩散模型", "强化", "代理",
	"ai", "llm", "gpt", "chatgpt", "openai", "anthropic", "claude", "gemini", "deepmind",
	"grok", "xai", "llama", "mistral", "qwen", "kimi", "deepseek", "copilot", "cursor",
	"sora", "midjourney", "stable diffusion", "transformer", "agent", "agi", "ml",
	"neural", "multimodal", "leak", "leaked", "beta", "preview", "fine-tune", "rag",
	"embedding", "context window", "prompt", "rlhf", "gpu", "h100", "h200", "b200",
	"tpu", "nvidia", "amd", "fsd", "optimus", "robot", "figure", "unitree", "tts",
	"asr", "whisper", "tokenizer", "hallucination", "benchmark", "sota",
}

// dealsKeywords doubles as the noise-filter protection list: any hit means the
// item may be a real 薅羊毛/限免/优惠 signal and must not be killed by default
// noise keywords such as 抽奖/giveaway.
var dealsKeywords = []string{
	"薅羊毛", "羊毛", "限免", "限时免费", "免费领", "白嫖", "bug价", "bug 价", "漏洞价",
	"神价", "史低", "折扣", "优惠", "促销", "秒杀", "满减", "优惠券", "折扣码", "优惠码",
	"空投", "云主机", "云服务器", "特价", "降价", "黑五", "终身", "买一送一", "返现",
	"返利", "礼品卡", "兑换码", "学生包", "免费额度", "促销码", "打折", "半价", "免单",
	"零元购", "1元购", "免费试用", "教育优惠", "拼车", "合租", "车位", "机场", "喜加一",
	"游戏限免", "立减", "红包", "百亿补贴", "话费", "流量包", "充值优惠", "限时", "清仓",
	"甩卖", "节点",
	"vps", "promo code", "coupon", "discount", "deal", "deals", "airdrop",
	"black friday", "cyber monday", "free tier", "lifetime", "cashback", "gift card",
	"redeem code", "free trial",
}

// adKeywords identify commercial promos, IDC/VPS reseller pitches, airport/proxy
// ads and affiliate shills; unlike softer engagement-bait words, they are not
// exempted by dealsKeywords.
var adKeywords = []string{
	"独立服务器", "独享ip", "双isp", "住宅ip", "家宽", "广播ip", "原生ip", "落地机",
	"专线机场", "机场节点", "高速节点", "三网优化", "cn2 gia", "cn2gia", "月起", "/月起",
	"元/月", "月付", "年付", "欢迎选购", "下单链接", "购买链接", "发卡网", "代充",
	"aff", "aff链接", "邀请返佣", "推广链接", "返佣链接", "商务合作", "广告投放", "tg频道",
	"sponsored", "#ad", "paid promotion", "referral link", "affiliate link",
}

// noiseKeywords are typical zero-information spam / engagement-bait / ads.
// They are skipped whenever a deals signal is present (unless the user set an
// explicit custom block keyword or an allow keyword overrides everything).
var noiseKeywords = []string{
	"互粉", "互关", "回关", "互fo", "求关注", "求fo", "关注回粉", "转发抽奖", "关注抽奖",
	"抽奖", "互推", "涨粉", "刷粉", "买粉", "代刷", "接推广", "付费推广", "诚招代理",
	"招代理", "加微信", "加v信", "加vx", "加qq", "私聊领", "扫码领", "点击领取",
	"免费领取课程", "网赚", "兼职日结", "日赚", "躺赚", "暴富", "稳赚", "包赚", "带单",
	"喊单", "百倍币", "千倍币", "土狗", "冲土狗", "内幕群", "荐股群", "炒股群", "带单群",
	"老师带", "跟单", "合约带单", "资金盘", "传销", "拉人头", "刷单", "点赞员", "打字员",
	"日结兼职", "无门槛提现", "秒提现", "注册送", "登录送", "签到领", "挖矿分红",
	"云挖矿", "静态收益", "动态收益", "拉新奖励",
	"follow back", "followback", "f4f", "l4l", "like4like", "likeforlike",
	"followforfollow", "giveaway", "paid promote", "dm for promo", "casino", "porn",
	"xxx", "sex", "viagra", "cialis", "pump and dump", "100x gem",
}

// urgentKeywords raise the L1 priority hint to "high".
var urgentKeywords = []string{
	"突发", "紧急", "快讯", "熔断", "停牌", "宕机", "崩了", "泄露", "被盗", "跑路",
	"爆仓", "暴跌", "暴涨", "跳水", "异动", "回滚", "召回", "诉讼", "禁令", "制裁",
	"降息", "加息", "预警", "事故", "故障", "瘫痪", "挤兑", "违约", "破产", "清算",
	"财报", "非农",
	"breaking", "urgent", "halted", "outage", "leak", "leaked", "0day", "zero-day",
	"exploit", "hacked", "rug pull", "hotfix", "rollback", "recall", "lawsuit",
	"sanction", "rate cut", "rate hike", "fomc", "cpi", "earnings", "incident",
	"liquidation", "bankruptcy",
}

type keywordMatcher struct {
	kw string
	re *regexp.Regexp // non-nil for ASCII keywords (word-boundary match)
}

func newKeywordMatcher(kw string) keywordMatcher {
	lower := strings.ToLower(strings.TrimSpace(kw))
	if lower == "" {
		return keywordMatcher{}
	}
	if isASCII(lower) {
		return keywordMatcher{kw: lower, re: regexp.MustCompile(`\b` + regexp.QuoteMeta(lower) + `\b`)}
	}
	return keywordMatcher{kw: lower}
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// match expects pre-lowercased text. CJK keywords use substring matching;
// ASCII keywords use word boundaries so "ai" never matches "email".
func (m keywordMatcher) match(lowerText string) bool {
	if m.kw == "" {
		return false
	}
	if m.re != nil {
		return m.re.MatchString(lowerText)
	}
	return strings.Contains(lowerText, m.kw)
}

type l1Rules struct {
	minRunes       int
	filterRetweets bool
	allow          []keywordMatcher
	block          []keywordMatcher
	ad             []keywordMatcher
	noise          []keywordMatcher
	deals          []keywordMatcher
	investing      []keywordMatcher
	ai             []keywordMatcher
	urgent         []keywordMatcher
}

func newL1Rules(cfg Config) *l1Rules {
	compile := func(kws []string) []keywordMatcher {
		out := make([]keywordMatcher, 0, len(kws))
		for _, kw := range kws {
			if m := newKeywordMatcher(kw); m.kw != "" {
				out = append(out, m)
			}
		}
		return out
	}
	return &l1Rules{
		minRunes:       cfg.L1MinContentRunes,
		filterRetweets: cfg.L1FilterRetweets,
		allow:          compile(cfg.L1AllowKeywords),
		block:          compile(cfg.L1BlockKeywords),
		ad:             compile(adKeywords),
		noise:          compile(noiseKeywords),
		deals:          compile(dealsKeywords),
		investing:      compile(investingKeywords),
		ai:             compile(aiKeywords),
		urgent:         compile(urgentKeywords),
	}
}

func anyMatch(ms []keywordMatcher, text string) bool {
	for _, m := range ms {
		if m.match(text) {
			return true
		}
	}
	return false
}

func firstMatch(ms []keywordMatcher, text string) string {
	for _, m := range ms {
		if m.match(text) {
			return m.kw
		}
	}
	return ""
}

// evaluate returns a complete, Validate-safe L1 result. Noise hits are final
// (filtered, never sent to L2); everything else is kept with a domain/priority
// guess and a nil AIScore (no fabricated scores).
func (r *l1Rules) evaluate(in triage.Input) triage.Result {
	text := strings.ToLower(in.Title + "\n" + in.Content)
	domain := r.guessDomain(text)
	priority := "normal"
	if anyMatch(r.urgent, text) {
		priority = "high"
	}

	if anyMatch(r.allow, text) {
		return triage.Result{Domain: domain, AITags: []string{}, AISummary: "L1 白名单放行：命中自定义关注关键词", Priority: priority, TriageStatus: "kept"}
	}
	if hit := firstMatch(r.block, text); hit != "" {
		return noiseResult("l1-block-keyword", "L1 过滤：命中自定义黑名单关键词「"+hit+"」")
	}
	if r.filterRetweets && isRetweet(in.RawMeta) {
		return noiseResult("l1-retweet", "L1 过滤：转推内容（AI_L1_FILTER_RETWEETS=true）")
	}
	if hit := firstMatch(r.ad, text); hit != "" {
		return noiseResult("l1-ad", "L1 过滤：命中推广/卖货广告特征「"+hit+"」")
	}
	if isSearchReplyChatter(in) {
		return noiseResult("l1-reply", "L1 过滤：X 检索中的 @对话回复水帖")
	}
	if !anyMatch(r.deals, text) {
		if hit := firstMatch(r.noise, text); hit != "" {
			return noiseResult("l1-noise-keyword", "L1 过滤：命中噪声关键词「"+hit+"」")
		}
	}
	if r.minRunes > 0 && effectiveRunes(in.Content) < r.minRunes && effectiveRunes(in.Title) == 0 && !hasValuableLink(in) {
		return noiseResult("l1-short", "L1 过滤：正文有效信息过短")
	}

	summary := "L1 规则放行：未命中噪声规则（未评分）"
	if domain != "other" {
		summary = "L1 规则放行：疑似 " + domain + " 域信号（未评分）"
	}
	return triage.Result{Domain: domain, AITags: []string{}, AISummary: summary, Priority: priority, TriageStatus: "kept"}
}

func noiseResult(reasonTag, summary string) triage.Result {
	return triage.Result{
		Domain:       "other",
		AITags:       []string{"l1-noise", reasonTag},
		AISummary:    summary,
		Priority:     "low",
		TriageStatus: "filtered",
	}
}

// guessDomain picks the domain with the most keyword hits; ties resolve to
// deals > ai > investing (actionability order). No hits means "other".
func (r *l1Rules) guessDomain(text string) string {
	counts := []struct {
		domain string
		n      int
	}{
		{"deals", countMatches(r.deals, text)},
		{"ai", countMatches(r.ai, text)},
		{"investing", countMatches(r.investing, text)},
	}
	best, bestN := "other", 0
	for _, c := range counts {
		if c.n > bestN {
			best, bestN = c.domain, c.n
		}
	}
	return best
}

func countMatches(ms []keywordMatcher, text string) int {
	n := 0
	for _, m := range ms {
		if m.match(text) {
			n++
		}
	}
	return n
}

// effectiveRunes strips URLs, @mentions, whitespace and the media-only
// placeholder before counting meaningful characters.
func effectiveRunes(content string) int {
	s := strings.ToLower(content)
	s = strings.ReplaceAll(s, "(media-only tweet)", "")
	s = urlRe.ReplaceAllString(s, "")
	s = mentionRe.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), "")
	return utf8.RuneCountInString(s)
}

// hasValuableLink reports whether the item carries a non-social external link
// (in content or as its canonical URL) that may hold the real information.
func hasValuableLink(in triage.Input) bool {
	candidates := urlRe.FindAllString(in.Content, -1)
	if in.URL != "" {
		candidates = append(candidates, in.URL)
	}
	for _, raw := range candidates {
		if strings.HasPrefix(raw, "www.") {
			raw = "https://" + raw
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		host := strings.ToLower(parsed.Hostname())
		if host != "" && !socialHosts[host] {
			return true
		}
	}
	return false
}

func isSearchReplyChatter(in triage.Input) bool {
	if strings.ToLower(strings.TrimSpace(in.SourceType)) != "x" {
		return false
	}
	trimmed := strings.TrimSpace(in.Content)
	if !strings.HasPrefix(trimmed, "@") {
		return false
	}
	origin, _ := in.RawMeta["origin_key"].(string)
	return strings.HasPrefix(origin, "query:")
}

func isRetweet(meta map[string]any) bool {
	v, ok := meta["is_retweet"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t == 1
	case int:
		return t == 1
	default:
		return false
	}
}