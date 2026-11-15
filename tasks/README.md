# `tasks/` 双通道跨工具协作与交接规范

> **设计目的**：支持根据不同 AI 编程平台/工具的剩余额度灵活派发任务，确保任务可在不同工具之间无缝接力且不破坏文件边界与架构基线。  
> **当前阶段提示**：**Gate 0 已于 2026-11-15 批准**，当前处于 **Phase 1（v0.1 纵向切片）**；所有编码与文档任务均须通过本目录任务卡派发并严格限定在 `Owned Files` 内（详见 [INDEX.md](./INDEX.md)）。

---

## 1. 双通道派发模型（Dual-Channel Dispatch）

无论通过哪条通道派发，**唯一真理源**均为本仓库的 [AGENTS.md](../AGENTS.md)、`docs/` 基线文档及对应的 `tasks/<TASK-ID>.md` 任务卡。

### 通道 A：文档驱动交接（ZCode / Qoder / Trae）
适用于交互式 IDE / 客户端工具按卡领单、手动或半自动接力执行：
1. **派发前**：协调者或人类基于 [tasks/_TEMPLATE.md](./_TEMPLATE.md) 创建 `tasks/<TASK-ID>.md`（明确 `Owned Files`、验收标准与门禁前提），并在 [tasks/INDEX.md](./INDEX.md) 登记为 `TODO`。
2. **领单执行**：新会话启动后，执行端按 [AGENTS.md](../AGENTS.md) 顺序阅读基线文档与目标任务卡，将任务卡及 `INDEX.md` 状态改为 `IN_PROGRESS`，严格在 `Owned Files` 内开发或改写文档。
3. **交接回填**：完成后在任务卡末尾填写 `## Handoff Report`（附改动文件、实测命令与输出），将状态置为 `IN_REVIEW`，交由核验端或人类审查。

### 通道 B：CodexHost 调度执行（Codex / WorkBuddy / OpenCode / DeepSeek Harness）
适用于通过 **CodexHost** 按额度调度 CLI / Harness 执行端批量或隔离运行任务：
1. **任务卡即 Prompt 上下文**：调度前同样须先落盘 `tasks/<TASK-ID>.md`（并在 `INDEX.md` 登记），将任务卡路径、目标与 [AGENTS.md](../AGENTS.md) 约束作为执行输入（具体宿主机调度命令与参数以 CodexHost 实际配置为准，不在文档中预设私有参数）。
2. **Git Worktree 隔离原则**：
   - 每个通道 B 任务应分配**独立的 Git worktree 与工作分支**（例如与任务 ID 绑定的独立分支），严禁多个 Harness 实例同时写入同一工作区。
   - 任务卡内的 `Owned Files` 边界在合入前依然严格生效：哪怕在独立 worktree 中，超出 `Owned Files` 的意外改动在合入审查时也须拒绝或剥离。
3. **结果回传与状态回填**：Harness 执行完成后，由执行端（或合入协调者）将实测结果与改动摘要写入对应任务卡 `## Handoff Report`，并将状态置为 `IN_REVIEW` 后进入核验流程。

---

## 2. 任务状态流转（Task State Machine）

所有任务卡（及 `tasks/INDEX.md`）统一使用以下 5 种状态：

```text
TODO ──▶ IN_PROGRESS ──▶ IN_REVIEW ──▶ DONE
  │           │               │
  └───────────┴──▶ BLOCKED ◀──┘
```

| 状态 | 含义 | 允许触发流转的角色与前提 |
| :--- | :--- | :--- |
| `TODO` | 已定义范围与边界、前置 Gate 已满足，待领取 | 协调者 / 人类创建任务卡时设定 |
| `IN_PROGRESS` | 已由某执行端/工具认领并正在处理 | 执行端开始工作时更新（同一任务同时仅允许 1 个执行端占用） |
| `IN_REVIEW` | 自测完成并填妥 `Handoff Report`，等待核验/合入 | 执行端完成任务自检后更新 |
| `DONE` | 独立核验通过（且若使用 worktree 已干净合入目标分支） | **仅核验端 / 协调者 / 人类**有权标为 `DONE` |
| `BLOCKED` | 遇到外部依赖缺失、Human Gate 未放行或需修改边界外文件 | 执行端遇到硬阻塞时立即标记并在 `Handoff Report` 说明原因 |

---

## 3. 合入与独立核验规范（Merge & Verification Gate）

任务从 `IN_REVIEW` 进入 `DONE` 前，必须逐项通过以下核验：
1. **门禁与非目标检查**：确认未越过当前 Human Gate（Gate 0 前严禁存在源码/`Dockerfile`），未擅自锁定 `[TBD]` 选型，未引入 `Explicit Non-Goals`。
2. **边界检查（Diff Audit）**：核对实际变更文件列表（如 `git diff --name-only`），100% 落入任务卡定义的 `Owned Files`；若存在独立 worktree 分支，确认无冲突且未夹带无关修改。
3. **协议与资源合规检查**：确认未引入 GPL/AGPL 依赖，未引入重型基础设施或高内存依赖（如 Chromium/Kafka/ELK/JVM）。
4. **实测证据核查**：确认 `Handoff Report` 中列出的验证命令在最终修改后确实运行且结果如实记录（无跳过、无猜测）。
5. **看板与交接同步**：合入后同步更新 [tasks/INDEX.md](./INDEX.md) 状态；若涉及里程碑推进或决策变更，同步更新 [docs/HANDOFF.md](../docs/HANDOFF.md) / [docs/DECISIONS.md](../docs/DECISIONS.md)。
