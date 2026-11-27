# `P2-02a`：单源 RSS 刷新 30s 超时热修（解耦 HTTP 抓取与 AI 分拣超时 + 同源刷新去重）

## 元信息（Metadata）
- **Task ID**：`P2-02a`
- **Phase / Milestone**：`Phase 2 (v0.2 Hotfix)`
- **前置门禁（Gate Dependency）**：`Gate 1（2026-11-20）与 DEC-008（2026-11-27）已批准 + P2-02 已 DONE 并合入 main`
- **当前状态（Status）**：`DONE`（2026-11-27 独立 `verifier` 全量 Go 单测、`go vet`、并发安全与边界审查零缺陷通过）
- **调度通道与目标执行端（Channel / Runner）**：`Swarm Worker / hotfix-p202a（独立 verifier 复验）`
- **工作区 / 分支策略（Worktree / Branch）**：`与文档任务按 Owned Files 严格隔离；变更须通过 verifier 核验后合入`

---

## 目标与上下文（Objective & Context）
- **问题根因**：在 `backend/internal/handler/feed.go` 的 `createFeed`、`refreshFeed` 与 `batchCreateFeeds` 中，后台调用 `h.puller.RefreshFeed(ctx, feedID)` 目前统一复用 `context.WithTimeout(context.Background(), time.Duration(h.config.PullTimeout)*time.Second)`（默认 **30s**）。而在 `backend/internal/pull/puller.go` 中，`pullFeed(ctx, feed)` 在同一个 `ctx` 下顺序执行 `FetchAndParse`（RSS HTTP 抓取）和 `saveItems`（对开启 `ai_enabled=true` 的源逐条同步执行 AI 评估，最多 `AI_RSS_MAX_ITEMS_PER_PULL` 条）。当 L3 采用 `CLIProvider`（如 `AI_CLI_COOLDOWN_SECONDS=20`、`AI_TIMEOUT_SECONDS=90`）时，第 1~2 条评估就会触发 30s `context.DeadlineExceeded`，导致同批剩余已提交 `pending` 条目立即短路并被回写为 `triage_status="failed"`。
- **核心目标**：
  1. 在 `puller.pullFeed` 内将 RSS HTTP 抓取超时与 AI 分拣超时解耦：仅对 `FetchAndParse` 派生独立的 `fetchCtx`（受 `p.timeout` 即 `FUSION_PULL_TIMEOUT` 约束，抓取返回后立即 `cancel()`），`saveItems` 继续沿用父 `ctx`；
  2. 在 `handler/feed.go` 将单源后台刷新（`refreshFeed`、`createFeed`、`batchCreateFeeds`）的父上下文超时放宽为 `singleFeedRefreshTimeout = 15 * time.Minute`（或 `max(15*time.Minute, time.Duration(h.config.PullTimeout)*time.Second)`，兼容大超时配置）；
  3. 在 `Handler` 上增加按 `feed_id` 维度的在途刷新去重守卫（`feedRefreshMu sync.Mutex` + `feedRefreshInFlight map[int64]bool` 及 `tryStartFeedRefresh` / `finishFeedRefresh`），避免重复点击「刷新」堆积并发 CLI 分拣任务，并确保 `defer` 在任何退出路径下可靠释放；
  4. 补充快速、无外网依赖的确定性单元测试覆盖 `puller` 与 `handler`。
- **前置输入 / 依赖任务**：`P2-02`（已 `DONE`）；本热修完成后直接解锁 `P2-02b`（两者均涉及 `puller.go` 与 `handler.go`）。

---

## 关联文档与约束（References & Constraints）
开始前必读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)
4. [docs/PRD.md](../docs/PRD.md)
5. [docs/ROADMAP.md](../docs/ROADMAP.md)

本任务需特别注意的约束：
- [x] 遵守当前 Phase 门禁与 `[TBD]` 未决项边界（不触碰数据库迁移、前端或 `backend/internal/ai/**`）
- [x] 遵守 Demo-First（最小热修切片，零多余抽象）
- [x] 遵守宽松开源协议（零新增依赖，严禁修改 `backend/go.mod` / `backend/go.sum`）
- [x] 遵守 1C1G–2C2G 小型 Linux VPS Docker 资源预算（保证并发锁、map 初始化、context `cancel()` 零泄漏）

---

## 文件边界（Owned / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `backend/internal/pull/puller.go`
- `backend/internal/pull/puller_test.go`
- `backend/internal/handler/feed.go`
- `backend/internal/handler/handler.go`
- `backend/internal/handler/feed_refresh_test.go`

### Read-Only Reference（仅供读取对照，禁止修改）
- `AGENTS.md`、`docs/**`、`tasks/**`、`backend/internal/ai/**`、`backend/internal/config/config.go`、`backend/internal/store/**`

