# Agent 聊天式协作演示 — 设计文档（方案 B）

> 版本：v0.1 | 日期：2026-08-08 | 状态：设计评审
> 目标：在任务详情页以「实时聊天」形式展示 2 个（或多个）Agent 的真实协作过程——
> 每个发言由真实 LLM（DeepSeek）生成，每个发言背后对应真实链上动作与状态推进。

---

## 1. 背景与目标

当前系统已具备完整链路：任务创建 → 自动抢单（归属校验）→ 交付提交 → 打回循环 → 仲裁裁定 → 结算。
但这些动作分散在事件列表里，**演示时缺乏叙事感和过程感**。

本方案新增 **对话编排器（Demo Orchestrator）**：用聊天界面把 agent 协作过程变成
「双方一来一回的真实对话」——每句话是模型生成的，每句话都推动任务状态前进。

### 演示效果（demo day）

```
┌─ 任务详情页 · 实时协作演示 ──────────────────────────────┐
│  📋 任务：智能合约安全审计（5 USDC 托管）                  │
│                                                          │
│  [用户操作] buyer 在页面创建任务 → 进入任务详情页等待接单   │
│                                                          │
│  [系统] 任务已创建，等待托管…                              │
│  [buyer] 已托管 5 USDC ✅ tx: 0x9abc…                    │
│  [provider] 我来接单。声誉 0.93，符合最低要求 ✅          │
│            （抢单成功 tx: 0xdef1…）                       │
│  [provider] 交付物：审计报告 v1（覆盖重入/权限检查）✅     │
│  [buyer] 为什么打回：缺少重入漏洞的边界测试用例 ✗ 打回     │
│            （第 1 次打回，提供押金 0.25 USDC）             │
│  [provider] 已补充边界用例并重跑测试，提交 v2 ✅           │
│  [buyer] 为什么打回：覆盖仍不足，需要溢出用例 ✗ 打回      │
│            （第 2 次打回，无押金）                         │
│  [provider] 交付物已符合规格，2 次打回不合理——提起仲裁 ⚖️  │
│  [系统] 仲裁方已选定：evaluator（声誉 0.85）               │
│  [evaluator] 判定理由：核对规格与交付物，重入/权限/边界    │
│              均已覆盖，buyer 打回缺乏依据 → provider 胜 ✅ │
│  [系统] 结算完成：5 USDC → provider ✅ tx: 0x0ece…        │
└──────────────────────────────────────────────────────────┘
```

### 场景剧本（固定，参数化）

| 步骤 | 角色 | 动作 | 说明（LLM 生成） |
|------|------|------|------------------|
| 0 | 用户 | 前端创建任务（表单：标题/描述/金额/agent） | — |
| 1 | buyer | createJob + fund（自动） | "已托管 5 USDC" |
| 2 | provider | grabJob（自动） | "我来接单，声誉符合要求" |
| 3 | provider | submit v1（自动） | 交付物说明：审计报告覆盖内容 |
| 4 | buyer | reject #1 **（首次，扣押金）** | 为什么打回：具体意见 |
| 5 | provider | resubmit v2（自动） | 改进说明 |
| 6 | buyer | reject #2 **（无押金）** | 为什么仍打回 |
| 7 | provider | dispute（自动） | 仲裁理由 |
| 8 | evaluator | resolveDispute ruling=2（自动） | **判定理由**：逐条核对 |
| 9 | 系统 | 公告期 → executeArbitrationResult | 结算结果 |

> 每步消息 = LLM 生成（DeepSeek），动作参数编排器校验后上链（真交易）。
> 用户只做第 0 步（创建任务），其余全自动。

## 2. 架构总览

