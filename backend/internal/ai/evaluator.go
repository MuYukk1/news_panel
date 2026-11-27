package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"news_panel/internal/triage"
)

// Evaluator and its providers are shared by RSS and plugin ingest.
type Evaluator struct {
	state    atomic.Pointer[evalSnapshot]
	reloadMu sync.Mutex
	env      Config
	counters pipelineCounters
}

type evalSnapshot struct {
	decision DecisionProvider
	counters *pipelineCounters
	cfg      Config
	provider CompletionProvider // nil => L1-only
	l1       *l1Rules
	logger   *slog.Logger
}

var _ triage.TriageEvaluator = (*Evaluator)(nil)

// New wires the effective pipeline. Degradation to L1-only is silent-safe:
// disabled, provider=l1, openai without a key, or cli without a command all
// yield a working L1-only evaluator instead of an error or a startup block.
func New(cfg Config, logger *slog.Logger) *Evaluator {
	if logger == nil {
		logger = slog.Default()
	}
	owner := &Evaluator{env: cfg}
	e := &evalSnapshot{cfg: cfg, l1: newL1Rules(cfg), logger: logger, counters: &owner.counters}
	switch {
	case !cfg.Enabled:
		logger.Info("ai pipeline disabled; running L1-only")
	case cfg.Provider == "l1":
		logger.Info("ai provider=l1; running L1-only")
	case cfg.Provider == "cli":
		if cfg.CLICommand == "" {
			logger.Warn("AI_PROVIDER=cli but AI_CLI_COMMAND is empty; degrading to L1-only")
		} else {
			e.provider = NewCLIProvider(cfg.CLICommand, cfg.CLIArgs, cfg.Timeout, cfg.CLICooldown)
			logger.Info("ai L3 provider enabled", "provider", "cli", "cooldown", cfg.CLICooldown)
		}
	default: // openai
		if cfg.APIKey == "" {
			logger.Warn("AI_PROVIDER=openai but AI_API_KEY is empty; degrading to L1-only")
		} else {
			e.provider = NewOpenAIProvider(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.OpenAIJSONFormat, cfg.Timeout)
			logger.Info("ai L3 provider enabled", "provider", "openai")
		}
	}
	if e.provider != nil {
		e.decision = newDecisionProvider(cfg)
		if e.decision == nil {
			logger.Info("ai L2 decision gate disabled or unconfigured; bypassing L2 directly to L3")
		} else {
			logger.Info("ai L2 decision gate enabled", "provider", e.decision.Name())
		}
	}
	owner.state.Store(e)
	return owner
}

func (e *Evaluator) EnvConfig() Config {
	cfg := e.env
	cfg.Decision.L3Priorities = slices.Clone(cfg.Decision.L3Priorities)
	cfg.L1BlockKeywords = slices.Clone(cfg.L1BlockKeywords)
	cfg.L1AllowKeywords = slices.Clone(cfg.L1AllowKeywords)
	cfg.CLIArgs = slices.Clone(cfg.CLIArgs)
	return cfg
}

func (e *Evaluator) Reload(cfg Config) {
	e.reloadMu.Lock()
	defer e.reloadMu.Unlock()
	next := *e.state.Load()
	next.cfg = cfg
	next.l1 = newL1Rules(cfg)
	if next.provider != nil {
		next.decision = newDecisionProvider(cfg)
	}
	e.state.Store(&next)
}

func NewEvaluatorFromEnv(logger *slog.Logger) *Evaluator {
	return New(ConfigFromEnv(logger), logger)
}

// NewFromEnv is the minimal assembly entry point used by cmd/fusion.
func NewFromEnv() *Evaluator { return NewEvaluatorFromEnv(slog.Default()) }

func (e *Evaluator) Evaluate(ctx context.Context, in triage.Input) (triage.Result, error) {
	return e.state.Load().evaluate(ctx, in)
}

func (e *evalSnapshot) evaluate(ctx context.Context, in triage.Input) (triage.Result, error) {
	if err := ctx.Err(); err != nil {
		return triage.Result{}, err
	}
	l1 := e.l1.evaluate(in)
	if l1.TriageStatus == "filtered" {
		e.counters.l1Filtered.Add(1)
		return l1, nil
	}
	if e.provider == nil {
		l1.AITags = []string{"l1-only"}
		return l1, nil
	}

	if e.decision == nil {
		e.counters.l2Skipped.Add(1)
		return e.score(ctx, in, l1, nil)
	}
	hint, err := e.decision.Decide(ctx, in, l1)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return triage.Result{}, ctxErr
	}
	if err != nil {
		e.counters.l2Errors.Add(1)
		safe := e.safeError(err.Error())
		e.logger.Warn("ai L2 decision failed", "mode", e.cfg.Decision.FallbackMode, "error", safe)
		switch e.cfg.Decision.FallbackMode {
		case "fail":
			return triage.Result{}, errors.New(safe)
		case "keep_l1":
			l1.AITags = []string{"l2-decision-fallback"}
			return l1, nil
		default:
			return e.score(ctx, in, l1, nil)
		}
	}
	if hint.Action == "filter" {
		e.counters.l2Filtered.Add(1)
		return gateResult(l1, hint, "filtered", "l2-decision-filtered"), nil
	}
	e.counters.l2Passed.Add(1)
	if !slices.Contains(e.cfg.Decision.L3Priorities, hint.Priority) {
		e.counters.l2BypassedL3.Add(1)
		if e.cfg.Decision.LowAction == "filter" {
			return gateResult(l1, hint, "filtered", "l2-decision-low-filtered"), nil
		}
		return gateResult(l1, hint, "kept", "l2-decision-bypass"), nil
	}
	return e.score(ctx, in, l1, &hint)
}