### Forbidden / Out-of-Scope（严禁触碰）
- 未列在 `Owned Files` 内的所有其他文件
- **高风险禁区**：`docs/**`、`tasks/**`、`backend/go.mod`、`backend/go.sum`、`backend/internal/ai/**`、`backend/internal/store/migrations/**`、`frontend/**`、`plugins/**`、`deploy/**`、`Temp/**`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）
- [x] **抓取与分拣超时解耦**：`puller.pullFeed` 中 `FetchAndParse` 严格受 `p.timeout` 约束且返回后立即 `cancel()`；`saveItems` 不再受 30s `PullTimeout` 截断。
- [x] **单源刷新超时与去重守卫**：`refreshFeed` / `createFeed` / `batchCreateFeeds` 均采用 `>= 15m`（且不小于 `PullTimeout`）的后台上下文；同一 `feed_id` 刷新在途时重复请求 `POST /api/feeds/:id/refresh` 立即返回 `202 Accepted` 且不重复调用 `RefreshFeed`；不同 `feed_id` 可并行刷新；完成后可再次触发刷新；struct 字面量 `&Handler{}` 场景下不会因 nil map 发生 panic。
- [x] **确定性单测覆盖**：
  - `puller_test.go` 覆盖「短 `PullTimeout`（如 50ms）+ 快速 HTTP 抓取 + 慢评估器（>100ms）在宽父上下文下成功完成」以及「慢 HTTP 抓取（>`PullTimeout`）仍正常超时失败」；
  - `feed_refresh_test.go` 覆盖「单源刷新 context deadline ≥ 15m」、「同源并发去重」、「异源并行」与「完成后可再触发」。
- [x] **所有变更均位于 `Owned Files` 范围内**，Go 全量单测与 `go vet` 通过。

建议核验命令：
```bash
# 1) Go 单测与静态检查（含可选 -race）
cd backend
go test -count=1 ./...
go vet ./...

# 2) 边界核验
git status --short
git diff --name-only
```

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`DONE`（2026-11-27，经独立 `verifier` 复验零缺陷通过）
- **执行端 / 通道**：Swarm Worker `hotfix-p202a`（独立复验：`verifier`）
- **实际改动文件清单**：
  - `backend/internal/pull/puller.go`：在 `pullFeed` 中当 `p.timeout > 0` 时为 `FetchAndParse` 派生独立的 `fetchCtx` 并在抓取返回后立即调用 `cancelFetch()`，使 `saveItems(ctx, feed, inputs)` 继续沿用父 `ctx` 而不受 30s 抓取超时截断。
  - `backend/internal/handler/handler.go`：在 `Handler` 结构体新增 `feedRefreshMu sync.Mutex` 与 `feedRefreshInFlight map[int64]bool`（`New` 中初始化，同时对 `&Handler{}` 字面量惰性初始化安全）。
  - `backend/internal/handler/feed.go`：新增 `singleFeedRefreshTimeout = 15 * time.Minute`、`feedRefreshTimeout()`（`max(15m, PullTimeout)`，nil-config 安全）、`tryStartFeedRefresh` / `finishFeedRefresh` 与统一 `startFeedRefresh` 辅助函数（含 `h.puller == nil` 保护与 `defer h.finishFeedRefresh(id)`），并接入 `refreshFeed`、`createFeed` 与 `batchCreateFeeds`。
  - `backend/internal/pull/puller_test.go`：新增 `TestRefreshFeedDecouplesFetchTimeoutFromTriage` 与 `TestRefreshFeedFetchTimeoutStillCancelsSlowHTTP`。
  - `backend/internal/handler/feed_refresh_test.go`：保留 `TestRefreshAllRunGuard`，新增 `TestFeedRefreshRunGuard`、`TestFeedRefreshTimeoutCalculation`、`TestRefreshFeedTimeoutAndDeduplication` 与 `TestCreateAndBatchCreateFeedsUseSingleFeedRefreshTimeout`。
- **实测验证命令与观察结果**（独立 `verifier` 在 WSL `go version go1.26.2 linux/amd64`，`/mnt/d/Project/Maintain-online/news_panel/backend` 实测观察）：
  - 定向单测：`go test -v -count=1 ./internal/pull/... ./internal/handler/... -run "TestRefreshFeed|TestFeedRefresh|TestCreateAndBatchCreateFeeds|TestRefreshAllRunGuard"` → 全部 7 项测试 PASS（`TestRefreshFeedDecouplesFetchTimeoutFromTriage` 0.17s、`TestRefreshFeedFetchTimeoutStillCancelsSlowHTTP` 0.08s、`TestRefreshAllRunGuard`、`TestFeedRefreshRunGuard`、`TestFeedRefreshTimeoutCalculation`、`TestRefreshFeedTimeoutAndDeduplication` 0.08s、`TestCreateAndBatchCreateFeedsUseSingleFeedRefreshTimeout` 0.08s）。
  - 全量单测：`go test -count=1 ./...` → **PASS**（退出码 0；全部 9 个含测试包 `ai` / `auth` / `config` / `handler` / `pkg/httpc` / `pull` / `pullpolicy` / `store` / `triage` 全部 `ok`）。
  - 静态检查：`go vet ./...` → 无告警（退出码 0）。
  - 边界检查：`git status --short`、`git diff --name-only`、`git diff --check` → 全部 5 个 Go 文件 100% 落入 `Owned Files`，`go.mod`/`go.sum`/`Temp/` 均未触碰，退出码 0。
  - 说明：WSL 环境无 `gcc`（`command -v gcc` 返回空），故 `CGO_ENABLED=1 go test -race` 无法编译运行；已由 `verifier` 逐行静态审计 `feedRefreshMu`、`feedRefreshInFlight` 惰性初始化、`defer` 释放与 `cancelFetch()` 确认无竞态与泄漏风险。
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - 无边界外修改；本热修已解锁 `P2-02b`（`tasks/P2-02b-decision-apis-and-ai-settings.md`），可直接开分支认领开发。