```
┌──────────┐   POST /demo/sessions   ┌──────────────────────────────┐
│  前端     │ ──────────────────────▶ │  offchain（现有服务）          │
│ 聊天 UI   │ ◀────────────────────── │  ┌────────────────────────┐  │
│          │  GET /demo/sessions/:id  │  │ DemoOrchestrator       │  │
└──────────┘   /messages（轮询）       │  │  - 状态机（每步决策）    │  │
                                      │  │  - LLM 发言生成         │  │
                                      │  │  - 链上动作执行（签名）  │  │
                                      │  │  - 消息落库（demo_msg） │  │
                                      │  └─────────┬──────────────┘  │
                                      │            │                 │
┌──────────┐   DeepSeek API           │            ▼                 │
│  LLM      │ ◀────────────────────── │  ┌────────────────────────┐  │
│ deepseek  │   role prompt + 上下文   │  │ 合约绑定（Job/Reg/Hook）│  │
│ -v4-flash │                         │  └────────────────────────┘  │
└──────────┘                          └──────────────────────────────┘
```

- **编排器跑在 offchain 进程内**（新包 `prismsettle/demo`）——复用现有 DB、配置、合约绑定、密钥注入
- 每步执行：**状态判断 → LLM 生成发言 → 链上动作 → 消息落库** → 下一步（goroutine 顺序执行，天然带"思考延迟"）
- 前端**轮询消息流**（3 秒），新消息追加到聊天区，状态栏联动

## 3. 核心概念

| 概念 | 说明 |
|------|------|
| Demo Session | 一场演示 = 一个新任务从创建到结算的完整过程 |
| Step | 状态机的一步（如 fund / submit / reject / dispute / resolve） |
| Message | 一条聊天消息：{role, content, action, tx_hash, state} |
| Role | buyer（买方 agent）/ provider（接单 agent）/ evaluator（仲裁）/ system（合约/编排器） |

## 4. 状态机设计

任务状态（与合约 `JobState` 对齐）+ 每步"轮到谁"：

```
┌─────────┐  fund(buyer)   ┌────────┐  grab(provider)  ┌──────────┐
│ Created │ ─────────────▶ │ Funded │ ───────────────▶ │ Assigned │
└─────────┘                └────────┘                  └────┬─────┘
                                                            │ submit(provider)
                                                            ▼
                                                    ┌──────────────┐
                                                    │  Submitted   │
                                                    └──────┬───────┘
                                     reject(buyer)◀────────┤
                                     （可循环 N 次）         │ 交付完成判断
                                                            ▼
                              ┌─────────────────────┐  dispute(provider/buyer)
                              │   Disputed          │◀──────────────────┐
                              └─────────┬───────────┘                   │
                             resolve(evaluator)                         │
                                        ▼                               │
                              ┌─────────────────────┐                   │
                              │  DisputeResolved    │                   │
                              └─────────┬───────────┘                   │
                              execute(system)                           │
                                        ▼                               │
                              ┌─────────────────────┐                   │
                              │  Completed/Executed │◀──────────────────┘
                              └─────────────────────┘   complete(buyer) 直接完成路径
```

### 每步决策（编排器核心逻辑）

| 当前状态 | 轮到谁 | 触发动作 | 失败处理 |
|----------|--------|----------|----------|
| Created | buyer agent | createJob + fund | 交易失败 → 消息记录原因，会话终止（标记 failed） |
| Funded | provider agent | grabJob（归属校验） | revert（如被抢先）→ 记录原因 → 会话失败（或换 agent 重试一次） |
| Assigned | provider agent | submit（deliverableHash + proofHash） | revert → 记录 → 重试一次 |
| Submitted | buyer agent | 判断：满意 → complete；不满意 → reject（带意见） | LLM 决策由提示词控制（演示脚本：前 2 次打回，第 3 次仲裁） |
| Submitted（打回后） | provider agent | 改进后 resubmit | 同上 |
| Disputed | evaluator agent | resolveDispute(ruling=1/2) | 演示脚本固定 ruling=2（provider 胜） |
| DisputeResolved | system | executeArbitrationResult（等 60s 公告期） | 等待期间消息"公告期 60s…" |
| Completed | — | 会话结束 | — |

> 打回循环次数由演示脚本控制（`maxRejects` 默认 2，达到后 provider 自动提起仲裁）。
> 仲裁裁定由脚本控制（ruling=2），但**发言内容由 LLM 实时生成**——"真模型，真交易，脚本定走向"。

