package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"news_panel/internal/triage"
)

// Evaluator is the news_panel v0.1 triage.TriageEvaluator: L1 local rules
// followed by an optional L2 completion provider. It never fabricates scores
// (AIScore is nil unless L2 actually scored the item) and degrades safely when
// AI is disabled, unconfigured, or failing.
type Evaluator struct {
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
	e := &Evaluator{cfg: cfg, l1: newL1Rules(cfg), logger: logger}
	switch {
	case !cfg.Enabled:
		logger.Info("ai pipeline disabled; running L1-only")
	case cfg.Provider == "l1":
		logger.Info("ai provider=l1; running L1-only")
	case cfg.Provider == "cli":
		if cfg.CLICommand == "" {
			logger.Warn("AI_PROVIDER=cli but AI_CLI_COMMAND is empty; degrading to L1-only")
		} else {
			e.provider = NewCLIProvider(cfg.CLICommand, cfg.CLIArgs, cfg.Timeout)
			logger.Info("ai L2 provider enabled", "provider", "cli", "command", cfg.CLICommand)
		}
	default: // openai
		if cfg.APIKey == "" {
			logger.Warn("AI_PROVIDER=openai but AI_API_KEY is empty; degrading to L1-only")
		} else {
			e.provider = NewOpenAIProvider(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.OpenAIJSONFormat, cfg.Timeout)
			logger.Info("ai L2 provider enabled", "provider", "openai", "model", cfg.Model)
		}
	}
	return e
}

func NewEvaluatorFromEnv(logger *slog.Logger) *Evaluator {
	return New(ConfigFromEnv(logger), logger)
}

// NewFromEnv is the minimal assembly entry point used by cmd/fusion.
func NewFromEnv() *Evaluator { return NewEvaluatorFromEnv(slog.Default()) }

func (e *Evaluator) Evaluate(ctx context.Context, in triage.Input) (triage.Result, error) {
	if err := ctx.Err(); err != nil {
		return triage.Result{}, err
	}
	l1 := e.l1.evaluate(in)
	if l1.TriageStatus == "filtered" {
		return l1, nil // obvious noise never consumes L2 tokens
	}
	if e.provider == nil {
		l1.AITags = []string{"l1-only"}
		return l1, nil
	}

	system, user := e.prompts(in, l1)
	completion, err := e.provider.Complete(ctx, system, user)
	if err == nil {
		var v verdict
		if v, err = parseVerdict(completion); err == nil {
			return normalizeVerdict(v, l1, e.cfg.KeepMinScore), nil
		}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return triage.Result{}, ctxErr
	}
	return e.fallback(in, l1, err)
}

// fallback applies AI_FALLBACK_MODE after an L2 failure. keep_l1 preserves the
// L1 verdict (important information is never lost) with an l2-fallback tag;
// fail returns a redacted error so the handler stores triage_status=failed.
func (e *Evaluator) fallback(in triage.Input, l1 triage.Result, cause error) (triage.Result, error) {
	safe := redact(fmt.Sprintf("%s provider: %v", e.provider.Name(), cause), e.cfg.APIKey)
	if e.cfg.FallbackMode == "fail" {
		return triage.Result{}, errors.New(safe)
	}
	e.logger.Warn("ai L2 failed; keeping L1 result",
		"provider", e.provider.Name(), "external_id", in.ExternalID, "error", safe)
	l1.AITags = []string{"l2-fallback"}
	return l1, nil
}

const promptContentLimit = 2000

func (e *Evaluator) prompts(in triage.Input, l1 triage.Result) (string, string) {
	system := fmt.Sprintf(`你是 news_panel 个人情报面板的信息分拣器。用户关注三大核心情报域：
- investing：投资/股票突发新闻、财报、宏观催化剂（美联储、CPI、关税、并购等）
- ai：AI 官方动态、前沿模型发布、内测/灰度泄露、重要开源与算力进展
- deals：高时效薅羊毛、限免、Bug 价、云主机/VPS 折扣、空投规则等可操作线报
- other：以上之外的一般信息
用户自定义关注主题：%s

评估每条输入的相关性、时效性、可操作性与信息密度，严格只输出一个 JSON 对象，不要输出任何解释、前后缀或代码围栏：
{"domain":"investing|ai|deals|other","score":0-100,"tags":["最多6个短标签"],"summary":"一句话高密度中文摘要","priority":"critical|high|normal|low","keep":true|false}
裁定规则：score>=%d 且确有情报价值才 keep=true；水帖、互粉、抽奖引流、纯广告、无信息情绪帖一律 keep=false 且 score 给低分。突发重大催化剂可给 priority=critical/high。summary 必须是中文一句话。`,
		e.cfg.Topics, e.cfg.KeepMinScore)

	var user strings.Builder
	fmt.Fprintf(&user, "请评估以下条目：\n来源类型：%s\n作者：%s\n标题：%s\n正文：%s\n发布时间：%s\n链接：%s\nL1 初判：domain=%s priority=%s",
		in.SourceType, in.Author, in.Title, truncateRunes(in.Content, promptContentLimit),
		in.PublishedAt.UTC().Format(time.RFC3339), in.URL, l1.Domain, l1.Priority)
	if rt, ok := in.RawMeta["is_retweet"]; ok {
		fmt.Fprintf(&user, "\n是否转推：%v", rt)
	}
	return system, user.String()
}