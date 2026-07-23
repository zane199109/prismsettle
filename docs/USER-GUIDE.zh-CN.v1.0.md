# PrismSettle 用户手册 v1.0

> 面向最终用户（Agent 买家 / Agent 提供方 / Validator / Arbitrator）的完整使用指南，
> 含页面地图、用户旅程、操作步骤、合约配置、FAQ、缺失项反推与 PRD 完善度评估。
>
> 本文档由代码现状反推撰写，与 `docs/PRD.zh-CN.v1.0.md` / `docs/SD.zh-CN.v1.0.md`
> 互为印证。任何 PRD 已定义但代码未实现的项，均在 §9「缺失项分析」中显式列出。

---

## 目录

- [§1 项目简介](#1-项目简介)
- [§2 角色与权限模型](#2-角色与权限模型)
- [§3 页面总览](#3-页面总览)
- [§4 核心用户旅程](#4-核心用户旅程)
- [§5 页面详解](#5-页面详解)
- [§6 钱包与网络配置](#6-钱包与网络配置)
- [§7 合约地址与 ABI](#7-合约地址与-abi)
- [§8 常见问题 FAQ](#8-常见问题-faq)
- [§9 缺失项分析（按优先级）](#9-缺失项分析按优先级)
- [§10 PRD 完善度反推](#10-prd-完善度反推)
- [§11 改进建议（超出 PRD 范畴）](#11-改进建议超出-prd-范畴)

---

## §1 项目简介

**PrismSettle** 是面向 AI Agent 经济的链上信任层，部署在 Monad Testnet 上。
核心价值：

1. **256 分片声誉存储** — 通过 `agentId & 0xFF` 将声誉数据分散到 256 个存储槽，
   消除 Monad OCC 在高并发写场景下的 ~60% 中止率，降至 5% 以下。
2. **双路径验证** — 声誉来自三个 source：
   - `source=0` Validator（质押 ETH 后给 Agent 评分）
   - `source=1` Evaluator（Job 完成后自动评分）
   - `source=2` Arbitration（仲裁裁决后强制扣分）
3. **ERC-8183 核心 4 态状态机** — Job 生命周期：Open → Funded → Submitted →
   Terminal，仲裁作为可选 Hook 不污染核心状态。
4. **Stake-weighted 评分** — Validator 需质押 MIN_STAKE = 5 ETH，作恶可被 slash
   最高 30% 累积声誉。

**当前版本**：v1.0（黑客松提交版）  
**部署链**：Monad Testnet (chainId 10143)  
**演示前端**：`http://172.21.89.8:3001`（dev 服务器）

---

## §2 角色与权限模型

PrismSettle 有四类用户角色，**同一个钱包地址可以同时扮演多个角色**：

| 角色 | 主要操作 | 入口页面 | 是否需质押 |
|---|---|---|---|
| **Visitor（游客）** | 浏览 Marketplace、查看 Agent 详情、查看性能对比、查看链上事件 | `/`、`/agents`、`/jobs`、`/events`、`/perf` | 否 |
| **Agent Buyer（买家）** | 注册 Agent、创建 Job + 注资、调用 Agent、发起仲裁 | `/agents`（注册）、`/jobs/new`（创建 Job）、`/jobs/[id]`（追踪+仲裁） | 否 |
| **Agent Provider（提供方）** | 提交交付物、查看自己 Agent 的声誉曲线 | `/jobs/[id]`（提交交付物）、`/agents/[id]`（声誉监控） | 否 |
| **Validator** | 质押 ETH、提交 `submitValidation`、解除质押 + 提取 | `/validator` | 是（5 ETH） |
| **Arbitrator** | 调用 `Hook.resolveDispute(jobId, ruling)` 裁决仲裁 | ⚠️ **当前无专用 UI**，需通过 Etherscan 调用 | 否 |

> **注意**：当前实现中 **Arbitrator 角色没有专用前端页面**，详见 §9 P1 缺失项 #4。

---

## §3 页面总览

前端共 11 个路由，覆盖 PRD FR-M01 ~ FR-M13 + FR-JM01 ~ FR-JM06：

| 路由 | 页面名 | 主要功能 | 对应 PRD |
|---|---|---|---|
| `/` | Landing Page | 价值主张 + Hero + Problem/Solution + Live Preview + Tech Stack + CTA | FR-M01 |
| `/dashboard` | Dashboard 控制台 | Metrics + PrismHologram + ShardHeatmap + ValidationFeed + AgentLeaderboard | FR-M07/M08 |
| `/agents` | Agent Marketplace | Agent 列表 + 搜索 + 排序 + 注册入口 | FR-M02/M06 |
| `/agents/[id]` | Agent 详情 | 声誉曲线 + 分片活动 + FailureCounter + AgentInvokeBox | FR-M03/M04/M09/M12 |
| `/jobs` | Job 列表 | 所有 Job 表格 + 状态筛选 + 分页 | FR-A08 |
| `/jobs/new` | 创建 Job | 选 Agent + 金额 + deadline + TrustGate 预检查 + 链上 createJob + fund | FR-JM01/M13 |
| `/jobs/[id]` | Job 详情 | StatusTracker + DeliverableSubmit + DisputePanel + FundFlowChart | FR-JM02/JM03/JM04/JM05/JM06 |
| `/validator` | Validator Console | Leaderboard + Records + SlashHistory + 个人质押（Stake/Unstake/Withdraw） | FR-M05 |
| `/events` | 全局事件流 | 16 种 event_type 筛选 + 分页 + reorg 标记 | FR-M07/M10 |
| `/perf` | 性能基准 | V0V1Comparison + ShardHeatmap + ReorgAwareFeed | FR-M08b |
| `/not-found` | 404 | shadcn 语义色的 404 页面 | — |

**导航栏**（[PageHeader](file:///home/administrator/Documents/trae_projects/PrismSettle/frontend/components/PageHeader.tsx)）：
`Home / Dashboard / Agents / Jobs / Validator / Events / Performance`

---

## §4 核心用户旅程

### 旅程 A — Agent 买家（创建 Job → 完成）

```
1. 访问 /                    → 看到价值主张 + Live Preview
2. 点 "Browse Agents"        → 进入 /agents
3. 按 A/B/C/D 评级筛选        → 选中一个 A 级 Agent
4. 进入 /agents/[id]          → 看声誉曲线 + Shard Slot + InvokeBox
5. 点 "Create Job with this agent"
   → 跳转 /jobs/new?agent=...
6. 填金额 + deadline + 任务文本
7. TrustGate 预检查           → 返回 ALLOW / DENY / REQUIRE_VALIDATION
   - ALLOW         → 直接创建
   - DENY          → 拦截，提示换 Agent
   - REQUIRE_VALIDATION → 提示挂载 ArbitrationHook
8. 钱包签名 createJob + fund  → 链上交易
9. 跳转 /jobs/[jobId]         → 看 StatusTracker
10. 等 Provider 提交交付物    → 状态 Submitted
11.（可选）发起仲裁           → DisputePanel
12. 完成或退款                → Terminal
```

### 旅程 B — Agent Provider（接单 → 提交）

```
1. 在 /jobs 列表筛选 Pending 状态
2. 找到自己被 assign 的 Job（通过 creator/evaluator 字段识别）
3. 进入 /jobs/[id]
4. 在 "Submit Deliverable" 表单填 deliverableHash + proofHash
5. 点 "Submit Deliverable"   → 调 Job.submit()
6. 等 Evaluator 评分          → 自动 submitValidation source=1
7. 等 aggregateEpoch 触发     → 声誉更新
8. 在 /agents/[myAgentId] 看声誉曲线变化
```

### 旅程 C — Validator（质押 → 验证）

```
1. 进入 /validator
2. 看 ValidatorLeaderboard（确认自己未在榜单上 → 准备加入）
3. 在 "Your Stake" 区点 "Stake"
4. 输入金额（≥ 5 ETH）       → 调 Registry.stake()
5. ⚠️ 当前实现缺 submitValidation 入口 → 见 §9 P0 缺失项 #2
   （临时方案：通过 Etherscan 直接调用 Registry.submitValidation）
6. 在 ValidationRecords 表看自己的提交记录
7. 不想做了 → 点 "Unstake"   → 进 7 天锁定期
8. 7 天后 → 点 "Withdraw"    → 取回 ETH
```

### 旅程 D — Arbitrator（裁决争议）

```
1. 监听 Disputed 事件（在 /events 筛选 PRISM_DISPUTED）
2. ⚠️ 当前无专用 Arbitrator 面板 → 见 §9 P1 缺失项 #4
   （临时方案：通过 Etherscan 调用 Hook.resolveDispute(jobId, ruling)）
3. ruling=1 退款给买家 + Agent 被 slash（max(0.2e18, score×30%)）
4. ruling=2 放款给 Provider
5. ruling=0 revert（不允许）
```

---

## §5 页面详解

### 5.1 Landing Page `/`

**目的**：5 秒内向评委/投资人/首次访客传达价值主张。

**结构**（6 个 section，自上而下）：
1. **Hero** — 大标题 PrismSettle + 副标题 + 2 个 CTA（Browse Agents / View Dashboard）+ PrismHologram 装饰 + 3 个数据信号（256 shards / ~5% abort / 3 sources）
2. **ProblemSection** — 3 个痛点卡片：信任碎片化 / Sybil 攻击 / OCC 写冲突 60%
3. **SolutionSection** — 3 个方案支柱：256-shard / 双路径验证 / stake-weighted，每张卡片有 metric 标签
4. **LivePreview** — 左 Top 3 Agents / 右 Active 3 Jobs（点击进详情，空数据时显示 Empty State）
5. **TechStackSection** — 3 个 benchmark 卡片（500 并发 / 60%→5% / <5s）+ 6 项技术栈
6. **CTA Footer** — Browse Agents / Post Job / Source

**操作**：
- 点击任意 CTA → 跳转到对应页面
- 点击 LivePreview 中的 Agent/Job 卡片 → 跳转到详情页

### 5.2 Dashboard `/dashboard`

**目的**：运营者控制台，展示网络整体状态。

**组件**：
- 3 张 MetricCard（Total Validations / Registered Agents / Active Shards，含 count-up 动画）
- PrismHologram（旋转 3D 棱镜，显示聚合分数 0.8e18）
- ShardHeatmap（16×16 网格，每个 shard 有写入时脉冲发光）
- ValidationFeed（实时事件流，新事件绿色 flash 滑入）
- AgentLeaderboard（Top 5 Agent 排行）

**数据来源**：所有组件自轮询 `/api/v1/prismsettle/*`，间隔 4-6 秒。

### 5.3 Agent Marketplace `/agents`

**功能**：
- 顶部搜索框（按 agentId / owner / endpoint 模糊匹配）
- 排序下拉（Score / Recent）
- 网格卡片展示，每张含：Agent ID、owner、声誉分数、A/B/C/D 评级 Badge、endpoint 链接、注册区块
- 底部可折叠的 "Register a new agent" 表单（FR-M06）

**注册 Agent**：
1. 展开 "Register a new agent" 折叠面板
2. 填 agentId（建议用 keccak256 的哈希）+ metadata（JSON）+ endpoint URL
3. 点 Register → 钱包签名 Registry.registerAgent()
4. 等待交易确认 → 列表中出现新 Agent

> ⚠️ **缺失**：PRD FR-M02 要求"能力标签筛选"（DeFi/Data/Translation/Eval），当前未实现。详见 §9 P1 #1。

### 5.4 Agent 详情 `/agents/[id]`

**组件**：
- Header 卡片：Agent ID + 评级 Badge + owner + 注册时间 + endpoint + metadata JSON
- 左：ScoreHistoryChart — Recharts 三条 Line 按 source 着色（Validator=blue / Job=purple / Arbitration=red）+ Legend + CartesianGrid
- 右：ShardHeatmap（该 Agent 触达的分片活动）
- 左：AgentFailureCounterUI（近期调用失败次数，FR-M12 隐性差评）
- 右：AgentInvokeBox（一键调用，含 Sad Path 重试/切换）
- 底部：Create Job with this agent / Stake as validator 快捷链接

**一键调用流程**（FR-M04 + UC-01）：
1. 在 textarea 输入任务描述
2. 点 Invoke（或 ⌘/Ctrl+Enter）
3. 30s 超时；成功 → 绿色结果面板 + toast；失败 → 红色面板 + Retry / Switch Agent 按钮
4. 低分 Agent（score < 0.3）会显示 amber 警告条

> ⚠️ **缺失**：PRD FR-M03 要求显示 "Shard Slot: #{agentId & 0xFF}" + "Expected Latency: < 1s" 性能标签，当前未实现。详见 §9 P0 #1。

### 5.5 Job 列表 `/jobs`

**功能**：
- 顶部搜索框（按 jobId / creator / evaluator / shard ID 过滤）
- 状态下拉（All / Pending / Submitted / Completed / Disputed / Resolved）
- 表格列：Job ID / Status Badge / Shard # / Creator / Evaluator / Created time / View 按钮
- 行点击或 View 按钮 → 跳转 `/jobs/[id]`
- 底部分页（每页 15 条）+ 空状态引导（Create the first job）

### 5.6 创建 Job `/jobs/new`

**流程**：
1. 选择 Agent（从下拉或 URL 参数 `?agent=...`）
2. 输入金额（ETH）+ deadline（秒）+ 任务文本
3. **TrustGate 预检查**（FR-M13）：调 `/api/v1/prismsettle/trust?agentId=...&amount=...`
   - `ALLOW` → 显示绿色 "Proceed to create"
   - `DENY` → 显示红色 "Pick another agent" + 阻塞提交
   - `REQUIRE_VALIDATION` → 显示 amber "ArbitrationHook recommended"
4. 点 "Create + Fund" → 钱包签名 `Job.createJob(...)` + `Job.fund(...)`
5. 交易确认后跳转 `/jobs/[jobId]`

### 5.7 Job 详情 `/jobs/[id]`

**组件**：
- Header：Job ID + 当前状态 + terminal 标记 + FundingPathBadge
- JobStatusTracker（FR-JM02）：核心 4 态进度条 + Hook 仲裁态独立区块
- DeliverableSubmit（FR-JM03）：仅在 Funded/Assigned 状态显示
  - 填 deliverableHash (bytes32) + proofHash (bytes32)
  - 点 Submit Deliverable → 调 `Job.submit(jobId, deliverableHash, proofHash)`
- DisputePanel（FR-JM05/JM06）：仅在 Submitted/Disputed/DisputeResolved 状态显示
  - Submitted → 显示 "Raise Dispute" 表单（填 reasonHash bytes32）
  - Disputed → 显示 "等待裁决" 提示
  - DisputeResolved → 显示裁决结果（ruling=1 refund）
- FundFlowChart（FR-JM04）：通过 `useReadContract(getJobState)` 读取真实链上 Job 状态，渲染 buyer → Job contract → provider 资金流向

### 5.8 Validator Console `/validator`

**顶部 3 个数据区块**：
- **ValidatorLeaderboard**（Top 10）：基于 PRISM_STAKED 事件聚合，按总质押量排序。列：# / Address / Stake ETH / Stake Count / Last Active
- **ValidationRecords**（最近 10）：基于 PRISM_VALIDATION_SUBMITTED 事件。列：Agent / Score / Source Badge / Job / Block / Time
  - Source Badge 颜色：Validator=blue / Evaluator=purple / Arbitration=red
- **SlashHistory**（最近 10）：基于 PRISM_SLASHED 事件。列：Agent / Penalty Badge / Reason / Block / Time

**底部个人质押操作**：
- 3 张 Stat 卡片：Your Stake / Pending Unstake / Wallet
- 3 个 Tab：Stake / Unstake / Withdraw
  - **Stake**：输入金额（≥5 ETH），调 `Registry.stake()`
  - **Unstake**：输入金额，调 `Registry.unstake()`，进入 7 天锁定期
  - **Withdraw**：调 `Registry.withdraw()`，仅在锁定期过后可调用
- 显示交易 hash + confirming 状态
- 钱包未连接 / 错链时显示提示

> ⚠️ **缺失**：PRD FR-M05 要求 "提交验证表单"（Validator 给 Agent 评分的入口，对应 UC-02），当前未实现。详见 §9 P0 #2。  
> ⚠️ **缺失**：PRD FR-M05 要求 "收益追踪"，当前未实现。详见 §9 P1 #3。

### 5.9 全局事件流 `/events`

**功能**：
- Event type 下拉（16 种：PRISM_AGENT_REGISTERD / PRISM_VALIDATION_SUBMITTED / PRISM_AGGREGATED / PRISM_STAKED / PRISM_UNSTAKE_STARTED / PRISM_UNSTAKE_WITHDRAWN / PRISM_SLASHED / PRISM_JOB_CREATED / PRISM_JOB_FUNDED / PRISM_JOB_ASSIGNED / PRISM_JOB_SUBMITTED / PRISM_JOB_COMPLETED / PRISM_JOB_REFUNDED / PRISM_DISPUTED / PRISM_DISPUTE_RESOLVED / All）
- 表格列：Block / Event Badge / From / To / Value / Tx link / Time
- Reorg-affected 行显示 amber 背景 + ↻ 标记（从 `extra` JSON 字段解析）
- 每页 30 条 + 分页

### 5.10 性能对比 `/perf`

**组件**：
- V0V1Comparison（FR-M08b）：分组柱状图对比 V0（single-slot）vs V1（256-shard），含 500 并发、中止率、平均延迟、区块时间指标卡片
- ShardHeatmap：500 并发写入时的分片分散效果动画
- ReorgAwareFeed：reorg 事件流

---

## §6 钱包与网络配置

### 6.1 支持的钱包

通过 RainbowKit 集成，支持：
- MetaMask（推荐）
- WalletConnect（移动端）
- Coinbase Wallet
- 注入式钱包

### 6.2 Monad Testnet 网络配置

**自动添加**：连接钱包时，RainbowKit 会自动弹出 "Add Monad Testnet" 提示。

**手动添加**：
- Network Name: `Monad Testnet`
- Chain ID: `10143`
- RPC URL: `https://testnet-rpc.monad.xyz`
- Block Explorer: `https://testnet.monadexplorer.com`
- Currency Symbol: `MON`
- Faucet: `https://faucet.monad.xyz`

### 6.3 测试币获取

- **MON（gas 用）**：从 `https://faucet.monad.xyz` 领取
- **testnet USDC（Job 注资用）**：从 Circle faucet 领取（需 EIP-3009 兼容版本）

### 6.4 常见钱包错误

| 错误 | 原因 | 解决 |
|---|---|---|
| "Wrong network" | 钱包连接到其他链 | 在钱包中切换到 Monad Testnet |
| "Insufficient balance" | MON 不足以付 gas | 从 faucet 领取 |
| "UserRejectedRequestError" | 用户在钱包中拒绝了签名 | 重新发起交易 |
| "Transaction reverted" | 合约 require 失败 | 检查输入参数（如 bytes32 格式、金额 ≥ MIN_STAKE） |

---

## §7 合约地址与 ABI

### 7.1 合约地址

合约地址集中管理在 [lib/contracts.ts](file:///home/administrator/Documents/trae_projects/PrismSettle/frontend/lib/contracts.ts)，通过环境变量注入：

```bash
# frontend/.env.local
NEXT_PUBLIC_REGISTRY_CONTRACT_ADDRESS=0x...
NEXT_PUBLIC_JOB_CONTRACT_ADDRESS=0x...
NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS=0x...
NEXT_PUBLIC_RPC_URL=https://testnet-rpc.monad.xyz
NEXT_PUBLIC_WC_PROJECT_ID=...  # WalletConnect projectId
```

### 7.2 合约部署顺序

按 AGENTS.md「部署顺序」硬约束：
1. **Hook**（with job=0）→ 得到 hookAddr
2. **Job**（with hook address）→ 得到 jobAddr
3. **Hook.setJobContract(Job)** → 完成 Hook ↔ Job 双向引用

### 7.3 核心 ABI 方法

**PrismSettleRegistry**：
- `registerAgent(agentId, metadata, endpoint)` — 注册 Agent
- `stake()` — Validator 质押 ETH（payable）
- `unstake(amount)` — 发起解除（进入 7 天锁）
- `withdraw()` — 锁定期过后提取
- `submitValidation(agentId, score, proofHash, jobId, source)` — 提交验证（source: 0=Validator, 1=Evaluator, 2=Arbitration）
- `aggregateEpoch(shardId)` — 聚合某分片的 ValidationRecords
- `getScore(agentId)` — 读取声誉（含 lazy decay 计算）
- `stakeInfo(address)` → `(amount, pendingUnstake, unstakeAt)` — 查询个人质押

**PrismSettleJob**：
- `createJob(agentId, deadline, hook)` → returns jobId
- `fund(jobId, amount)` — ERC-20 注资
- `assignJob(jobId, provider)` — 指派 Provider
- `submit(jobId, deliverableHash, proofHash)` — Provider 提交交付物
- `complete(jobId)` — 买家确认完成 → 放款给 Provider
- `claimRefund(jobId)` — 买家申请退款（需过 deadline 或 Hook.ruling=1）
- `getJobState(jobId)` → `(state, buyer, provider, amount, deliverableHash, proofHash, deadline, hook)`

**ArbitrationHook**：
- `dispute(jobId, reasonHash)` — 发起仲裁
- `resolveDispute(jobId, ruling)` — 裁决（ruling: 1=refund, 2=release, 0=revert）
- `setJobContract(jobAddr)` — 设置 Job 合约引用

---

## §8 常见问题 FAQ

### Q1：为什么我看不到任何 Agent / Job 数据？
A：后端 offchain 服务（端口 9527）未启动。前端会显示 Skeleton 加载态或 Empty State。启动 offchain 后数据自动填充。

### Q2：为什么 Reputation History 图表显示 "No reputation history yet"？
A：要么该 Agent 没有验证记录，要么后端 `/reputation/history` 返回的数据为空。可在 `/events` 页面筛选 `PRISM_VALIDATION_SUBMITTED` 确认是否有数据。

### Q3：Job 创建后多久能被 Provider 接单？
A：取决于 mock Provider 是否启用。当前演示流程需要 mock Provider goroutine 自动监听 Assigned 事件并调 `Job.submit()`，详见 offchain 服务配置。

### Q4：Validator 的 7 天锁定期可以提前解除吗？
A：不行。`unstake` 后必须等满 7 天才能 `withdraw`，这是合约强制的安全机制。

### Q5：发起仲裁后多久能拿到裁决？
A：取决于 Arbitrator 何时调用 `Hook.resolveDispute(jobId, ruling)`。当前无专用 Arbitrator UI，需通过 Etherscan 手动调用。

### Q6：我可以同时是 Validator 和 Agent Buyer 吗？
A：可以。同一个钱包地址可以同时质押（Validator 角色）+ 创建 Job（Buyer 角色）+ 注册 Agent（Provider 角色）。

### Q7：为什么我的交易一直 "confirming..."？
A：Monad Testnet 出块时间约 0.4s，正常情况 1-2 秒确认。如果超过 30 秒，可能是 RPC 不稳定，检查 `NEXT_PUBLIC_RPC_URL` 是否可达。

### Q8：TrustGate 返回 REQUIRE_VALIDATION 但我还是创建了 Job，会怎样？
A：Job 仍会创建成功，但不挂载 ArbitrationHook。发生争议时无法走仲裁流程。建议在创建 Job 时同时部署/挂载 Hook。

---

## §9 缺失项分析（按优先级）

> 本节对照 PRD FR-M01 ~ FR-M13 + FR-JM01 ~ FR-JM06 + UC-01 ~ UC-06，列出代码现状缺失的功能。
> 优先级：**P0** 阻塞核心 demo 流程；**P1** 影响 UX 完整性；**P2** 锦上添花。

### P0 缺失项（阻塞核心 demo）

#### P0-1：Agent 详情缺性能标签（违反 FR-M03）
**PRD 要求**：FR-M03 明确要求 Agent 详情页显示 "Shard Slot: #{agentId & 0xFF}" + "Expected Latency: < 1s" 性能标签，用 mono 字体 + 紫色高亮，让分片价值传导显性化。  
**当前实现**：[agents/[id]/page.tsx](file:///home/administrator/Documents/trae_projects/PrismSettle/frontend/app/agents/[agentId]/page.tsx) 未显示性能标签。  
**影响**：评委无法直观看到 256-shard 价值传导，削弱技术深度叙事。  
**修复**：在 Header 卡片下方加 2 个 Badge：`Shard Slot: #N`（N = agentId & 0xFF）+ `Expected Latency: <1s`。

#### P0-2：Validator 缺 submitValidation 入口（违反 FR-M05 + UC-02）
**PRD 要求**：FR-M05 要求 Validator Console 含"提交验证表单"；UC-02 描述 Validator 调用 `Registry.submitValidation(agentId, score, proofHash, jobId, source)` 的完整流程。  
**当前实现**：[/validator](file:///home/administrator/Documents/trae_projects/PrismSettle/frontend/app/validator/page.tsx) 只有 Stake/Unstake/Withdraw，无 submitValidation 表单。  
**影响**：Validator 角色不完整，无法演示 source=0 路径，整个双路径验证叙事缺一条腿。  
**修复**：在 Validator Console 加第 4 个 Tab "Submit Validation"，表单字段：agentId / score (0..1e18) / proofHash (bytes32) / jobId (uint256) / source（固定为 0）。调用 `Registry.submitValidation(...)`。

#### P0-3：移动端导航折叠缺失（违反 FR-M10）
**PRD 要求**：FR-M10 P2 要求"关键页面可读"（移动端基础适配）。  
**当前实现**：[PageHeader.tsx](file:///home/administrator/Documents/trae_projects/PrismSettle/frontend/components/PageHeader.tsx) 第 32 行 `nav` 用 `hidden md:flex`，在 md 以下完全隐藏，没有 hamburger 菜单替代。  
**影响**：手机/平板访问时无法跳转页面。  
**修复**：在 md 以下显示 hamburger 图标，点击展开 Sheet/Drawer 菜单。可复用 shadcn Sheet 组件。

### P1 缺失项（影响 UX 完整性）

#### P1-1：Agent 列表缺能力标签筛选（违反 FR-M02）
**PRD 要求**：FR-M02 要求 Agent 列表支持"能力标签筛选"。SD 提到 4 类 Agent：DeFi / Data Labeling / Translation / Eval。  
**当前实现**：[/agents](file:///home/administrator/Documents/trae_projects/PrismSettle/frontend/app/agents/page.tsx) 只有搜索 + Score/Recent 排序，无标签筛选。  
**修复**：在 AgentVO 类型加 `category` 字段（从 metadata JSON 解析），列表顶部加 4 个 Toggle Button 筛选。

#### P1-2：Agent 详情缺 Validator 评价展示（违反 FR-M03）
**PRD 要求**：FR-M03 要求 Agent 详情页显示 "Validator 评价"（Validator 对该 Agent 的文字评价，非分数）。  
**当前实现**：未实现。当前只有 score 数值 + 声誉曲线，无评价文本。  
**影响**：Agent 详情页信息密度不足，评委看不到"同行评议"信号。  
**修复**：在 ScoreHistoryChart 下方加 "Validator Reviews" 区块，从 `/events?event_type=PRISM_VALIDATION_SUBMITTED&to=agentId` 聚合，显示 Validator 地址 + 评价文本（从 extra JSON 解析）+ 时间。

#### P1-3：Validator 缺收益追踪（违反 FR-M05）
**PRD 要求**：FR-M05 要求 Validator Console 含"收益追踪（mock）"。  
**当前实现**：未实现。  
**修复**：加一个 Stat 卡片"累计奖励"，从 ValidationRecords 聚合该 Validator 的提交次数 × 0.001 ETH（mock 单价）。标注 "mock"。

#### P1-4：Arbitrator 角色无专用 UI
**PRD 状况**：PRD UC-06 描述了仲裁流程，但未明确 Arbitrator 是谁、如何调用 resolveDispute。  
**当前实现**：完全没有 Arbitrator UI。  
**影响**：演示仲裁流程时只能跳到 Etherscan 手动调用，演示流畅度差。  
**修复**：新增 `/arbitrator` 路由，列出所有 Disputed 状态的 Job，点击进入裁决面板，调 `Hook.resolveDispute(jobId, ruling)`。ruling 用 RadioGroup 选择 1/2（禁止 0）。

#### P1-5：缺用户个人页面 `/me`
**PRD 状况**：PRD 假定钱包即身份，但未定义"我的页面"。  
**当前实现**：无 `/me` 或 `/profile` 路由。  
**影响**：用户无法一站式查看"我注册的 Agents / 我发布的 Jobs / 我的 Stake / 我的交易历史"。  
**修复**：新增 `/me` 路由，基于当前钱包地址聚合：
- My Agents（filter agents.owner == address）
- My Jobs（filter jobs.creator == address）
- My Stake（Registry.stakeInfo(address)）
- My Validations（filter events.from == address && event_type=PRISM_VALIDATION_SUBMITTED）
- My Slash History（filter events.to == address && event_type=PRISM_SLASHED）

#### P1-6：缺 Dispute 列表页
**PRD 状况**：PRD FR-JM05/JM06 只定义了单个 Job 的 Dispute 面板，没有全局 Dispute 列表。  
**当前实现**：无 `/disputes` 路由。  
**影响**：Arbitrator 无法快速浏览所有待裁决的 Dispute。  
**修复**：新增 `/disputes` 路由，表格列：Job ID / Disputer / Reason Hash / Block / Time / Status / Resolve 按钮。复用 `/events?event_type=PRISM_DISPUTED`。

### P2 缺失项（锦上添花）

#### P2-1：缺全局搜索
**状况**：PRD 未定义。  
**修复**：在 PageHeader 中间加一个搜索框，支持 Agent ID / Job ID / Tx Hash / Address / Block # 跳转。

#### P2-2：缺通知中心
**状况**：PRD 未定义（NFR-UX03 提了"WebSocket 实时推送"但未实现）。  
**修复**：在 PageHeader 右上角加一个铃铛图标，下拉显示最近 10 条与当前钱包相关的事件（你接单了 / 你的 Agent 被 slash / 你的 Job 已完成）。

#### P2-3：缺 Validator 详情页
**状况**：PRD 未定义。  
**修复**：点击 ValidatorLeaderboard 任一行 → 进入 `/validators/[address]`，展示该 Validator 的所有质押记录 + 验证记录 + Slash 记录 + 累计收益。

#### P2-4：缺合约地址公示页
**状况**：PRD 未定义。  
**修复**：新增 `/contracts` 路由，展示 3 个合约地址（Registry / Job / Hook）+ Etherscan 链接 + ABI 下载链接 + 源码链接。

#### P2-5：缺 API 文档页面
**状况**：PRD FR-A01~A12 定义了 12 个 REST API，但无用户可见的 API 文档页面。  
**修复**：新增 `/docs/api` 路由，展示 12 个 endpoint 的 method / path / params / response schema + curl 示例。

#### P2-6：缺系统状态页
**状况**：PRD FR-A05 定义了 `/health`，但前端无展示。  
**修复**：新增 `/status` 路由，展示后端 /health 状态 + RPC 延迟 + 链上最新区块 + 数据库连接状态。

#### P2-7：缺 FAQ 页面
**状况**：PRD 未定义。  
**修复**：新增 `/faq` 路由，把本文档 §8 的 8 个 Q&A 渲染成可搜索的 Accordion。

#### P2-8：Reputation 分布直方图
**状况**：PRD 未定义。  
**修复**：在 Dashboard 加一个直方图卡片，按 0.0-0.2 / 0.2-0.4 / 0.4-0.6 / 0.6-0.8 / 0.8-1.0 分桶统计 Agent 数量。

#### P2-9：Unstake 倒计时可视化
**状况**：PRD 未定义。  
**修复**：在 Validator Console 的 "Pending Unstake" 卡片加一个倒计时进度条，显示"还需 X 天 Y 小时才能 withdraw"。

#### P2-10：键盘导航 + aria-label 完整性
**状况**：NFR-UX04 要求 WCAG AAA。  
**当前**：skip-link + focus-visible 已加，但部分交互组件（如 DisputePanel 的 RadioGroup、ValidatorLeaderboard 的行点击）缺 aria-label。  
**修复**：审计所有交互组件，补 aria-label + role + tabIndex。

---

## §10 PRD 完善度反推

> 本节反向评估 `docs/PRD.zh-CN.v1.0.md` 的完善度，列出 PRD **未覆盖但实际需要**的方面。

### 10.1 PRD 覆盖良好的方面

- ✅ **核心功能完整**：FR-M01 ~ FR-M13 + FR-JM01 ~ FR-JM06 共 19 个 marketplace 功能点全部有定义
- ✅ **API 层完整**：FR-A01 ~ FR-A12 共 12 个 REST endpoint 全部有定义 + 验收标准
- ✅ **合约层完整**：FR-C01 ~ C12 + FR-J01 ~ J11 + FR-T01 ~ T06 全部有定义
- ✅ **用户故事 + 用例完整**：US-01 ~ US-15 + UC-01 ~ UC-06 覆盖所有核心场景
- ✅ **非功能需求清晰**：NFR-MN / NFR-UX / NFR-EV / NFR-C / NFR-M / NFR-OBS / NFR-S 七大类
- ✅ **验收标准量化**：每个 FR 都有验收标准（如 "首屏 < 2s"、"p99 < 200ms"）

### 10.2 PRD 未覆盖或定义不充分的方面

#### 缺失 1：Arbitrator 角色定义
**问题**：UC-06 描述了仲裁流程，但 PRD 未明确：
- Arbitrator 是谁？是合约 deployer？是 multisig？是 DAO？
- Arbitrator 如何调用 `resolveDispute`？有权限控制吗？
- Arbitrator 有无专用 UI？
- 多 Arbitrator 投票机制如何？

**影响**：当前实现完全缺 Arbitrator UI（§9 P1-4），且合约层 `resolveDispute` 的权限控制未在 PRD 中规范。

**建议补充**：在 PRD §4 增加 "4.14 Arbitrator 角色" 章节，定义：
- 权限模型（建议用 OpenZeppelin AccessControl，role=ARBITRATOR_ROLE）
- UI 入口（`/arbitrator`）
- 多 Arbitrator 投票机制（V2 范畴）

#### 缺失 2：用户身份与"我的页面"
**问题**：PRD 假定钱包地址即身份，但没有定义用户个人页面。所有页面都是"全局视角"，没有"我的视角"。

**影响**：用户无法一站式查看自己的 Agents / Jobs / Stakes / Validations。

**建议补充**：在 PRD §4.11 Marketplace 应用层增加：
- FR-M14 「我的页面」 `/me`：聚合当前钱包的所有实体（Agents / Jobs / Stakes / Validations / Slashes）
- FR-M15 「通知中心」：与当前钱包相关的事件订阅

#### 缺失 3：搜索功能
**问题**：PRD 只在 FR-M02 提到 Agent 列表搜索，未定义全局搜索。

**影响**：用户无法快速跳转到特定 Agent / Job / Tx。

**建议补充**：FR-M16 「全局搜索」：顶部搜索框，支持 Agent ID / Job ID / Tx Hash / Address / Block # 模糊匹配跳转。

#### 缺失 4：合约地址公示
**问题**：PRD 未要求公示合约地址 + ABI + 源码链接。

**影响**：用户无法验证合约源码，削弱信任叙事（一个号称"trust layer"的产品连自己的合约都没公示）。

**建议补充**：FR-M17 「合约公示页」 `/contracts`：展示 3 个合约地址 + Etherscan 链接 + ABI 下载 + 源码链接。

#### 缺失 5：API 文档
**问题**：PRD FR-A01 ~ A12 定义了 API，但未要求面向开发者的 API 文档页面。

**影响**：开发者无法快速上手集成。

**建议补充**：FR-A13 「API 文档页」 `/docs/api`：展示所有 endpoint 的 method / path / params / response schema + curl 示例。

#### 缺失 6：错误恢复与 Sad Path 完整性
**问题**：UC-01 详细定义了一键调用的 Sad Path（超时/失败），但其他 UC（UC-02~06）的 Sad Path 未充分定义。

**缺失场景**：
- UC-02 Validator submitValidation 失败（如 Epoch 配额超限）→ 无恢复路径
- UC-05 Job 创建失败（如 TrustGate DENY）→ 无替代方案
- UC-06 Arbitrator 超时不裁决 → 无超时机制

**建议补充**：每个 UC 补充 "Sad Path" 子章节，明确失败时的 UI 反馈 + 用户可采取的恢复动作。

#### 缺失 7：移动端适配具体规范
**问题**：NFR-UX01 ~ UX06 提到移动端，但只说"关键页面可读"，未定义：
- 哪些页面是"关键页面"
- 移动端导航如何处理（hamburger？bottom nav？）
- 触摸目标尺寸（WCAG 建议 44×44px）
- 移动端是否支持钱包连接（WalletConnect 深链）

**建议补充**：NFR-UX07 「移动端规范」：明确关键页面清单（Landing / Agents / Jobs / Agent 详情 / Job 详情）+ 导航方案（hamburger Sheet）+ 触摸目标 ≥ 44px。

#### 缺失 8：可观测性面向用户
**问题**：NFR-OBS01 ~ OBS03 定义了结构化日志 + 健康检查，但都是后端面向运维，无用户可见的健康面板。

**影响**：用户无法判断"为什么我的数据没显示"是后端挂了还是真的没数据。

**建议补充**：FR-M18 「系统状态页」 `/status`：展示后端 /health + RPC 延迟 + 链上最新区块 + 数据库连接状态。

#### 缺失 9：Evaluator 角色与 LLM 配置
**问题**：PRD FR-E01 ~ E14 定义了 Evaluator 行为，但未明确：
- Evaluator 是后端服务还是独立 Agent？
- LLM 模型如何配置（默认 gpt-4o-mini）？
- LLM 调用失败时如何降级？
- Evaluator 的延迟如何监控？

**现状**：代码已实现（offchain/prismsettle/evaluator），但 PRD 文档对架构描述不足。

**建议补充**：在 SD（不是 PRD）补充 "Evaluator 架构" 章节，明确 LLM 配置 + 降级策略 + 熔断机制（当前用 HALF_OPEN 熔断器）。

#### 缺失 10：x402 集成细节
**问题**：PRD FR-AP06 ~ AP09 定义了 x402 支付流程，但当前实现走 ERC-20 path，x402 path 未启用。PRD 未明确：
- x402 何时启用？
- x402 失败时如何降级到 ERC-20？
- Facilitator URL 配置在哪里？

**建议补充**：在 SD 补充 x402 集成状态：V1 范围只实现 ERC-20 path，x402 path 留到 V2。

---

## §11 改进建议（超出 PRD 范畴）

> 以下建议**超出 PRD v1.0 范畴**，但能让项目更完善。按价值/成本比排序。

### 11.1 高价值低成本（建议立即做）

#### A. Agent 详情加 "Shard Slot" 性能标签（对应 §9 P0-1）
工作量：1 个 Badge 组件 + 1 行代码。价值：技术叙事显性化。

#### B. Validator Console 加 submitValidation Tab（对应 §9 P0-2）
工作量：1 个表单组件 + 1 个 Tab。价值：补全双路径验证叙事。

#### C. 移动端 hamburger 导航（对应 §9 P0-3）
工作量：复用 shadcn Sheet 组件，~50 行代码。价值：移动端可用。

#### D. Agent 列表加能力标签筛选（对应 §9 P1-1）
工作量：4 个 Toggle Button + 1 个 filter 函数。价值：用户能快速找到所需 Agent 类型。

### 11.2 中价值中成本（建议后续迭代）

#### E. `/me` 个人页面（对应 §9 P1-5）
工作量：1 个新路由 + 5 个聚合查询组件。价值：用户粘性。

#### F. `/arbitrator` 仲裁面板（对应 §9 P1-4）
工作量：1 个新路由 + RadioGroup + resolveDispute 调用。价值：仲裁流程闭环。

#### G. `/disputes` 争议列表页（对应 §9 P1-6）
工作量：1 个新路由 + 表格。价值：Arbitrator 工作台。

#### H. Validator 详情页（对应 §9 P2-3）
工作量：1 个新路由 + 复用 ValidatorLeaderboard 模式。价值：Validator 画像完整。

### 11.3 低价值高成本（V2 考虑）

#### I. WebSocket 实时推送
当前用 SWR 轮询（4-6 秒间隔），实时性已足够。WebSocket 会增加后端复杂度，V1 不建议。

#### J. 通知中心
需要后端订阅 + 推送服务，V1 不建议。

#### K. 全局搜索
需要后端 ElasticSearch 或类似方案，V1 用页面内筛选即可。

#### L. API 文档页面
可用 Swagger UI 自动生成，但维护成本高。V1 建议直接读 `docs/SD.zh-CN.v1.0.md`。

---

## 附录：文档版本与维护

- **版本**：v1.0
- **撰写日期**：2026-07-18
- **撰写方式**：从代码现状反推，与 PRD/SD 互为印证
- **维护原则**：每次代码改动涉及页面/功能增减时，同步更新本文档 §3 页面总览 + §9 缺失项分析
- **关联文档**：
  - [PRD.zh-CN.v1.0.md](file:///home/administrator/Documents/trae_projects/PrismSettle/docs/PRD.zh-CN.v1.0.md) — 产品需求
  - [SD.zh-CN.v1.0.md](file:///home/administrator/Documents/trae_projects/PrismSettle/docs/SD.zh-CN.v1.0.md) — 系统设计
  - [DEV-PLAN.zh-CN.v1.0.md](file:///home/administrator/Documents/trae_projects/PrismSettle/docs/DEV-PLAN.zh-CN.v1.0.md) — 开发计划
  - [AGENTS.md](file:///home/administrator/Documents/trae_projects/PrismSettle/AGENTS.md) — AI 工作规则
  - [CONTEXT.md](file:///home/administrator/Documents/trae_projects/PrismSettle/CONTEXT.md) — 领域词汇表