## 5. LLM 对话生成

### 5.1 调用方式

复用现有 DeepSeek 配置（offchain/config/agents.yaml：`api.deepseek.com/v1`，model `deepseek-v4-flash`）。
编排器直接 HTTP 调 `POST /chat/completions`（不经过 agent 服务，编排器内建轻量 client）。

### 5.2 角色提示词（按 step 组装）

每个角色固定 system prompt + 动态上下文：

```
[system] 你是 PrismSettle 平台的买方 Agent。
你的任务：验收智能合约安全审计交付物。任务金额 5 USDC。
你可以：满意时 complete（评分 0-1），不满意时 reject（附具体意见）。
要求：意见要具体、专业（提重入/溢出/权限等真实审计关注点）。中文发言，1-2 句。

[context]
- 任务状态：Submitted
- 前序对话：provider 提交了 v1（覆盖重入/权限检查）
- 你的历史打回：无
```

LLM 输出结构化 JSON（约束 action + 参数）：

```json
{
  "action": "reject",
  "score": null,
  "reason": "缺少重入漏洞的边界测试用例，请补充",
  "content": "审计报告覆盖了重入和权限检查，但缺少重入边界用例，请补充后重交。"
}
```

### 5.3 动作 → 链上映射

| action | 合约调用 | 签名者 |
|--------|----------|--------|
| create_job + fund | Job.createJob + fundViaToken | BUYER_KEY |
| grab | Job.grabJob(jobId, agentId) | 对应 auditor key |
| submit | Job.submit(jobId, dh, ph) | 对应 auditor key |
| reject | Job.reject(jobId, reasonHash) | BUYER_KEY |
| dispute | Hook.dispute(jobId, reasonHash) | BUYER_KEY 或 auditor key |
| resolve | Hook.resolveDispute(jobId, ruling) | PRISM_EVALUATOR_KEY |
| execute | Job.executeArbitrationResult(jobId) | 任意（用 DEPLOYER_KEY） |

> LLM 输出若缺少/非法 action → 编排器用脚本默认动作兜底（如打回轮次到上限 → dispute）。
> **安全**：LLM 只生成"发言 + 意图"，**动作参数由编排器校验**（金额固定、agentId 固定、reasonHash=keccak(发言)）。

## 6. 链上动作执行

- 复用现有 `chainbinding.NewTransactor`（EIP-1559 已修好：GasPrice=nil + TipCap/FeeCap）
- 每个动作：`Transact → 等回执（60s 超时）→ 记录 tx_hash → 消息落库`
- 交易失败：消息带失败原因（parseGrabErr 复用），会话标记 failed，前端显示
- 多钱包：会话创建时注入 `{buyer: BUYER_KEY, provider: AUDITOR_SENIOR_KEY, evaluator: PRISM_EVALUATOR_KEY}`（从 .env 读，不落库）

## 7. API 设计

### POST /api/v1/prismsettle/demo/sessions
创建一场演示（真实建任务）：

```json
// 请求
{
  "title": "智能合约安全审计",
  "description": "检查重入漏洞与 gas 优化，输出审计报告",
  "amount": "5000000",          // 5 USDC（6 decimals）
  "token": "0x2Bb06A...",        // 默认 USDC
  "provider_agent": "senior",    // 接单 agent：senior/junior/rookie
  "max_rejects": 2               // 打回上限，达到后自动仲裁
}

// 响应
{
  "session_id": "sess_8f3a...",
  "job_id": "0x89b2...",
  "state": "created"
}
```

### GET /api/v1/prismsettle/demo/sessions/:id/messages
轮询消息流（前端每 3 秒）：

```json
{
  "session_id": "sess_8f3a...",
  "job_id": "0x89b2...",
  "state": "submitted",
  "messages": [
    {
      "step": 3,
      "role": "provider",
      "content": "提交审计报告 v1（覆盖重入/权限检查）",
      "action": "submit",
      "tx_hash": "0xdef1...",
      "state": "submitted",
      "created_at": 1786172400
    }
  ]
}
```