func gateResult(l1 triage.Result, hint DecisionOutcome, status, tag string) triage.Result {
	l1.Domain = hint.Domain
	l1.Priority = hint.Priority
	l1.TriageStatus = status
	l1.AITags = []string{tag}
	return l1
}

func (e *evalSnapshot) score(ctx context.Context, in triage.Input, l1 triage.Result, hint *DecisionOutcome) (triage.Result, error) {
	system, user := e.l3Prompts(in, l1, hint)
	e.counters.l3Called.Add(1)
	completion, err := e.provider.Complete(ctx, system, user)
	if err == nil {
		var v verdict
		if v, err = parseVerdict(completion); err == nil {
			result := normalizeVerdict(v, l1, e.cfg.KeepMinScore)
			if result.TriageStatus == "filtered" {
				e.counters.l3Filtered.Add(1)
			} else {
				e.counters.l3Kept.Add(1)
			}
			return result, nil
		}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return triage.Result{}, ctxErr
	}
	e.counters.l3Fallback.Add(1)
	return e.fallback(l1, err)
}

// fallback applies AI_FALLBACK_MODE after an L3 failure. keep_l1 preserves the
// L1 verdict (important information is never lost) with an l2-fallback tag;
// fail returns a redacted error so the handler stores triage_status=failed.
func (e *evalSnapshot) fallback(l1 triage.Result, cause error) (triage.Result, error) {
	safe := e.safeError(fmt.Sprintf("%s provider: %v", e.provider.Name(), cause))
	if e.cfg.FallbackMode == "fail" {
		return triage.Result{}, errors.New(safe)
	}
	e.logger.Warn("ai L3 failed; keeping L1 result",
		"provider", e.provider.Name(), "error", safe)
	l1.AITags = []string{"l2-fallback"}
	return l1, nil
}

func (e *evalSnapshot) safeError(s string) string {
	return redactCredential(redact(s, e.cfg.APIKey), e.cfg.Decision.APIKey)
}

const promptContentLimit = 2000

func (e *evalSnapshot) l3Prompts(in triage.Input, l1 triage.Result, hint *DecisionOutcome) (string, string) {
	system := fmt.Sprintf(`你是 news_panel 个人情报面板的信息分拣器。用户关注三大核心情报域：
- investing：投资/股票突发新闻、财报、宏观催化剂（美联储、CPI、关税、并购等）
- ai：AI 官方动态、前沿模型发布、内测/灰度泄露、重要开源与算力进展
- deals：高时效薅羊毛、限免、Bug 价、云主机/VPS 折扣、空投规则等可操作线报
- other：以上之外的一般信息
用户自定义关注主题：%s

评估每条输入的相关性、时效性、可操作性与信息密度，严格只输出一个 JSON 对象，不要输出任何解释、前后缀或代码围栏：
{"domain":"investing|ai|deals|other","score":0-100,"tags":["最多6个短标签"],"summary":"中文高密度摘要与核心译文","priority":"critical|high|normal|low","keep":true|false}
裁定规则：
1. score>=%d 且确有实质情报价值才 keep=true；
2. 凡 VPS/主机/机场/代理节点卖货广告、推广返佣、课程带货、互粉引流、纯抽奖水帖、无信息情绪吐槽或无上下文的 @他人闲聊回复，一律 keep=false 且 score<=20；
3. 突发重大催化剂可给 priority=critical/high；
4. summary 必须用自然准确的中文：若原文为英文等外语，请完整提炼并翻译核心事实、关键数值与结论（保留专有名词）；中文原文则精炼核心信息。`,
		e.cfg.Topics, e.cfg.KeepMinScore)

	var user strings.Builder
	fmt.Fprintf(&user, "请评估以下条目：\n来源类型：%s\n作者：%s\n标题：%s\n正文：%s\n发布时间：%s\n链接：%s\nL1 初判：domain=%s priority=%s",
		in.SourceType, in.Author, in.Title, truncateRunes(in.Content, promptContentLimit),
		in.PublishedAt.UTC().Format(time.RFC3339), in.URL, l1.Domain, l1.Priority)
	if rt, ok := in.RawMeta["is_retweet"]; ok {
		fmt.Fprintf(&user, "\n是否转推：%v", rt)
	}
	if hint != nil {
		fmt.Fprintf(&user, "\nL2 门控初判：priority=%s domain=%s", hint.Priority, hint.Domain)
	}
	return system, user.String()
}
