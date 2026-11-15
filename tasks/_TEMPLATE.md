# `<TASK-ID>`：<简短任务标题>

## 元信息（Metadata）
- **Task ID**：`<TASK-ID>`（如 `P1-01`）
- **Phase / Milestone**：`<Phase X / Milestone>`
- **前置门禁（Gate Dependency）**：`<如：Gate 0 已由用户拍板>`
- **当前状态（Status）**：`TODO`（可选：`TODO` | `IN_PROGRESS` | `IN_REVIEW` | `DONE` | `BLOCKED`）
- **调度通道与目标执行端（Channel / Runner）**：`<通道 A：ZCode / Qoder / Trae | 通道 B：CodexHost (Codex / WorkBuddy / OpenCode / DeepSeek Harness) | Antigravity>`
- **工作区 / 分支策略（Worktree / Branch）**：`<单工作区串行 | 独立 Git worktree: <branch-name>>`

---

## 目标与上下文（Objective & Context）
<!-- 用 3~5 句话写清本任务要交付什么、为什么做、与当前纵向切片的关系，使任何新接入的工具无需翻阅历史对话即可上手。 -->
- **核心目标**：
- **前置输入 / 依赖任务**：

---

## 关联文档与约束（References & Constraints）
开始前必读：
1. [AGENTS.md](../AGENTS.md)
2. [docs/HANDOFF.md](../docs/HANDOFF.md)
3. [docs/DECISIONS.md](../docs/DECISIONS.md)
4. [docs/PRD.md](../docs/PRD.md)
5. [docs/ROADMAP.md](../docs/ROADMAP.md)

本任务需特别注意的约束（按需勾选/补充）：
- [ ] 遵守当前 Phase 门禁与 `[TBD]` 未决项边界（不擅自锁定未拍板技术选型）
- [ ] 遵守 Demo-First（不编写超出本切片范围的通用抽象）
- [ ] 遵守宽松开源协议（仅 MIT / Apache-2.0 / BSD，严禁引入 GPL / AGPL）
- [ ] 遵守 1C1G–2C2G 小型 Linux VPS Docker 资源预算

---

## 文件边界（Owned / Forbidden Files）

### Owned Files（仅允许创建或修改以下路径）
- `<path/to/owned_file_or_dir_1>`
- `<path/to/owned_file_or_dir_2>`

### Read-Only Reference（仅供读取对照，禁止修改）
- `docs/PRD.md`、`docs/DECISIONS.md`、`<其他只读契约或模块>`

### Forbidden / Out-of-Scope（严禁触碰）
- 未列在 `Owned Files` 内的所有其他文件
- `<显式列出的高风险冲突区或不属于本阶段的文件，如 src/（Gate 0 前）>`

---

## 验收标准与验证命令（Acceptance Criteria & Verification）
<!-- 必须给出客观、可核验的完成条件与待执行命令 -->
- [ ] 标准 1：
- [ ] 标准 2：
- [ ] 所有变更均位于 `Owned Files` 范围内

建议核验命令：
```text
# 在此填写核验所需命令（如文件列表检查、构建、单测、lint 等）
```

---

## Handoff Report（交付回执 —— 结束前必填）
- **提交状态**：`IN_REVIEW` | `BLOCKED`
- **执行端 / 通道**：`<实际执行工具与通道>`
- **实际改动文件清单**：
  - `<path>`（说明改动内容）
- **实测验证命令与观察结果**（仅填写最终修改后亲自跑过并观察到的真实结果）：
  - 命令：`...` → 结果：`...`
- **边界变更申请 / 遗留风险 / 下一步建议**：
  - `<无，或写明需要边界外联动修改的文件及原因>`