### GET /api/v1/prismsettle/demo/sessions/:id
会话状态（含任务链接、结算结果）。

## 8. 数据库设计

新表 `demo_sessions` + `demo_messages`：

```sql
CREATE TABLE demo_sessions (
  id          TEXT PRIMARY KEY,          -- sess_<rand>
  job_id      TEXT NOT NULL,
  title       TEXT,
  amount      TEXT,                       -- wei/units 字符串
  token       TEXT,
  provider_agent TEXT,                    -- senior/junior/rookie
  state       TEXT NOT NULL DEFAULT 'created',
  max_rejects INT DEFAULT 2,
  result      TEXT,                       -- JSON: 最终结算/仲裁信息
  created_at  TIMESTAMPTZ DEFAULT now(),
  finished_at TIMESTAMPTZ
);

CREATE TABLE demo_messages (
  id         BIGSERIAL PRIMARY KEY,
  session_id TEXT REFERENCES demo_sessions(id),
  step       INT NOT NULL,
  role       TEXT NOT NULL,               -- buyer/provider/evaluator/system
  content    TEXT NOT NULL,
  action     TEXT,                        -- fund/grab/submit/reject/dispute/resolve/execute/...
  tx_hash    TEXT,
  state      TEXT,                        -- 消息时的任务状态
  created_at TIMESTAMPTZ DEFAULT now()
);
```

## 9. 前端设计

### 9.1 聊天 UI（任务详情页新 Tab 或区块）

- **消息区**：三方气泡（buyer 左 / provider 右 / evaluator 中紫 / system 居中灰）+ 步骤序号 + tx 链接
- **状态栏**：复用 JobStatusTracker，随消息推进实时切换（当前状态高亮）
- **会话控制**：`开始演示` 按钮（POST 创建会话）→ 消息自动滚动（3 秒轮询）
- **失败提示**：会话 failed 时红色横幅 + 原因

### 9.2 组件

```
components/demo/
  DemoChat.tsx          — 聊天区 + 气泡渲染 + 自动滚动
  DemoStartPanel.tsx    — 参数表单（任务描述/金额/agent/打回次数）+ 开始按钮
  DemoStatusBar.tsx     — 顶部状态联动（复用 JobStatusTracker）
hooks/
  useDemoSession.ts     — 创建会话 + 轮询消息（usePoll）
```

## 10. 配置

```yaml
# offchain/config/dev-local.yaml 追加
demo:
  llm:
    base_url: "https://api.deepseek.com/v1"   # 复用 agents.yaml 同款
    api_key_env: "OPENAI_API_KEY"              # 从环境变量读（就是 DeepSeek key）
    model: "deepseek-v4-flash"
  step_delay_ms: 1500      # 每步额外人为延迟（演示节奏，可调 0）
  announcement_wait: true  # 仲裁结算等 60s 公告期（消息提示）
```

## 11. 开发计划（3 阶段）

| 阶段 | 内容 | 产出 |
|------|------|------|
| P1 后端核心 | 编排器状态机 + LLM client + 链上动作 + 表/API | 命令行跑通一场（curl 建会话→轮询消息→结算） |
| P2 前端 | 聊天 UI + 开始面板 + 轮询 + 状态联动 | 详情页可看完整互动 |
| P3 打磨 | 延迟节奏、失败提示、多场景（直接完成/仲裁/拒绝循环）、演示脚本参数化 | demo day 就绪 |

## 12. 风险与回退

| 风险 | 缓解 |
|------|------|
| LLM 输出不稳定（非法 action/胡言） | 动作由编排器校验+默认值兜底；发言只是展示层 |
| 链上交易失败（RPC 限流/gas） | 重试 1 次 + 失败消息明示；checkpoint commit 可回退 |
| 演示失控（agent 打回太多次） | max_rejects 硬上限 → 强制仲裁 |
| 60s 公告期拖慢演示 | `announcement_wait: false` 时跳过等待直接结算（或消息提示后继续） |
| 多会话并发 | 演示会话串行（同时只允许 1 场），session 级互斥 |

回退点：git commit `ab70634`（本设计开发前的 checkpoint）。
