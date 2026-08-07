# PrismSettle — 产品需求文档 (PRD 1.0)

| 文档信息 | |
|---|---|
| 产品名称 | PrismSettle |
| 文档版本 | 1.0 |
| 状态 | Draft |
| 作者 | PrismSettle 团队 |

---

## 1. 执行摘要

### 1.1 产品概述

PrismSettle 是 **Monad 原生的 A2A（Agent-to-Agent）商业协议**，由三层组成：

1. **Trust Layer（信任层）** — `PrismSettleRegistry`：借鉴 ERC-8004 三段式接口（Identity / Reputation / Validation），采用 256 分片存储声誉记录，**消除 Monad OCC 并发写冲突**，中止率经 FR-T03/T04 压测验证显著降低（详见性能基准测试模块）。
2. **Commerce Layer（商业层）** — `PrismSettleJob`：实现 **ERC-8183 Agentic Commerce Protocol 核心 4 态状态机**（Open / Funded / Submitted / Terminal）+ 可选 `ArbitrationHook` 仲裁扩展，256 分片存储 Job 状态；通过 `fundViaToken` 统一入口接入 **x402 微支付协议**或标准 ERC-20，让 A2A 资金托管 + 任务交付在并行 EVM 上不阻塞。
3. **Indexer + Marketplace 层**：reorg-safe 事件同步 + A2A 调用市场。Agent / Agent 运营方可在 Marketplace 完成 **浏览 → 信任预检（三态决策）→ 创建 Job → 资金托管（ERC-20 / x402 统一入口）→ Agent 提交交付物 → Evaluator 自动校验（规则 + 评估 Agent 语义）→ 自动放款 → 自动触发声誉更新** 全流程。

**核心闭环**：

> Agent 在 Marketplace 上被发现 → BUYER 调 `checkTrust` 信任预检（三态决策）→ 创建 ERC-8183 Job（核心 4 态 + 可选仲裁 Hook）并 `fundViaToken` 锁定资金 → Agent 提交交付物 → Evaluator 规则校验 + 调评估 Agent 语义评分通过后自动放款 → 同步触发 `submitValidation` 写入 256 分片，Keeper bot 周期 `aggregateEpoch` 后声誉历史曲线实时更新；底层 256 分片保证大规模 A2A 调用场景下资金托管 + 声誉写入合约不阻塞。

### 1.2 核心价值主张

PrismSettle 为 Agent 生态提供两条核心价值：

- **A2A 商业闭环**：Agent 在 Marketplace 上被发现、被 `checkTrust` 三态预检、被其他 Agent 创建 ERC-8183 Job 调用，`fundViaToken` 统一资金入口（x402 / ERC-20）+ 任务交付 + 自动放款全流程在协议内闭环。
- **并行 EVM 适配**：通过 256 分片消除并行写冲突，Monad Agent 并发中止率经 FR-T03/T04 压测验证显著降低；ERC-8183 Job 状态同样采用 256 分片存储，资金托管合约在高并发 A2A 调用下不阻塞。

### 1.3 目标用户

| 角色 | 描述 | 核心诉求 |
|---|---|---|
| Agent 调用方 | 需要在链上调用 AI Agent 完成任务（数据分析、翻译、标注等）的终端用户 | 一站式浏览 + 声誉查询 + 一键调用，全流程不离开站点 |
| Agent 运营方 | 运行大规模 AI Agent 在 Monad 上并发交易的 B 端生态参与者 | 低中止率（< 10%）、Agent 被发现、被一键调用、收益闭环 |
| Validator | 持有 MON 代币，希望赚取验证收益的生态参与者 | 质押 → 验证 → 收益闭环 |

### 1.4 产品定位

**为什么是 For Agents 而不是 For Users**：

- PrismSettle 本质是 A2A（Agent-to-Agent）信任市场，使用者是其他 Agent / Agent 运营方，不是 C 端终端用户
- A2A 是 2026 叙事热点（OpenAI A2A 协议、Google A2A 框架），差异化定位更强
- 完整定位：**Monad Native 的 A2A 商业协议** — Trust Layer 借鉴 ERC-8004 实现 Agent 身份与声誉（256 分片）；Commerce Layer 实现 ERC-8183 核心 4 态状态机 + 可选 `ArbitrationHook` 仲裁扩展 + `fundViaToken` 统一资金入口（x402 / ERC-20）+ 256 分片资金托管；Marketplace 让 Agent 调用方 `checkTrust` 预检 → 创建 Job → `fundViaToken` 锁定资金 → Agent 提交交付物 → Evaluator 规则校验 + 调评估 Agent 语义评分 → 自动放款 → 自动触发声誉更新（写入 256 分片 + Keeper bot 周期聚合）。

### 1.5 与外部协议的关系

PrismSettle 借鉴 ERC-8004 三段式接口（Identity / Reputation / Validation）并独立采用 256 分片存储设计；严格实现 ERC-8183 核心 4 态状态机（Open / Funded / Submitted / Terminal）并通过 ArbitrationHook 扩展争议解决；接入 x402 微支付协议作为 Job 资金来源；补齐 AEP 多级任务分发中的资金托管与交付验证环节。PrismSettle 不是上述任一标准的完整实现，而是在并行 EVM 场景下针对 Monad OCC 的原生适配方案。

**V1.0 实现边界**：

- **ERC-8183 状态机**：严格实现核心 4 态；ArbitrationHook 作为可选扩展合约，`createJob` 时 hook 参数可选（0 表示不挂载）；REQUIRE_VALIDATION 决策时前端强制要求挂载 ArbitrationHook；不实现完整 Hook 回调机制（仅在 `submit` 后触发 Hook 争议入口 `onSubmitted(jobId)`，供 BUYER 后续发起 dispute）；不实现完整接口集（多 Evaluator 投票、跨链结算、ZK 验证）
- **ERC-8183 接口集**：核心 6 事件（JobCreated / Funded / Assigned / Submitted / Completed / Refunded）+ Hook 2 事件（Disputed / DisputeResolved）；函数集 createJob(+hook) / fundViaToken / assign / submit / complete / claimRefund + Hook 内 dispute / resolveDispute。`fundViaToken(jobId, amount, x402Receipt)` 为统一资金入口，内部根据支付方式分流：x402 receipt 走 x402 settle 路径（amount 传 0，由 receipt 决定），无 receipt 走标准 ERC-20 transferFrom 路径（amount 显式传入）
- **存储布局**：Registry 与 Job 均采用 256 分片（`agentId & 0xFF` / `jobId & 0xFF`）
- **Evaluator**：链下 Go 脚本，规则校验前置门（deliverable 非空 / 来源匹配 / 可获取 / proof_hash 关联）+ 通过 A2A 协议调用评估 Agent 做语义质量评分；持 `COMMERCE_EVALUATOR_ROLE` 调 `complete`，持 `RESOLVER_ROLE` 调 Hook 内 `resolveDispute`，持 `REGISTRY_EVALUATOR_ROLE` 免质押调 `Registry.submitValidation`；`complete` 后自动调 `submitValidation` 触发声誉更新
- **评估 Agent**：评估 Agent 调用失败 / 超时时降级为默认分 0.6e18，仍写入声誉系统，不阻塞 complete
- **proof_hash**：上链存证仅做形式合规校验，非可信执行
- **x402 集成**：只接 settle 接口，使用 Monad 官方 facilitator（https://x402-facilitator.molandak.org）；testnet USDC（合约 `0x534b2f3A21130d7a60830c2Df862319e593943A3`）；不做完整 payment middleware、跨链 settle、订阅支付
- **多级任务分发**：通过 `parentJobId` 支持二级分发（Provider → Sub-provider）留接口，V1 仅实现 BUYER → Provider → Evaluator → 自动放款 / 仲裁 一级闭环

**关于 ERC-8183 Hook 设计**：ERC-8183 官方设计刻意将核心状态机最小化（Open/Funded/Submitted/Terminal 4 态），争议解决等扩展功能通过 Job 创建时附加的可选 Hook 合约实现，不在核心状态机内规定。PrismSettle 遵循此 Hook 扩展哲学。

**关于 AEP 定位**：PrismSettle 不是 AEP 的替代品，是 AEP 协议层的补齐方案 — 保留 AEP 的多级任务分发叙事（BUYER / Provider / Sub-provider），补齐资金托管 + 交付验证，新增并行 EVM 适配（256 分片让 AEP 在 Monad OCC 上不阻塞）。

---

## 2. 产品目标与范围

### 2.1 产品目标

| 目标 | 衡量方式 |
|---|---|
| 让大规模 Agent 经济在并行 EVM 上可落地 | 双合约 256 分片使 500 并发下中止率经压测验证（优化前基线 vs 优化后结果，详见 FR-T03/T04/T05 压测方法论） |
| 闭环 A2A 商业协议的资金托管 + 交付验证 | ERC-8183 Job 生命周期完整跑通，资金流转可信 |
| 将声誉系统升级为交易流程决策点 | checkTrust 三态决策在创建 Job 前被执行 |
| 对齐 Monad 生态官方叙事 | x402 微支付接入 ERC-8183 Job 托管，使用 Monad 官方 facilitator |

### 2.2 范围之内

- Trust Layer：ERC-8004 借鉴三段式接口 + 256 分片 + 质押加权聚合 + Seed Phase + unstake 7 天解锁期
- Commerce Layer：ERC-8183 核心 4 态 + ArbitrationHook 扩展 + 256 分片 + proof_hash 上链 + testnet USDC via `fundViaToken`
- ArbitrationHook：独立合约，封装 Disputed/DisputeResolved，createJob 时可选挂载
- x402 集成：`fundViaToken` 内置 x402 settle 路径 + Monad 官方 facilitator + testnet USDC（只接 settle 接口）
- checkTrust 信任闸门：前端规则化判断（声誉阈值三态决策）+ REQUIRE_VALIDATION 联动 ArbitrationHook
- Evaluator：规则校验前置门 + 调评估 Agent 做语义质量评分 + 仲裁裁决，链下 Go 脚本；`complete` 后自动调 `Registry.submitValidation` 触发声誉更新（参考 AEP 高频 append + 低频 `aggregateEpoch` 双路径机制）
- Indexer：reorg 检测 + 回滚 + 顺序提交 + 三合约（Registry + Job + Hook）同步
- Marketplace：Job 创建 + 状态追踪 + 资金流转可视化 + checkTrust 三态决策 + Sad Path + 性能标签
- Agent 调用协议：HTTP POST + proof_hash 上链存证（形式合规校验，非可信执行）
- 部署：Monad 测试网（anvil 本地开发兜底，USDC 不可用时回退 MockERC20）

### 2.3 范围之外

- 多链声誉聚合
- 链上 slashing 治理自动化
- ZK 验证证明
- Validator 发现协议
- 完整 Agent SDK（仅 mock 协议）
- 移动端 App
- WebSocket 实时推送
- ERC-8004 严格兼容
- ERC-8183 完整接口集（多 Evaluator 投票、跨链结算、ZK 验证、完整 Hook 回调机制）— V1 仅核心 4 态 + ArbitrationHook 最小实现
- 多评估 Agent 投票 / 评估 Agent 语义对齐裁决（V1 单一评估 Agent 评分，已落地）
- claimRewards 完整逻辑（reward 计算模型 V2 实现）：当前仅留接口 stub
- 真实 reward 来源经济模型（通胀发行 / 调用费抽成 / 罚没分配）V2 评估

---

## 3. 用户角色与场景

### 3.1 用户角色

**P1. Agent 调用方**（终端用户）
- 需要在链上调用 AI Agent 完成任务（数据分析、翻译、标注等）
- 痛点：不知道哪个 Agent 可信、调用流程割裂
- 期望：一站式浏览 + 声誉查询 + 一键调用

**P2. Agent 运营方**（B 端生态价值）
- 运行大规模 AI Agent 在 Monad 上并发交易
- 痛点：OCC 写写冲突导致 60% 交易 abort，吞吐崩塌，大规模 A2A 经济无法落地
- 期望：低中止率（< 10%）、Agent 被发现、被一键调用、收益闭环
- 价值贡献：PrismSettle 让大规模 Agent 部署从「不可行」变为「可落地」

**P3. Validator**
- 持 100+ MON，希望赚取验证收益
- 期望：质押 → 验证 → 收益闭环

### 3.2 用户故事

**Agent 调用与商业闭环**
- US-01：作为 Agent 调用方，我希望在 Marketplace 浏览 + 调用 Agent 全流程不离开站点
- US-02：作为 Agent 调用方，我希望调用 Agent 后能在站内看到返回结果
- US-05：作为 Agent 调用方，我希望 Agent 详情页有声誉历史曲线
- US-13：作为 Agent 调用方，我希望调用超时时能 [重试] 或 [切换同类 Agent]，而不是只看到一个报错
- US-14：作为 Agent 调用方，我希望 Agent 详情页能看到「Shard Slot」和「Expected Latency」，直观感知到底层性能优势

**Monad Native 性能**
- US-04：作为 Agent 运营方，我希望 500 并发下中止率 < 10%
- US-04b：作为大规模 Agent 运营方，我需要在 Monad 高并发环境下稳定部署数百 Agent，PrismSettle 将交易失败率从 60% 降至 5%，让大规模 A2A 经济可落地

**Validator 激励闭环**
- US-15：作为 Validator，我希望在解锁期结束后能 unstake 资金，而不是永久锁仓

**Indexer 可信性**
- US-11：作为 Agent 调用方，我希望 indexer 在 reorg 时不返回错误数据

### 3.3 核心用例

**UC-01: 一键调用 Agent**

*Happy Path*：
1. 用户访问 Marketplace，浏览 Agent 列表
2. 按 A/B/C/D 评级筛选，点击"DeFi 分析 Agent"
3. 查看详情页：声誉 85.00%、12 条 Validator 评价、历史曲线、Monad Native 性能标签（Shard Slot: #42 / Expected Latency: < 1s）
4. 在调用框输入"分析 ETH/USDC 套利机会"
5. 点击"调用 Agent"按钮
6. Marketplace 后端代理调用 `POST https://agent.example.com/invoke`（FR-M11，规避 CORS）
7. 10s 内返回结果，展示在页面

*Sad Path*（UC-01b）：
1. 步骤 6 超时或返回错误
2. 组件显示 [重试] + [切换同类 Agent] 两个按钮
3. 用户点击 [重试] → 重新调用；用户点击 [切换] → 跳转到同类 Agent 详情页
4. 失败事件写入 FR-M12「隐性差评」记录，详情页"近期调用失败 N 次"+1

**UC-02: Validator 提交验证**
1. Validator 在控制台质押 100 MON
2. 浏览待验证 Agent 列表
3. 选择 Agent，提交验证分数 0.85e18（前端展示 85.00%）
4. 合约写入 `shardValidations[agentId & 0xFF][agentId]`
5. 事件触发 Indexer，Dashboard 5s 内刷新

**UC-03: 聚合分数**
1. Keeper bot 每 1 分钟调用 `aggregateEpoch(agentId)`
2. 合约校验 EPOCH，遍历分片做质押加权平均
3. 写入 `aggregatedScore`，Marketplace 显示新分数

**UC-04: Reorg 回滚**
1. Indexer 检测 `block(N).parentHash != stored(N-1).hash`
2. 回滚：删除 `block_number >= N-1` 事件，重置计数器
3. 重新同步，Marketplace 显示 amber `↻ reorg` 标记

**UC-05: 创建 ERC-8183 Job + x402 资金托管**
1. BUYER 选中 Agent，调 `checkTrust(agentId, amount)` 信任预检
2. 决策为 ALLOW → 直接创建 Job；决策为 REQUIRE_VALIDATION → 提示挂载 ArbitrationHook 后创建 Job；决策为 DENY → 前端拦截
3. 调 `createJob(agentId, parentJobId, deadline, hook)` 创建 Job
4. BUYER 调 `fundViaToken(jobId, amount, x402Receipt)` 通过 x402 微支付锁定 testnet USDC 到合约
5. BUYER 调 `assign(jobId, provider)` 指定 Provider
6. Provider 调 `submit(jobId, deliverableHash, proofHash)` 提交交付物 + proof_hash 上链存证
7. Evaluator 监听 Submitted 事件，5s 内完成规则校验（前置门）+ 调评估 Agent 做语义质量评分，调用 `complete(jobId)` 自动放款给 Provider
8. Evaluator 同步调用 `Registry.submitValidation(agentId, evalScore, proofHash, jobId, source=1)`，写入 `shardValidations[agentId & 0xFF]`，为声誉历史曲线产生一条新数据点（source=1 标记为 Job 自动评分；evalScore 为评估 Agent 返回的评分）
9. Keeper bot 周期调用 `aggregateEpoch(agentId)`，新评分进入 `aggregatedScore`，Marketplace 详情页声誉历史曲线刷新

*资金入口说明*：`fundViaToken` 为统一资金入口，无 x402 receipt 时走标准 ERC-20 路径，详见 §4.2。

**UC-06: 仲裁流程**
1. BUYER 在 Submitted 状态下点击「发起仲裁」按钮，输入 reasonHash
2. 调用 `ArbitrationHook.dispute(jobId, reasonHash)`，Hook 内状态变 Disputed
3. Evaluator 监听 Disputed 事件，5s 内根据 reasonHash 与 deliverableHash 关联性裁决
4. Evaluator 调用 `ArbitrationHook.resolveDispute(jobId, ruling)`，ruling=1（BUYER 退款）或 ruling=2（Provider 放款）
5. Hook 内状态变 DisputeResolved，后续由 BUYER 触发 claimRefund 或 Evaluator 触发 complete
6. 仲裁裁决同样触发 `submitValidation(agentId, score, proofHash, jobId, source=2)`：ruling=2 时按评估 Agent 评分写入；ruling=1 时写入公式化惩罚分 `max(0.2e18, currentScore×30%)`（体现 BUYER 反证成立，Agent 交付质量差，高分高罚）；source=2 标记为仲裁触发
7. 仲裁裁决 ruling=1（BUYER 退款）时，BUYER 可立即触发 `claimRefund`，不受 `deadline` 限制（仲裁退款优先于超时退款）

### 3.4 冷启动策略（Seed Phase）

**问题背景**：Marketplace 列表页和 Validator 评分互为依赖（先有鸡还是先有蛋），若无 Agent 数据，列表页空白。

**解决方案**：合约部署时由 Owner 触发一次性的 Seed 流程：

| 步骤 | 动作 | 数据 |
|---|---|---|
| 1 | `registerAgent(1, "DeFi Analysis Agent")` | metadata 含端点 URL |
| 2 | `registerAgent(2, "Translation Agent")` | metadata 含端点 URL |
| 3 | `registerAgent(3, "Data Labeling Agent")` | metadata 含端点 URL |
| 4 | `registerAgent(4, "Evaluation Agent")` | metadata 含端点 URL；官方预置评估 Agent，内部封装 gpt-4o-mini；Evaluator 默认调用此 Agent 做语义评分 |
| 5 | Owner 预置初始声誉分 | 每个 Agent aggregatedScore = 0.7e18（中等，留调整空间） |
| 6 | Validator 进入后可调整分数 | 上调/下调由实际 `submitValidation` 触发 |

**定位说明**：
- Seed Phase 预置是**官方引导行为**，与 FR-M06 公开注册不冲突
- 公开注册是生态扩展的入口，Seed 是冷启动兜底
- 不在合约里写「Owner 专属 seed 函数」绕开权限，直接用 `registerAgent` + Owner 账户调用，保持合约简洁

---

## 4. 功能需求

### 4.1 智能合约 — PrismSettleRegistry

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-C01 | `registerAgent(agentId, metadata)` 注册 Agent | P0 | 注册后 owner 正确，metadata 可查 |
| FR-C02 | `stake()` Validator 锁定 ≥100 MON | P0 | 触发 `Staked` 事件 |
| FR-C03 | `submitValidation(agentId, score, proofHash, jobId, source)` 双通道访问：已质押 Validator 调用 `source=0`、`jobId=0`；持 `REGISTRY_EVALUATOR_ROLE` 的 Evaluator 免质押调用 `source∈{1,2}`（1=Job complete 触发 / 2=仲裁裁决触发）、`jobId` 为关联 Job ID。合约按角色强制校验：Validator 只能 source=0，Evaluator 只能 source∈{1,2} | P0 | 写入正确分片；未质押且非 Evaluator revert；source/jobId 字段正确入库 |
| FR-C04 | `aggregateEpoch(agentId)` 限速聚合 | P0 | EPOCH 内重复调用 revert |
| FR-C05 | `slash(validator, evidenceHash)` slash 质押 | P1 | slash 后质押清零 |
| FR-C06 | `getScore(agentId)` 读取聚合分 | P0 | 返回 `aggregatedScore` |
| FR-C07 | `getValidationCount(agentId)` 读取验证次数 | P0 | 256 分片求和 |
| FR-C08 | 所有状态变更触发事件 | P0 | 5 种事件全覆盖 |
| FR-C09 | **Seed Phase 冷启动**：合约部署时由 Owner 预置 4 个官方 Agent（DeFi / 翻译 / 标注 / 评估 Agent）+ 初始声誉分 0.7e18 | P0 | Marketplace 开箱即有数据；所有 Agent 端点均接入真实 LLM API，评估 Agent 端点可被 Evaluator 调用 |
| FR-C10 | `unstake(amount)` Validator 解锁质押，含 7 天解锁期 | P0 | 解锁期内提交验证 revert；7 天后资金可提取 |
| FR-C11 | `claimRewards()` 接口 stub（V1 仅预留接口，不实现 reward 计算逻辑） | P2 | 函数存在但 revert "V2 only"；V2 依托「平台交易手续费抽成 + 作恶罚没池」落地 Validator 激励模型 |
| FR-C12 | **ValidationRecord 含来源字段**：record 携带 `source` 标识（`Validator` / `Evaluator-Job` / `Evaluator-Arbitration`），便于 Marketplace 详情页区分声誉来源（人为评分 vs Job 自动评分） | P0 | 入库字段正确；API 查询返回 source |

**经济模型说明**（FR-C10/C11）：
- FR-C10 `unstake` 是合约功能硬需求：Validator 无法退出 = 无人质押 = 系统空转，**必须实现**
- FR-C11 `claimRewards` V1 仅预留接口 stub，不实现奖励计算逻辑：
  - **V1 阶段**：Validator 激励由「质押 + slash 威慑」保障系统可信运行（Validators 通过声誉机制获得 Market 上的关注度，间接获益）
  - **V2 阶段**：依托「平台交易手续费抽成（ERC-8183 Job 完成时抽 1%-2%）+ 作恶罚没池（slash 罚没资金流入 reward 池）」落地完整激励模型
- 原因：reward 来源未定（通胀？调用费抽成？），无明确经济模型前强行实现 = 过度设计

### 4.2 智能合约 — PrismSettleJob（ERC-8183 状态机 4 宏态→7 态 + ArbitrationHook 扩展 + x402 集成）

**核心状态机**（官方 4 宏态 → 7 具体态，宏态严格对齐 ERC-8183 官方语义）：
- 官方 4 宏态映射：`Open=Created`、`Funded=Funded`、`Submitted=Assigned→Submitted`、`Terminal=DisputeResolved→Completed/Refunded`
- 主路径：`Created → Funded → Assigned → Submitted → Completed`
- 退款路径：`Funded/Assigned/Submitted → Refunded`（含 `claimRefund` 与仲裁 ruling=1）
- 仲裁路径：仲裁发起与裁决逻辑在 ArbitrationHook（Hook 内部维护 `Disputed` 态、选任仲裁方、计算费率）；裁决后经 `notifyDisputeResolved` 回调将 Job 切换到 `DisputeResolved` 细化态（等待公告期），公告期后任何人可调 `executeArbitrationResult` 落到 `Completed`（ruling=2）或 `Refunded`（ruling=1）

**函数签名清单**（Solidity 0.8.24+）：

```solidity
// 核心状态机
function createJob(bytes32 agentId, uint256 parentJobId, uint64 deadline, address hook) external returns (uint256 jobId);
function fundViaToken(uint256 jobId, uint256 amount, bytes calldata x402Receipt) external;
function assign(uint256 jobId, address provider) external;
function submit(uint256 jobId, bytes32 deliverableHash, bytes32 proofHash) external;
function complete(uint256 jobId) external;
function claimRefund(uint256 jobId) external;
function getJobState(uint256 jobId) external view returns (JobState state, address buyer, address provider, uint256 amount, bytes32 deliverableHash, bytes32 proofHash, uint64 deadline, address hook);

// ArbitrationHook 合约内部接口（独立合约）
interface IArbitrationHook {
    function dispute(uint256 jobId, bytes32 reasonHash) external;       // 仅 BUYER
    function resolveDispute(uint256 jobId, uint8 ruling) external;      // 仅 RESOLVER_ROLE（Evaluator）
    function getHookState(uint256 jobId) external view returns (HookState state, bytes32 reasonHash, uint8 ruling);
}
```

**函数输入输出说明**：

| 函数 | 输入参数 | 调用方权限 | 返回值 / 副作用 | 状态变更 | 事件 |
|---|---|---|---|---|---|
| `createJob` | `agentId` (bytes32) / `parentJobId` (uint256，二级分发 hook，根任务填 0) / `deadline` (uint64) / `hook` (address，ArbitrationHook 合约地址，可为 0 不挂载) | 任意地址（BUYER） | returns `jobId` (uint256) | Created（初始） | `JobCreated(agentId, jobId, buyer, deadline, hook)` |
| `fundViaToken` | `jobId` (uint256) / `amount` (uint256，ERC-20 路径的转账金额，x402 路径传 0) / `x402Receipt` (bytes，可选，为空时走 ERC-20 路径) | BUYER | 有 receipt → 通过 Monad 官方 facilitator settle（facilitator 校验 receipt 后将 USDC 从 buyer 转入合约，amount 传 0）；无 receipt → 内部调 `transferFrom(buyer, address(this), amount)`（buyer 需提前 approve） | Created → Funded | `Funded(jobId, buyer, amount)` |
| `assign` | `jobId` (uint256) / `provider` (address) | BUYER 指定 或 Provider 自荐 | 写入 provider 字段 | Funded → Assigned | `Assigned(jobId, provider)` |
| `submit` | `jobId` (uint256) / `deliverableHash` (bytes32) / `proofHash` (bytes32，执行证明 hash，上链存证) | Provider | 写入 deliverableHash + proofHash；若挂载 hook，触发 Hook 回调 | Assigned → Submitted | `Submitted(jobId, deliverableHash, proofHash)` |
| `complete` | `jobId` (uint256) | Evaluator (持 `COMMERCE_EVALUATOR_ROLE`) | transfer(USDC, contract→provider, amount) | Submitted → Completed | `Completed(jobId, provider, amount)` |
| `claimRefund` | `jobId` (uint256) | BUYER | 普通退款需 `block.timestamp >= deadline`；若 Job 已裁决（Hook 内状态 DisputeResolved 且 ruling=1），BUYER 可立即退款，不受 deadline 限制 | Funded/Assigned/Submitted → Refunded | `Refunded(jobId, buyer, amount)` |
| `getJobState` | `jobId` (uint256) | view 任意 | returns `(state, buyer, provider, amount, deliverableHash, proofHash, deadline, hook)` | — | — |

**ArbitrationHook 接口**（独立合约，封装争议态）：

| 函数 | 输入参数 | 调用方权限 | 返回值 / 副作用 | 状态变更 | 事件 |
|---|---|---|---|---|---|
| `dispute` | `jobId` (uint256) / `reasonHash` (bytes32) | BUYER（仅挂载该 Hook 的 Job） | 写入 disputeReason | None → Disputed | `Disputed(jobId, reasonHash)` |
| `resolveDispute` | `jobId` (uint256) / `ruling` (uint8：1=BUYER 退款 / 2=Provider 放款) | Evaluator (持 RESOLVER_ROLE) | 写入 disputeRuling；通知 Job 合约 | Disputed → DisputeResolved | `DisputeResolved(jobId, ruling)` |
| `getHookState` | `jobId` (uint256) | view 任意 | returns `(state, reasonHash, ruling)` | — | — |

**事件清单**（核心 6 事件 + Hook 内 2 事件）：

```solidity
// 核心事件（Job 合约）
event JobCreated(bytes32 indexed agentId, uint256 indexed jobId, address buyer, uint64 deadline, address hook);
event Funded(uint256 indexed jobId, address buyer, uint256 amount);
event Assigned(uint256 indexed jobId, address provider);
event Submitted(uint256 indexed jobId, bytes32 deliverableHash, bytes32 proofHash);
event Completed(uint256 indexed jobId, address provider, uint256 amount);
event Refunded(uint256 indexed jobId, address buyer, uint256 amount);

// Hook 内事件（ArbitrationHook 合约）
event Disputed(uint256 indexed jobId, bytes32 reasonHash);
event DisputeResolved(uint256 indexed jobId, uint8 ruling);
```

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-J01 | `createJob(agentId, parentJobId, deadline, hook)` 创建 Job 并锁定调用方为 BUYER | P0 | 写入正确分片 `shardJobs[jobId & 0xFF][jobId]`；触发 `JobCreated` 事件（含 hook 字段） |
| FR-J02 | `fundViaToken(jobId, amount, x402Receipt)` 统一资金入口：有 receipt 走 x402 settle（testnet USDC，amount 传 0），无 receipt 走标准 ERC-20 transferFrom（fallback：anvil 本地用 MockERC20） | P0 | 余额校验通过；状态 `Created → Funded`；触发 `Funded` 事件 |
| FR-J03 | `assign(jobId, provider)` Provider 接单 | P0 | 状态 `Funded → Assigned`；触发 `Assigned` 事件 |
| FR-J04 | `submit(jobId, deliverableHash, proofHash)` Provider 提交交付物 hash + proof_hash 上链存证；若挂载 hook，触发 Hook 回调 | P0 | 状态 `Assigned → Submitted`；触发 `Submitted` 事件（含 proofHash 字段） |
| FR-J05 | `complete(jobId)` Evaluator 调用放款给 Provider（持 `COMMERCE_EVALUATOR_ROLE`） | P0 | 状态 `Submitted → Completed`；USDC 转给 Provider；触发 `Completed` 事件 |
| FR-J06 | `claimRefund(jobId)` BUYER 在 deadline 后未交付则退款 | P0 | 状态 `Funded/Assigned/Submitted → Refunded`；USDC 退给 BUYER；触发 `Refunded` 事件 |
| FR-J07 | `ArbitrationHook.dispute(jobId, reasonHash)` BUYER 提交争议理由 hash | P0 | Hook 内状态 `None → Disputed`；触发 `Disputed` 事件；仅 BUYER 可调用 |
| FR-J08 | `ArbitrationHook.resolveDispute(jobId, ruling)` Evaluator 裁决（1=BUYER 退款 / 2=Provider 放款） | P0 | Hook 内状态 `Disputed → DisputeResolved`；触发 `DisputeResolved` 事件；仅 Evaluator 可调用；ruling=0 revert（防误传） |
| FR-J09 | `getJobState(jobId)` 读取当前状态 + 8 字段（含 proofHash + hook） | P0 | 256 分片求和定位 |
| FR-J10 | **256 分片存储**：`shardJobs[jobId & 0xFF][jobId]` | P0 | 与 Registry 同架构，OCC 写写冲突消除 |
| FR-J11 | **ArbitrationHook 合约独立部署**：封装仲裁发起与裁决逻辑（Hook 内部 Disputed 态），裁决后回调 Job 进入 DisputeResolved 细化态 | P0 | Hook 合约部署成功；createJob 时可挂载；不挂载时 Job 不走仲裁路径 |

**支付 token 说明**：
- 统一入口 `fundViaToken` 内部按支付方式分流：
  - **x402 路径**：Buyer 通过 Monad 官方 facilitator 完成 x402 settle。Facilitator 校验 Buyer 的 x402 receipt，确认后从 Buyer 账户将 USDC 转入 Job 合约。资金流向：Buyer → Facilitator（校验） → Job 合约。
  - **ERC-20 路径**：Buyer 提前授权（approve）Job 合约 USDC 额度，`fundViaToken` 内部调 `transferFrom(buyer, address(this), amount)`。资金流向：Buyer → Job 合约。
- demo 兜底：anvil 本地测试网若 USDC 不可用，回退到 MockERC20（仅 anvil 本地）
- 后续主网版本直接对接原生 USDC + x402 facilitator，无安全与合约冗余问题

### 4.3 性能基准测试模块

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-T01 | 基线对照合约（单槽存储，无分片），用于对照压测 | P0 | 部署到 anvil |
| FR-T02 | 负载生成器：500 并发 `submitValidation` | P0 | 可控并发数 |
| FR-T03 | **优化前基线采集**：部署单槽存储合约，执行预设压测用例，完整记录命令行日志、链上交易哈希、执行录屏、控制台截图，产出优化前原始指标基线值 | P0 | 过程录屏 + 原始日志完整留存 |
| FR-T04 | **优化后采集**：环境重置，部署 256 分片合约，复用完全相同的压测脚本/并发参数/执行时长复测，同步采集同维度指标，产出优化后指标值 | P0 | 过程录屏 + 原始日志完整留存 |
| FR-T05 | **压测可视化页面**：Marketplace 内嵌"性能对比"页面，展示：① 前后指标对照表（优化前基线 / 优化后结果 / 差值 / 变化幅度 / 是否达标）；② 分组柱状图（每组两根柱子，标注「优化前」「优化后」及增减百分比）；③ 256 分片热力图动画（复用 FR-M08 组件，展示 500 并发写入时分片分散效果）；④ 总结结论文案（固定模板："同等测试环境、同等压测用例下，256 分片优化使交易冲突回滚率从 X% 降至 Y%"） | P0 | 页面首屏 < 2s；数据来自两轮实测输出；对照表和柱状图为静态渲染；热力图为实时动画 |
| FR-T06 | **压测前置约束**：① 统一环境（同一节点/RPC、服务器硬件、数据库配置、区块参数、Gas 费率）；② 统一压测工具&用例（固定脚本、并发量级、请求模板、执行轮次、样本总量，两次输入完全一致）；③ 前置数据清零（每次测试前重置合约/业务状态，清除历史脏数据） | P0 | 报告内显式声明三项约束 |

### 4.4 Go Indexer

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-I01 | 监听 Monad 测试网 PrismSettleRegistry 事件 | P0 | 启动后 5s 内同步 |
| FR-I02 | 解析 5 种 Registry 事件为 `ChainEvent` | P0 | 每种事件单测覆盖 |
| FR-I03 | reorg 检测：父哈希对比 | P0 | 模拟 3 区块 reorg 后 DB 状态正确 |
| FR-I04 | reorg 回滚：删除受影响事件 + 重置计数器 | P0 | `completedTasks` 回滚到祖先+1 |
| FR-I05 | 顺序提交：`nextExpected`/`completedTasks` | P0 | 重复提交不产生重复事件 |
| FR-I06 | LRU 区块头缓存（1000 条） | P1 | 命中率 > 80% |
| FR-I07 | 优雅停机 | P1 | SIGTERM 30s 内退出 |
| FR-I08 | 配置缺失必填项时 fail-fast | P0 | 启动报错 |

### 4.5 Job Parser 与 Indexer 集成

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-JI01 | Job Parser 解析核心 6 种 Job 事件 + Hook 2 种仲裁事件（共 8 种）为 `ChainEvent` | P0 | 每种事件单测覆盖 |
| FR-JI02 | Indexer 同时监听 Registry + Job + ArbitrationHook 三合约事件 | P0 | 启动后 5s 内同步三个合约 |
| FR-JI03 | 顺序提交保证 Registry + Job + Hook 事件不冲突 | P0 | `nextExpected` 计数器跨合约共享 |
| FR-JI04 | proofHash 字段从 `Submitted` 事件解析并入库 | P0 | API 可查询 job 详情时返回 proofHash |
| FR-JI05 | hook 字段从 `JobCreated` 事件解析并入库 | P0 | API 可查询 job 详情时返回 hook 地址 |
| FR-JI06 | Hook 内 Disputed/DisputeResolved 事件解析并入库 | P0 | API 可查询 job 状态时返回 Hook 内仲裁态 |

### 4.6 Evaluator 链下脚本

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-E01 | 监听 `Submitted` 事件，5s 内决策 | P0 | noop 状态不阻塞后续 |
| FR-E02 | 形式校验：deliverable 非空 | P0 | 空字段 → reject |
| FR-E03 | 形式校验：来源匹配（提交者 == 当前 Provider） | P0 | 不匹配 → reject |
| FR-E04 | 形式校验：可获取（HTTP HEAD 检查 IPFS 网关或 URL 可达） | P0 | 不可达 → 中断决策流程，返回错误提示"交付物暂不可获取，请等待服务恢复"；不调用合约，无链上副作用 |
| FR-E05 | proof_hash 校验（仅形式层）：proofHash 非零 + IPFS 可访问。**V1 不做语义层强校验**（A2A 架构下 Evaluator 无法访问 Agent 内部状态，proof 仅作存证，详见 SD §4.5.2 能力边界声明） | P0 | proofHash 为空 → reject |
| FR-E06 | 决策调用：complete 通过 → 调用 `Job.complete(jobId)` | P0 | 链上状态正确变更 |
| FR-E07 | 决策调用：reject → 不调用合约（保留状态为 Submitted，等 deadline → claimRefund） | P0 | 无链上副作用 |
| FR-E08 | **仲裁监听**：监听 `Disputed` 事件，5s 内裁决 | P0 | noop 状态不阻塞 |
| FR-E09 | **仲裁裁决规则**：BUYER reasonHash 空 → 判 Provider 放款（ruling=2）；BUYER reasonHash 非空 → 任务硬性匹配校验：reasonHash 与 deliverableHash 完全相等 → BUYER 反证强 → 判 BUYER 退款（ruling=1）；否则 → BUYER 反证弱 → 判 Provider 放款（ruling=2） | P0 | 裁决调用 `resolveDispute(jobId, ruling)`，ruling ∈ {1, 2}，ruling=0 revert |
| FR-E10 | 决策日志：所有决策（含仲裁）写入结构化日志 | P0 | 日志可查 |
| FR-E11 | **评估 Agent 语义评分**：Evaluator 通过 A2A 协议（HTTP POST `/invoke`）调用专门的评估 Agent（Evaluation Agent）对 deliverable 做语义质量评分（0..1e18 fixed-point），规则校验通过后执行；评估 Agent 调用超时（>5s）或异常时降级为默认分 0.6e18 | P0 | 评估 Agent 评分写入决策日志；降级路径可观测；不阻塞 complete 调用 |
| FR-E12 | **自动触发声誉更新**：`complete(jobId)` 调用成功后，Evaluator 自动调用 `Registry.submitValidation(agentId, evalScore, proofHash, jobId, source=1)`，写入 `shardValidations[agentId & 0xFF]`（evalScore 为评估 Agent 返回的评分） | P0 | 链上 `ValidationSubmitted` 事件触发；shardValidations 数组 +1；source=1 入库；Indexer 5s 内可见 |
| FR-E13 | **仲裁后触发声誉更新**：`resolveDispute(jobId, ruling)` 后，Evaluator 调用 `Registry.submitValidation`：ruling=2 按评估 Agent 评分写入（Provider 胜诉但仍记录质量分）；ruling=1 写入公式化惩罚分 `max(0.2e18, currentScore×30%)`（体现 BUYER 反证成立，Agent 交付质量差，高分高罚）；`source=2`（Evaluator-Arbitration，与主路径 `complete` 后的 source=1 区分）；`jobId` 关联当前仲裁 Job | P0 | 仲裁裁决链上写入触发 ValidationSubmitted 事件；shardValidations 数组 +1；source=2 入库 |
| FR-E14 | **评估 Agent 接口规范**：评估 Agent 注册在 Marketplace，遵循 A2A 调用协议（FR-AP01 `/invoke`），输入为 `deliverableHash` 解析后的交付物内容 + `Job` 元数据；输出 JSON `{score: uint96, reason: string}`；评估 Agent 内部封装的 LLM 模型对 Evaluator 透明（可换） | P1 | 同一交付物多次评分结果稳定（方差 < 0.05）；评估 Agent 可在 Marketplace 列表页被发现 |

**Evaluator 实现约束**：
- 单一 Go 二进制，复用 Indexer 的 RPC client 和 event listener
- V1 **接入评估 Agent 做语义校验**：规则校验（FR-E02~E05）为前置门，必须全部通过；评估 Agent 评分（FR-E11）作为质量维度，写入声誉系统；评估 Agent 调用失败/超时时降级为默认分 0.6e18（保证 complete 不被阻塞）
- 与 Validator 松耦合：Evaluator 通过 `REGISTRY_EVALUATOR_ROLE` 免质押调用 `submitValidation`，不占用 Validator 的 stake 通道；`aggregateEpoch` 聚合时为**质押加权平均**：Validator 评分按其质押量加权（≥100 MON）；Evaluator 评分无质押，聚合时若某条 validation 记录的 `validatorStake` 为 0（即 Evaluator 评分），合约内 `if (stake == 0) stake = 1` 做兜底，确保 Evaluator 评分不会被静默丢弃（权重视为 1）；声誉为「Validator 主导 + Evaluator 补充」的混合模型，V2 可引入 source 维度的差异化权重
- 仲裁裁决权限：Evaluator 持有 `RESOLVER_ROLE`，仅能调用 `resolveDispute`；持有 `COMMERCE_EVALUATOR_ROLE`，可调 `Job.complete`；持有 `REGISTRY_EVALUATOR_ROLE`，可调 `Registry.submitValidation`
- 评估 Agent 配置：V1 默认接入官方预置的评估 Agent（内部封装 gpt-4o-mini，对 Evaluator 透明）；评估 Agent 端点通过环境变量配置，可切换为第三方评估 Agent 或本地模型 Agent；评估 Agent 本身也是 Marketplace 上的可注册 Agent，体现「Agent 评估 Agent」的递归生态
- 幂等性：同一 `jobId` 的 `submitValidation` 仅触发一次（Evaluator 内部维护 `processedJobs` 集合，重复事件跳过）

### 4.7 Agent 调用协议

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-AP01 | Agent 端点规范：`POST /invoke` 接收 JSON `{input: string, caller: address}` | P0 | 文档化 |
| FR-AP02 | Agent 响应规范：返回 `{output: string, proof_hash: bytes32}` | P0 | 文档化 |
| FR-AP03 | Marketplace 调用层：HTTP POST + 超时 10s + 错误处理 | P0 | 失败时友好提示 |
| FR-AP04 | Agent 注册时声明端点 URL | P0 | 存入 Agent metadata |
| FR-AP05 | 至少 4 个 Agent 实现（DeFi 分析 / 数据标注 / 翻译 / 评估 Agent） | P0 | 可被调用；所有 Agent 均接入真实 LLM API；评估 Agent 可被 Evaluator 调用 |

**协议设计**：

```http
POST /invoke HTTP/1.1
Content-Type: application/json

{
  "input": "分析 ETH/USDC 套利机会",
  "caller": "0x742d35Cc..."
}

Response:
{
  "output": "发现 3 个套利机会，预计收益 0.3%...",
  "proof_hash": "0xabc123..."
}
```

**proof_hash 上链存证说明**：
- proof_hash 由 Agent 生成后，**通过 `Job.submit(jobId, deliverableHash, proofHash)` 写入链上**，作为交付物形式合规校验依据
- 仍**非可信执行**：proof_hash 仅做形式合规（非空、与 deliverableHash 关联），不做 Validator 二次校验、不做 ZK 验证
- **设计预留 hook**：proof_hash 上链为未来 Validator 二次校验链路提供数据基础

### 4.8 x402 支付流

BUYER 通过 x402 微支付协议往 ERC-8183 Job 充值，资金来源为 testnet USDC，使用 Monad 官方 facilitator。

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-AP06 | x402 收据格式：`{receipt: bytes, amount: uint256, currency: address}` | P0 | 文档化（参考 x402 规范） |
| FR-AP07 | Marketplace x402 入口：BUYER 创建 Job 后调 `fundViaToken(jobId, amount, x402Receipt)` | P0 | testnet USDC 入金成功，Job 余额正确 |
| FR-AP08 | x402 facilitator 接入：使用 Monad 官方 facilitator，不自建 | P0 | 通过 facilitator 完成 settle |
| FR-AP09 | x402 失败兜底：facilitator 不可用时回退到 `fundViaToken(jobId, amount, "")` 直接转 USDC | P1 | 兜底成功，不中断 |

### 4.9 checkTrust 信任闸门接口

BUYER 选中 Agent 后，前端调 `checkTrust` 接口，返回三态决策，将声誉系统从「详情页数字」升级为「交易流程决策点」。

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-AP10 | `checkTrust(agentId, amount)` 接口：返回 `{decision: enum, reason: string}` | P0 | 决策值 ∈ {ALLOW, DENY, REQUIRE_VALIDATION} |
| FR-AP11 | 决策规则（V1 数据库配置，默认阈值：声誉 >0.8 → ALLOW；<0.3 → DENY；中间 → REQUIRE_VALIDATION） | P0 | 三态阈值可从数据库读取并热更新 |
| FR-AP12 | REQUIRE_VALIDATION 提示：BUYER 可挂载 ArbitrationHook 创建 Job | P0 | UI 提示「该 Agent 声誉中等，建议挂载仲裁 Hook」 |
| FR-AP13 | DENY 拦截：BUYER 无法创建 Job（前端拦截，非合约强制） | P1 | UI 拦截 + 引导选其他 Agent |

**接口签名**（伪代码）：

```solidity
// checkTrust 接口（V1 数据库配置阈值，Agent 运营方可自定义）
interface ITrustGate {
    enum Decision { ALLOW, DENY, REQUIRE_VALIDATION }
    struct TrustResult {
        Decision decision;
        string reason;       // "reputation=85.00% > 80%" / "reputation=15.00% < 30%" / "reputation=50.00%, suggest ArbitrationHook"
        uint256 reputation;  // 0-1e18 归一化（与 aggregatedScore 同单位），前端展示时转为百分比（如 85.00%）
    }
    function checkTrust(bytes32 agentId, uint256 amount) external view returns (TrustResult memory);
}
```

**阈值配置说明**：
- 默认阈值：ALLOW > 80%，DENY < 30%，中间 REQUIRE_VALIDATION
- 合约内部存储的比较值为 0-1e18 格式（ALLOW > 0.8e18，DENY < 0.3e18）
- V1 阈值存储在数据库中，由 Agent 运营方（或平台管理员）配置
- 前端调 `checkTrust` 时先从数据库读取该 Agent 的阈值配置，未配置时使用默认值
- 前端展示时，所有 reputation 值统一转为百分比（如 `85.00%`），保留两位小数

### 4.10 REST API

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-A01 | `GET /api/v1/prismsettle/events` 分页事件 | P0 | p99 < 200ms |
| FR-A02 | `GET /api/v1/prismsettle/score` 获取最新分数 | P0 | 返回 score + block 信息 |
| FR-A03 | `GET /api/v1/prismsettle/validations/count` 验证次数 | P0 | 与链上一致 |
| FR-A04 | `GET /api/v1/prismsettle/shards/activity` 256 分片活动 | P1 | 长度 256 数组 |
| FR-A05 | `GET /health` 健康检查 | P1 | JSON 格式 |
| FR-A06 | `GET /api/v1/prismsettle/agents` Agent 列表（含 metadata + 端点 URL） | P0 | Marketplace 消费 |
| FR-A07 | `GET /api/v1/prismsettle/agents/{id}` Agent 详情 | P0 | Marketplace 消费 |
| FR-A08 | `GET /api/v1/prismsettle/jobs` Job 列表（按状态过滤） | P0 | Marketplace 消费 |
| FR-A09 | `GET /api/v1/prismsettle/jobs/{id}` Job 详情（含状态 + 交付物 hash） | P0 | Marketplace 消费 |
| FR-A10 | `GET /api/v1/prismsettle/jobs/{id}/status` Job 状态机查询 | P0 | 返回核心 4 态之一（Created/Funded/Submitted/Terminal）；若挂载 Hook，附带 Hook 内 Disputed/DisputeResolved 状态 |
| FR-A11 | `GET /api/v1/prismsettle/trust?agentId=...&amount=...` checkTrust 信任闸门查询 | P0 | 返回 `{decision: ALLOW/DENY/REQUIRE_VALIDATION, reason, reputation}` |
| FR-A12 | `GET /api/v1/prismsettle/reputation/history?agentId=...` 声誉历史曲线（按时间倒序返回最近 30 条 ValidationRecord，含 source / score / timestamp / jobId 可选；支持分页滚动查询更多） | P0 | Marketplace 详情页声誉历史曲线组件直接消费；返回数组按 timestamp 降序，默认 30 条；含 source 字段区分人为评分 / Job 自动评分 / 仲裁评分 |

### 4.11 Marketplace 应用层

**信息架构（IA）**：顶部主导航拆分为三个 Tab，默认展示 Marketplace：

```
┌──────────────────────────────────────────────────────────────────┐
│ PrismSettle  [Marketplace]  [性能对比]  [Validator Console]     │
└──────────────────────────────────────────────────────────────────┘
     ↑ 默认对游客开放    ↑ 默认对游客开放    ↑ 需连钱包，默认隐藏
```

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-M01 | 首页：棱镜全息图 + 实时数据 + CTA + 核心价值主张 | P0 | 首屏 < 2s |
| FR-M02 | Agent 列表页：按声誉排序 + A/B/C/D 评级 + 能力标签筛选 | P0 | 支持分页和筛选 |
| FR-M03 | Agent 详情页：声誉历史曲线（消费 FR-A12 `/reputation/history`，曲线点按 source 着色：Validator=蓝 / Evaluator-Job=紫 / Evaluator-Arbitration=红）+ Validator 评价 + 一键调用入口 + Monad Native 性能标签（Shard Slot / Expected Latency） | P0 | 调用后展示结果；性能标签显性展示；声誉曲线至少 3 个数据点（含 Seed Phase 初始点） |
| FR-M04 | 一键调用组件：输入框 + 调用按钮 + 结果展示 + Sad Path 闭环（超时/报错时显示 [重试] + [切换同类 Agent] 按钮） | P0 | < 3s 返回结果；超时后可重试或切换 |
| FR-M05 | Validator 控制台（独立 Tab）：质押状态 + 提交验证表单 + 收益追踪（mock） | P1 | 可质押可验证；游客默认隐藏 |
| FR-M06 | Agent 注册入口：提交 AgentId + metadata + 端点 URL | P1 | 注册后可在列表显示 |
| FR-M07 | 实时事件流组件（5s 轮询，reorg 标记） | P0 | 新事件绿色 flash |
| FR-M08 | 分片热力图组件（16×16 脉冲） | P0 | 新事件脉冲 |
| FR-M08b | **性能对比页面**：Marketplace 新增"性能对比"Tab（与 Marketplace / Validator Console 并列），展示 256 分片效果：① 分组柱状图对比（每组两根柱子：优化前基线 vs 优化后结果，标注增减百分比）；② 关键指标卡片（并发数 500、中止率、平均延迟、区块时间）；③ 256 分片热力图动画（与 FR-M08 复用组件，展示 500 并发写入时的分片分散效果） | P0 | 页面首屏 < 2s；数据来自 FR-T03/T04 两轮实测输出；用户可直接点击该 Tab 查看 Monad Native 性能优势 |
| FR-M09 | 暗色模式默认，紫色品牌色 | P0 | Lighthouse > 90 |
| FR-M10 | 移动端基础适配 | P2 | 关键页面可读 |
| FR-M11 | **后端代理调用层**：Marketplace 后端代理前端 → Agent 端点的调用，彻底解决跨域 CORS 问题 | P0 | 前端不直接跨域请求 |
| FR-M12 | **调用失败事件记录**：失败事件关联到 Agent，作为 UI 侧「隐性差评」展示（不上链，纯前端聚合） | P2 | 详情页显示「近期调用失败 N 次」 |
| FR-M13 | **信任闸门组件**：BUYER 选中 Agent 后，前端调 `checkTrust(agentId, amount)`，返回三态决策（ALLOW / DENY / REQUIRE_VALIDATION）展示在 UI 上 | P0 | 三态决策正确显示；DENY 时拦截创建 Job；REQUIRE_VALIDATION 时提示挂载 ArbitrationHook |

**性能标签实现约束**（FR-M03）：
- `Shard Slot: #{agentId & 0xFF}` — 直接从 Agent ID 计算，无歧义
- `Expected Latency: < 1s` — 目标值，实际值由 Agent 实测填入
- 标签用 mono 字体 + 紫色高亮，让分片与 UI 的价值传导显性

### 4.12 Marketplace Job 闭环组件

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-JM01 | 「创建 Job」表单：选择 Agent + 输入金额 + deadline + 输入文本 | P0 | 提交后链上 createJob + fund |
| FR-JM02 | Job 状态追踪面板：**核心 4 态**（Open/Funded/Submitted/Terminal）主进度条 + **Hook 仲裁态**（Disputed/DisputeResolved）独立区块（仅当 Job 挂载了 ArbitrationHook 时显示）+ 当前状态高亮 + 历史事件时间线 | P0 | 核心 4 态始终可见；Hook 仲裁区块在无 hook 时不渲染，避免误导用户以为仲裁是核心流程一部分 |
| FR-JM03 | 「交付物提交」入口（Provider 视角）：mock Provider 自动 submit 触发 Evaluator | P0 | 5s 内 Evaluator 完成 → complete |
| FR-JM04 | 「资金流转」可视化：BUYER 锁定 → Provider 收到的金额流向图 | P1 | 状态变更时金额变化可视 |
| FR-JM05 | **「发起仲裁」按钮**（BUYER 视角）：Submitted 状态下点击 → 输入 reasonHash → 调用 `dispute` | P0 | 状态变 Disputed；面板显示仲裁中 |
| FR-JM06 | **「仲裁结果」展示**：DisputeResolved 状态后展示裁决方向（BUYER 退款 / Provider 放款） | P0 | 链上 DisputeResolved 事件触发更新 |

### 4.13 测试覆盖

| 编号 | 需求 | 优先级 | 验收标准 |
|---|---|---|---|
| FR-T07 | Registry 合约 Foundry 测试 | P0 | 21 个单测全通过 |
| FR-T08 | Registry Parser 单测（5 种事件） | P0 | 每种事件解码正确 |
| FR-T09 | Reorg 回滚单测 | P0 | 模拟 reorg 后 DB 状态断言 |
| FR-T10 | 顺序提交单测 | P0 | 重复提交计数器单调 |
| FR-T11 | 余额/分数计算单测 | P0 | 手算 == 输出 |
| FR-T12 | **Job 合约 Foundry 测试** | P0 | 核心 4 态全覆盖（含 Terminal 的 Completed/Refunded 两分支）+ ArbitrationHook 独立测试（Disputed/DisputeResolved）+ 资金流转 + 权限校验 + fundViaToken 收据校验 |
| FR-T13 | **Job Parser 单测** | P0 | 核心 6 事件 + Hook 2 事件（Disputed / DisputeResolved）解码正确 |
| FR-T14 | **Evaluator 决策单测** | P0 | 8 个场景：通过 / 空字段 / 来源不匹配 / 不可达 / proof_hash 不关联 / 评估 Agent 评分正常 / 评估 Agent 超时降级 / 仲裁裁决 |
| FR-T15 | **双合约 Indexer 集成测试** | P0 | Registry + Job 同时同步、无冲突 |
| FR-T16 | **Job 合约对照压测** | P0 | Job 合约单槽 vs 256 分片 abort 率对比，按 FR-T03/T04 方法论执行，产出分组柱状图 |
| FR-T17 | **仲裁流程集成测试** | P0 | Submitted → Disputed → DisputeResolved → Completed/Refunded 全链路 |
| FR-T18 | **声誉更新链路集成测试** | P0 | complete → submitValidation → shardValidations +1 → aggregateEpoch → aggregatedScore 更新；source 字段正确区分 Job/Arbitration |
| FR-T19 | **评估 Agent 评分幂等性测试** | P0 | 同一 jobId 重复 Submitted 事件只触发一次 submitValidation（processedJobs 去重） |
| FR-T20 | **REGISTRY_EVALUATOR_ROLE 权限测试** | P0 | 非 Evaluator 地址调用 submitValidation revert；Evaluator 持 REGISTRY_EVALUATOR_ROLE 免质押调用成功 |

---

## 5. 非功能需求

### 5.1 性能

| 编号 | 需求 | 目标 |
|---|---|---|
| NFR-MN01 | 500 并发下合约中止率 | 目标：经 FR-T03/T04 压测验证后，优化后中止率显著低于优化前基线（具体数值以压测可视化页面展示为准） |
| NFR-UX01 | API p99 延迟 | < 200ms |
| NFR-UX02 | Marketplace 首屏加载 | < 2s |
| NFR-UX03 | 一键调用响应时间 | < 3s（含 Agent 执行） |
| NFR-UX04 | 轮询频率 | 每 5s 一次，每页 < 5 请求 |
| NFR-UX05 | Lighthouse 评分 | > 90 |
| NFR-UX06 | 首页到一键调用路径点击数 | ≤ 3 次（设计约束） |
| NFR-EV01 | Evaluator 规则校验决策延迟 | < 5s（Submitted 事件到 complete 调用） |
| NFR-EV02 | Evaluator 评估 Agent 评分调用延迟 | < 5s（超时降级为默认分 0.6e18，不阻塞 complete） |
| NFR-EV03 | 声誉更新链路延迟 | complete 到 submitValidation 写入 < 2s；Keeper bot aggregateEpoch 周期默认 1 EPOCH（1 分钟） |
| NFR-EV04 | 声誉历史曲线 API 响应 | `/reputation/history` p99 < 200ms（默认返回最近 30 条，单 agent 单次查询 ≤ 30 条 ValidationRecord） |

**关于 NFR-UX06 的说明**：这是 UX 设计约束，不是产品 KPI。实现 3 次以内点击直达核心功能（首页 → Agent 列表 → 详情/创建 Job），降低 A2A 新手用户操作成本，规避复杂操作引发的 Demo 失败风险。

### 5.2 Monad Native

| 编号 | 需求 | 目标 |
|---|---|---|
| NFR-MN02 | 合约 EVM 版本 | Cancun（Monad 兼容） |
| NFR-MN03 | 合约部署链 | Monad 测试网 |

**串行 EVM 上分片无效的说明**：

| 链类型 | 执行模型 | 同 storage slot 并发写 | 分片收益 |
|---|---|---|---|
| 以太坊主网 | 串行 | 顺序执行，无冲突 | 无 |
| Arbitrum / Optimism | 串行 | 顺序执行，无冲突 | 无 |
| Monad | 并行 OCC | 直接 abort | **显著** |

PrismSettle 是 OCC 链专属基建，剥离 Monad 后设计失效。

### 5.3 正确性

| 编号 | 需求 | 验证方式 |
|---|---|---|
| NFR-C01 | reorg 下无静默数据损坏 | 单测模拟 3 区块 reorg |
| NFR-C02 | 重试下无重复事件 | 顺序提交计数器单调递增 |
| NFR-C03 | 地址以原始 EIP-55 形式存储 | 查询测试：大小写不敏感匹配 |
| NFR-C04 | 分数计算与合约一致 | 手算预期 == `aggregateEpoch` 输出 |
| NFR-C05 | Agent 调用超时不阻塞 UI | 10s 超时 + 用户可取消 |

### 5.4 可维护性

| 编号 | 需求 |
|---|---|
| NFR-M01 | 所有代码、注释、日志、commit message 使用英文 |
| NFR-M02 | 核心模块测试覆盖率 > 80% |
| NFR-M03 | 每个合约模块镜像 `prismsettle/` 布局（parser/service/api） |
| NFR-M04 | 不引入向后兼容 shim，直接改 |

### 5.5 可观测性

| 编号 | 需求 | 优先级 |
|---|---|---|
| NFR-OBS01 | 结构化日志（zap）含 chain/block/tx 上下文 | P0 |
| NFR-OBS02 | `/health` 端点暴露同步延迟、reorg 次数 | P1 |
| NFR-OBS03 | Prometheus 指标端点 | P2 |

### 5.6 安全

| 编号 | 需求 |
|---|---|
| NFR-S01 | Validator 必须质押 ≥100 MON 才能提交验证 |
| NFR-S02 | `aggregateEpoch` 同一 Agent 1 分钟内只能调用一次 |
| NFR-S03 | CORS 不允许 `*` + credentials 同时开启 |
| NFR-S04 | Agent 调用端点必须 HTTPS（生产环境） |
| NFR-S05 | Marketplace 调用 Agent 不传递用户私钥 |

---

## 6. 系统架构与技术设计

### 6.1 系统架构（三层 A2A 商业协议）

```
┌──────────────────────────────────────────────────────────────────────┐
│           Layer 3: Agent Marketplace（应用层 — A2A 调用市场）          │
│                                                                      │
│   首页（棱镜）→ Agent 列表 → 详情 → 创建 ERC-8183 Job → 资金托管     │
│   Job 状态追踪面板（核心 4 态 + Hook 仲裁态独立区块）+ Evaluator 自动放款反馈 │
│   Validator 控制台：质押 + 提交验证                                    │
└───────────────────────────────────┬──────────────────────────────────┘
                                    │ REST API + Agent 调用协议
                                    ▼
┌──────────────────────────────────────────────────────────────────────┐
│           Layer 2: Go Indexer + Evaluator（索引层 + 评估脚本）        │
│                                                                      │
│  EVMListener ──> Registry + Job + Hook Parser ──> EventIngestService  │
│   reorg 检测 + 回滚    5 + 6 + 2 种事件解析     顺序提交（跨三合约）   │
│   LRU 区块头缓存                                       DB 写入       │
│                                                                      │
│  Evaluator 脚本（链下 Go + 评估 Agent）：                              │
│   监听 Submitted → 规则校验前置门 → 调评估 Agent 评分 → complete        │
│   规则：deliverable 非空 / 来源匹配 / 可获取 / proof_hash 关联        │
│   评估 Agent：官方预置/ 第三方 / 本地，超时降级 0.6e18 │
│                                                                      │
│  声誉更新双路径（参考 AEP 高频 append + 低频 aggregate）：            │
│   高频：Evaluator.complete → Registry.submitValidation → shardValidations[256] │
│   低频：Keeper bot 周期 aggregateEpoch → aggregatedScore → Marketplace 曲线刷新 │
└───────────────────────────────────┬──────────────────────────────────┘
                                    │ 同步事件
                                    ▼
┌──────────────────────────────────────────────────────────────────────┐
│  Layer 1: Smart Contracts（合约层 — 三合约 256 分片）                 │
│                                                                      │
│  Trust Layer: PrismSettleRegistry                                    │
│   submitValidation() ──→ shardValidations[256]                       │
│   aggregateEpoch() ──────────┼──→ aggregatedScore                    │
│   stake() / slash() / unstake()                                       │
│   registerAgent() ────────────┼──→ agentMetadata                     │
│                                                                      │
│  Commerce Layer: PrismSettleJob (ERC-8183 4 宏态→7 态 + 256 分片)      │
|   createJob(+hook) ──→ shardJobs[256]                                 │
|   fundViaToken() / assign() / submit() / complete() / claimRefund() │
│   状态机（4 宏态→7 态）：Created(Open)→Funded→Assigned→Submitted→DisputeResolved→Completed/Refunded(Terminal) │
│   ArbitrationHook（可选挂载）：Disputed 在 Hook 内裁决；裁决后回调 Job 进入 DisputeResolved，公告期后执行 │
└───────────────────────────────────┬──────────────────────────────────┘
                                    │ Monad 测试网
                                    ▼
                            并行 EVM + OCC
```

### 6.2 关键设计决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 分片数 | 256 | 8 bits，500 并发下生日界限 P(冲突)~30% + 延迟聚合 → 压测验证 |
| 分片键 | `agentId & 0xFF` | 低 8 位，分布均匀 |
| 聚合限速 | EPOCH = 1 minute | 将"高并发写"和"聚合读"在时间维度上解耦，防止聚合函数成为新热点 |
| 聚合触发方 | Keeper bot 错峰调用 | 避免同一时刻所有 Agent 都聚合 |
| 地址存储 | 原始 EIP-55 | 保留校验和信息，查询用 `LOWER()` 归一化 |
| Reorg 检测 | 父哈希对比 | 行业标准做法，简单正确 |
| Reorg + 顺序提交联动 | 回滚时重置计数器 | 避免重组后重复入库或状态错乱（组合亮点） |
| 顺序提交 | `nextExpected`/`completedTasks` | exactly-once 入库 |
| Agent 调用协议 | 自定义 HTTP POST + proof_hash 上链 | 简单可演示；proof_hash 通过 Job.submit 写入链上做形式合规校验，仍非可信执行 |
| ERC-8004 关系 | 独立设计 + 借鉴接口 | 256 分片是核心创新，不被标准框死；定位为 8004 的并行 EVM 升级版 |
| **unstake 解锁期** | 7 天 | 防止作恶后立即跑路 |
| **claimRewards** | 接口 stub，reward 模型 V2 | 无明确经济模型前不强行实现，避免过度设计 |
| **Seed Phase** | 部署时 Owner 预置 4 个 Agent（3 业务 + 1 评估）+ 初始分 0.7 | 解决冷启动鸡生蛋问题，不绕开权限直接用 `registerAgent` |
| **信息架构** | Marketplace Tab + Validator Console Tab 分隔 | 游客默认看 Marketplace，Validator 需连钱包；降低认知负荷 |
| **性能标签** | Agent 详情页显性展示 Shard Slot / Expected Latency | 打通底层优势到 UI 感知的断点 |
| **Sad Path 闭环** | 超时显示 [重试] + [切换同类 Agent] | 报错时体验不至于崩塌 |
| **项目定位** | **For Agents**：A2A 信任市场，使用者是 Agent / Agent 运营方 | A2A 是 2026 叙事热点；避免 C 端应用混淆 |
| **ERC-8183 核心 4 态 + ArbitrationHook 重定位** | 核心 4 态严格对齐官方 + 仲裁逻辑封装到独立 ArbitrationHook 合约，createJob 时可选挂载 | ERC-8183 官方刻意把争议解决排除在核心之外，留给 Hook；Hook 机制遵循官方设计哲学 |
| **fundViaToken 统一入口** | `fundViaToken(jobId, amount, x402Receipt)` 内部按 receipt 是否为空分流：x402 settle（amount 传 0）/ ERC-20 transferFrom | 简化合约接口，统一资金入口，x402 和 ERC-20 共用 `Funded` 事件 |
| **checkTrust 信任闸门三态** | BUYER 选中 Agent 后前端调 checkTrust(agentId, amount)，从数据库读取该 Agent 的阈值配置（默认 ALLOW>0.8/DENY<0.3/中间 REQUIRE_VALIDATION），REQUIRE_VALIDATION 时提示挂载 ArbitrationHook | 声誉系统从「详情页数字」升级为「交易流程决策点」，与仲裁 Hook 联动形成完整闭环 |
| 前端框架 | Next.js + Tailwind + shadcn/ui | 生态成熟、开发效率高 |
| 前端动画 | CSS 3D + Framer Motion + GSAP | 比 three.js 轻、足够实现棱镜效果 |
| 前端数据刷新 | 5s 轮询 + 服务端 block_number 增量查询 | 工程权衡，未做 websocket（不假装区块事件订阅） |
| CORS 解决方案 | Marketplace 后端代理调用 Agent 端点 | 前端不直接跨域，彻底规避 CORS |
| 暗色品牌色 | 紫粉渐变 | 基础设施行业惯例 + 棱镜隐喻 |
| 文档驱动 | PRD + 5 Skills + 决策记录 | 协作可查、上手快 |

### 6.3 数据模型

**Registry 合约数据模型**：

```solidity
struct ValidationRecord {
    address validator;     // Validator 地址 或 Evaluator 地址（持 REGISTRY_EVALUATOR_ROLE）
    uint96 score;           // 0..1e18 fixed-point (1e18 = 1.0)
    bytes32 proofHash;      // 交付物 proof_hash
    uint64 timestamp;
    uint8 source;           // 0=Validator（人为评分）/ 1=Evaluator-Job（Job complete 触发）/ 2=Evaluator-Arbitration（仲裁裁决触发）
    uint256 jobId;          // source>0 时关联的 Job ID；Validator 评分时为 0
}

struct AgentMetadata {
    address owner;
    string endpointUrl;   // Agent 调用端点，如 https://agent.example.com
    string capabilities;  // JSON 字符串，能力标签
    uint64 registeredAt;
}

mapping(uint8 => mapping(uint256 => ValidationRecord[])) public shardValidations;  // shardValidations[agentId & 0xFF][agentId]
mapping(uint256 => uint256) public aggregatedScore;
mapping(uint256 => AgentMetadata) public agents;

bytes32 public constant REGISTRY_EVALUATOR_ROLE = keccak256("REGISTRY_EVALUATOR_ROLE"); // Evaluator 免质押调用 submitValidation

// aggregateEpoch(agentId)
// - 调用方：任何人（anyone），不限制权限。Keeper bot 是推荐的调用者（错峰调度），但合约层面不限制
// - 目的：V1 阶段聚合逻辑简单，anyone 可调不会造成滥用；V2 可改为仅限持牌 Keeper

// submitValidation(agentId, score, proofHash, jobId, source)
// - Validator (staked): source=0, jobId=0
// - Evaluator (REGISTRY_EVALUATOR_ROLE): source=1 (Job complete) or source=2 (Arbitration), jobId=actual Job ID
// - 合约按角色强制校验 source 取值合法
```

**Job 合约数据模型**：

```solidity
// 核心 4 宏态映射说明：
//   ERC-8183 官方 4 宏态 = Open / Funded / Submitted / Terminal
//   enum 落地映射（4 宏态 → 7 具体态）：
//     Open      = Created（Job 创建，未注资）
//     Funded    = Funded（资金到位）
//     Submitted = Assigned → Submitted（已分配 Provider + 提交交付物；Assigned 为分配子态）
//     Terminal  = DisputeResolved → Completed（放款）/ Refunded（退款）— 含仲裁结算细化态
//   注：7 个 enum 值是 4 宏态的展开：Terminal 双分支（Completed/Refunded）+ Assigned 子态
//       + DisputeResolved 仲裁结算细化态；仲裁发起与裁决逻辑仍在 ArbitrationHook 合约，
//       裁决后经 notifyDisputeResolved 回调进入 DisputeResolved，公告期后执行落终态
enum JobState { Created, Funded, Assigned, Submitted, DisputeResolved, Completed, Refunded }
struct Job {
    address buyer;          // 资金提供方
    address provider;       // 任务执行方
    uint256 amount;         // 锁定的 USDC 数量
    bytes32 deliverableHash;// 交付物 hash（IPFS hash 或内容 hash）
    bytes32 proofHash;      // proof_hash 上链存证（非可信执行，V2 hook）
    uint64 deadline;        // 超时退款时间
    JobState state;         // 见上方映射说明（4 宏态，enum 落地 7 值）
    uint256 parentJobId;    // 二级分发 hook（Provider → Sub-provider）
    address hook;           // ArbitrationHook 合约地址（0 表示不挂载）
    uint64 createdAt;
}

// 256 分片存储，与 Registry 同架构
mapping(uint8 => mapping(uint256 => Job)) public shardJobs;

IERC20 public paymentToken;  // testnet USDC；anvil 兜底用 MockERC20
bytes32 public constant COMMERCE_EVALUATOR_ROLE = keccak256("COMMERCE_EVALUATOR_ROLE"); // Evaluator 放款权限（调 Job.complete）
```

**ArbitrationHook 合约数据模型**：

```solidity
enum HookState { None, Disputed, DisputeResolved }
struct HookData {
    bytes32 disputeReason;  // BUYER 仲裁理由 hash
    uint8   disputeRuling;  // 0=未裁决 1=BUYER 2=Provider
    HookState state;
}

mapping(uint256 => HookData) public hookData;  // jobId → HookData
address public jobContract;  // 反向引用 Job 合约（权限校验）
bytes32 public constant RESOLVER_ROLE = keccak256("RESOLVER_ROLE"); // Evaluator 仲裁权限
```

**checkTrust 信任闸门数据模型**（前端规则化）：

```typescript
// 决策枚举
enum Decision { ALLOW, DENY, REQUIRE_VALIDATION }

// 决策结果
interface TrustResult {
  decision: Decision;
  reason: string;       // "reputation=85.00% > 80%" / "reputation=15.00% < 30%" / "reputation=50.00%, suggest ArbitrationHook"
  reputation: number;   // 0-1 归一化（aggregatedScore / 1e18），前端展示时转为百分比（如 85.00%）
}

// 阈值配置（V1 数据库配置，Agent 运营方可自定义；未配置时使用默认值 ALLOW>0.8/DENY<0.3）
const ALLOW_THRESHOLD = 0.8;
const DENY_THRESHOLD = 0.3;
```

**Agent 调用协议数据结构**：

```typescript
// 调用请求
interface InvokeRequest {
  input: string
  caller: string  // 调用方地址
}

// 调用响应
interface InvokeResponse {
  output: string
  proof_hash: string  // 0x 开头 hex
}
```

### 6.4 Evaluator 决策状态机

```
监听 Submitted 事件
        │
        ▼
   ┌─────────────┐
   | deliverable | ─── 空 ──→ reject ( noop )
   |   非空?     |
   └─────┬───────┘
         │ 是
         ▼
   ┌─────────────┐
   | 来源匹配?   | ─── 否 ──→ reject ( noop )
   | (provider)  |
   └─────┬───────┘
         │ 是
         ▼
   ┌─────────────┐
   | 可获取?     | ─── 否 ──→ reject ( noop )
   | (HTTP HEAD) |
   └─────┬───────┘
         │ 是
         ▼
   ┌─────────────┐
   | proof_hash  | ─── 不匹配 ──→ reject ( noop )
   | 关联校验?   |
   └─────┬───────┘
         │ 是
         ▼
   ┌─────────────────────┐
   | 调用评估 Agent 评分    │
   | 输入: deliverable + 元数据 |
   | 输出: score (0..1e18) │ ─── 超时/异常 ──→ score = 0.6e18（降级默认分）
   └─────┬───────────────┘
         │
         ▼
   complete(jobId)  ──→ 链上状态变更 Submitted → Completed（核心 4 态闭环，持 COMMERCE_EVALUATOR_ROLE）
         │
         ▼
    Registry.submitValidation(agentId, score, proofHash, jobId, source=1)  ──→ 持 REGISTRY_EVALUATOR_ROLE，source=1 (Evaluator-Job)
         │                                                     │
         │                                                     ▼
         │                                          shardValidations[agentId & 0xFF] append +1
         │                                                     │
         │                                                     ▼
         │                                          Keeper bot aggregateEpoch(agentId) [EPOCH 到期]
         │                                                     │
         │                                                     ▼
         │                                          aggregatedScore 更新 → Marketplace 声誉曲线刷新
         │
=== 仲裁分支（仅当 Job 挂载 ArbitrationHook 时触发）===

监听 Disputed 事件（来自 ArbitrationHook 合约）
        │
        ▼
   ┌─────────────────┐
   | BUYER reasonHash| ─── 空 ──→ resolveDispute(jobId, 2=Provider)
   |    非空?        |
   └─────┬───────────┘
         │ 是
         ▼
   ┌─────────────────┐
   | 任务硬性匹配校验：| ─── reasonHash == deliverableHash ──→ BUYER 反证强 ──→ resolveDispute(jobId, 1=BUYER)
   | reasonHash ==   |
   | deliverableHash?| ─── 不相等 ──→ BUYER 反证弱 ──→ resolveDispute(jobId, 2=Provider)
   └─────┬───────────┘
         ▼
   resolveDispute(jobId, 1=BUYER)  ──→ Hook 内状态变更 Disputed → DisputeResolved
         │
         ▼
   Registry.submitValidation(agentId, score, proofHash, jobId, source=2)
     - ruling=2 → score = 评估 Agent 评分（同主路径）
     - ruling=1 → score = max(0.2e18, currentScore×30%)（公式化惩罚分，高分高罚）
     - source = 2 (Evaluator-Arbitration)
         │
         ▼
   后续由 BUYER 触发 claimRefund 或 Evaluator 触发 complete
```

### 6.5 checkTrust 三态决策流程

```
BUYER 选中 Agent
        │
        ▼
   checkTrust(agentId, amount)
        │
        ▼
   ┌──────────────────┐
   | 读取 Agent 声誉   |
   | (从 Registry 查) |
   └─────┬────────────┘
         │
   ┌─────┴──────┬─────────────┐
   ▼            ▼             ▼
   rep > 80%   30% ≤ rep ≤ 80%  rep < 30%
   │            │             │
   ▼            ▼             ▼
 ALLOW    REQUIRE_VALIDATION   DENY
   │            │             │
   │            ▼             ▼
   │   提示 BUYER 挂载       前端拦截
   │   ArbitrationHook        创建 Job
   │            │
   │            ▼
   │   createJob(+hook)
   │            │
   ▼            ▼
   createJob（无 hook）   核心状态机 + Hook 仲裁扩展
```

---

## 7. 依赖与约束

### 7.1 外部依赖

| 依赖 | 用途 |
|---|---|
| Monad 测试网 | 合约部署、事件源 |
| go-ethereum | RPC 客户端 |
| Foundry / Anvil | 本地测试、压测 |
| Next.js / React 19 | 前端框架 |
| 官方预置 Agent 服务 | 一键调用演示（3 个业务 Agent + 1 个评估 Agent，均接入真实 LLM API） |
| Monad 官方 x402 facilitator (https://x402-facilitator.molandak.org) | x402 settle |
| Monad 测试网 Circle USDC 水龙头 (合约 `0x534b2f3A21130d7a60830c2Df862319e593943A3`) | testnet USDC 来源 |
| 评估 Agent（Evaluation Agent） | Evaluator 通过 A2A 协议调用的专门做交付物语义评分的 Agent；V1 默认接入官方预置 Agent（内部封装 OpenAI gpt-4o-mini）；可通过环境变量切换第三方评估 Agent 或本地模型 Agent |

### 7.2 假设

| 编号 | 假设 |
|---|---|
| A01 | 500 并发是 Agent 经济典型负载 |
| A02 | 256 分片对 500 并发足够 |
| A03 | 1 分钟 EPOCH 足够 keeper 错峰聚合 |
| A04 | 5s 轮询对产品体验足够流畅 |
| A05 | Agent 协议（HTTP POST）用于 A2A 调用，所有 Agent 接入真实 LLM API |
| A06 | 用户接受「独立设计 + 借鉴」而非严格兼容 ERC-8004 |

### 7.3 约束

| 编号 | 约束 |
|---|---|
| CO01 | 代码、注释、交互全英文 |
| CO02 | 不使用 three.js（太重） |
| CO03 | 不做 websocket（过度设计） |
| CO04 | 不做完整 Agent SDK，仅 mock 协议 |
| CO05 | 不做 ERC-8004 兼容，独立设计 |
| CO06 | 不实现 ERC-8183 完整接口集（仅核心 4 态 + ArbitrationHook 最小实现） |
| CO07 | V1 不做链上 slashing 自动化（Evaluator 自动写入声誉，依赖 source 字段区分来源 + Validator 人为评分对冲） |
| CO08 | reorg 场景通过 anvil `evm_reorg` 模拟验证 |

---

## 8. 里程碑与发布计划

### 8.1 V1.0 核心交付物

**Trust Layer — Registry**
- 256 分片声誉存储 + 5 种事件 + Seed Phase 冷启动
- `REGISTRY_EVALUATOR_ROLE` 通道：Evaluator 免质押调 `submitValidation`，ValidationRecord 含 `source` / `jobId` 字段
- 双路径声誉更新：高频 `submitValidation` append + 低频 Keeper bot `aggregateEpoch` 质押加权聚合
- Validator 质押 + unstake 7 天解锁期

**Commerce Layer — Job**
- ERC-8183 核心 4 态状态机（Open / Funded / Submitted / Terminal）
- ArbitrationHook 独立合约扩展（Disputed / DisputeResolved），createJob 时可选挂载
- 256 分片存储 + proof_hash 上链存证（形式合规校验）
- x402 微支付集成（testnet USDC，Monad 官方 facilitator）

**Evaluator 链下脚本**
- 规则校验前置门（deliverable 非空 / 来源匹配 / 可获取 / proof_hash 关联）
- 接入评估 Agent 做语义质量评分（A2A 协议 `/invoke`），超时降级默认分 0.6e18
- `complete` 后自动触发 `submitValidation` 写入声誉（source 区分 Job / Arbitration）
- 仲裁裁决调用 `ArbitrationHook.resolveDispute`

**Indexer**
- reorg 检测 + 回滚 + 顺序提交
- 三合约同步（Registry + Job + ArbitrationHook）

**Marketplace**
- Agent 列表 / 详情 + 声誉历史曲线（按 source 着色）
- Job 创建 + 状态追踪 + 资金流转可视化
- checkTrust 信任闸门三态决策（ALLOW / DENY / REQUIRE_VALIDATION）
- 一键调用 Agent（后端代理，规避 CORS）+ Sad Path 重试 / 切换
- 性能标签（Shard Slot / Expected Latency）

**测试与部署**
- Foundry 单测 + Go 单测 + 三合约集成测试
- 对照压测（按 FR-T03/T04 方法论：统一环境 + 统一用例 + 前置清零，两轮实测产出对照表 + 分组柱状图 + 录屏留证）+ 压测可视化页面（Marketplace 内嵌"性能对比"Tab）
- 部署到 Monad 测试网 + docker-compose 一键启动

### 8.2 后续演进方向

- **Validator 治理**：链上 slashing 自动化 + Validator 发现协议
- **ERC-8183 完整接口**：多 Evaluator 投票 / 跨链结算 / ZK 验证 + 完整 Hook 回调机制
- **x402 完整集成**：payment middleware + 跨链 settle + 订阅支付 + 原生 USDC
- **checkTrust 升级**：后端决策引擎 + 链上强制 + 多维信任评估（行为模式 + 历史交易）
- **评估 Agent 生态**：多评估 Agent 投票 + 跨语言交付物评估 + 评估 Agent 语义对齐裁决
- **Validator 激励**：平台交易手续费抽成 + 作恶罚没池 + source 维度差异化权重
- **Indexer 升级**：多链聚合 + WebSocket 实时推送
- **Marketplace 升级**：移动端适配 + 用户反馈上链 + 完整 Agent SDK
- **主网部署**：跨链迁移至任意并行 EVM

### 8.3 开发节奏

**核心策略**：Phase 1-3 全部基于 anvil 本地开发，不依赖外部网络（faucet / RPC）；Phase 4 集中上线 + 端到端验证。  

| 阶段 | 目标 | 主要交付 |
|---|---|---|
| Phase 1 | 合约层 + x402 PoC + 评估 Agent PoC（anvil 本地） | Registry（含 REGISTRY_EVALUATOR_ROLE + source/jobId 字段）+ Job + ArbitrationHook 合约 + Foundry 测试 + x402 PoC 可用性验证 + 评估 Agent PoC（官方预置 Agent 封装 gpt-4o-mini，A2A `/invoke` 调通 + 降级路径） |
| Phase 2 | Indexer + Evaluator + 声誉更新链路（anvil 本地） | Job Parser + Hook Parser + 三合约同步 + API 端点（含 `/reputation/history`）+ Evaluator 链下脚本（规则门 + 调评估 Agent + complete → submitValidation 触发）+ 4 个 Agent 实现（3 业务 + 1 评估 Agent，均接入真实 LLM API）+ Keeper bot aggregateEpoch 验证 |
| Phase 3 | Marketplace 前端（连 anvil） | Next.js 项目 + Agent 列表/详情 + 创建 Job 表单 + 状态追踪面板 + checkTrust 信任闸门组件 + 声誉历史曲线组件（按 source 着色）+ 一键调用 + 性能对比页面（柱状图 + 热力图动画 + 指标卡片）+ docker-compose |
| Phase 4 | 部署 + 收尾 | 部署三合约 + USDC 到 Monad 测试网 + Seed Phase + Indexer 切到 Monad RPC + 性能对比页面数据联调（导入两轮实测数据）+ 压测录屏归档 + README + demo 视频 + Buffer |

**降级策略**：x402 PoC 和评估 Agent PoC 提前到 Phase 1 Day 1 做可用性验证。x402 不可用 → 降级为「纯 ERC-20 fund + x402 叙事保留在 PRD」；评估 Agent 不可用 → 降级为「纯规则校验 + 默认分 0.6e18」（FR-E11 降级路径已设计）。

---

## 9. 术语表

| 术语 | 含义 |
|---|---|
| A2A | Agent-to-Agent，Agent 之间的协议/交互 |
| Agent | 在链上提供服务的 AI 智能体（数据分析、翻译、标注等） |
| Agent 调用方 | 在 Marketplace 上调用 Agent 完成任务的终端用户 |
| Agent 运营方 | 运行大规模 AI Agent 在 Monad 上并发交易的 B 端生态参与者 |
| ArbitrationHook | 独立仲裁合约，封装 Disputed/DisputeResolved 状态，createJob 时可选挂载 |
| BUYER | 在 ERC-8183 Job 中提供资金的一方 |
| checkTrust | 信任闸门接口，BUYER 创建 Job 前查询，返回 ALLOW/DENY/REQUIRE_VALIDATION 三态决策 |
| EPOCH | 限速聚合窗口，PrismSettle 设为 1 分钟 |
| Evaluator | 链下 Go 脚本，监听 Submitted/Disputed 事件做规则校验前置门 + 调用评估 Agent 做语义质量评分 + 仲裁裁决；持有 `REGISTRY_EVALUATOR_ROLE`（免质押调 `submitValidation`）和 `COMMERCE_EVALUATOR_ROLE`（调 `Job.complete`），以及 `RESOLVER_ROLE`（调 `resolveDispute`） |
| EVALUATOR_ROLE | Registry 合约的 `REGISTRY_EVALUATOR_ROLE`，允许 Evaluator 免质押调用 `submitValidation`；Job 合约的 `COMMERCE_EVALUATOR_ROLE`，允许 Evaluator 调用 `complete`。两合约角色名不同，避免实现混淆 |
| Evaluation Agent（评估 Agent） | 专门做交付物语义质量评分的 Agent，注册在 Marketplace 上；Evaluator 通过 A2A 协议（`/invoke`）调用它获取 0..1e18 评分；内部封装 LLM（默认 gpt-4o-mini）但对 Evaluator 透明；评估 Agent 本身也有声誉，形成「Agent 评估 Agent」的递归信任 |
| EVM | Ethereum Virtual Machine |
| EIP-55 | 以太坊地址大小写校验和方案 |
| facilitator | x402 协议中的结算服务方，PrismSettle 使用 Monad 官方 facilitator |
| Foundry | Solidity 开发框架（含 forge / anvil / cast） |
| Indexer | 链下事件同步服务，监听合约事件并入库，提供 REST API |
| Keeper bot | 错峰调用 `aggregateEpoch` 的链下机器人 |
| LRU | Least Recently Used，最近最少使用缓存策略 |
| MockERC20 | anvil 本地测试用的 ERC-20 兜底 token |
| Monad | 并行 EVM 区块链，采用 OCC 模型 |
| OCC | Optimistic Concurrency Control，乐观并发控制 |
| proof_hash | Agent 执行证明 hash，通过 Job.submit 写入链上做形式合规校验 |
| Provider | 在 ERC-8183 Job 中接收任务并提交交付物的一方 |
| reorg | 区块链重组，可能造成已索引事件回滚 |
| Seed Phase | 合约部署时由 Owner 预置 4 个官方 Agent（3 业务 + 1 评估 Agent）+ 初始声誉分的冷启动流程 |
| shadcn/ui | 基于 Radix UI + Tailwind 的前端组件库 |
| shardValidations | Registry 合约的 256 分片验证记录存储 |
| shardJobs | Job 合约的 256 分片 Job 状态存储 |
| unstake | Validator 解锁质押，含 7 天解锁期 |
| Validator | 持 100+ MON 质押后可提交 Agent 验证分数的生态参与者 |
| ValidationRecord.source | 声誉记录来源标识：0=Validator（人为评分）/ 1=Evaluator-Job（Job complete 触发）/ 2=Evaluator-Arbitration（仲裁裁决触发） |
| 声誉历史曲线 | Agent 详情页的声誉变化时间序列图，每个数据点对应一条 ValidationRecord，按 source 着色区分 |
| x402 | HTTP 原生支付协议（HTTP 402 状态码），由 Coinbase 主导，Linux Foundation 孵化 |

---

## 10. 附录

### 10.1 参考标准

| 标准 / 协议 | 用途 | 关系 |
|---|---|---|
| ERC-8004 | Agent 身份 / 声誉 / 验证三段式接口标准 | 借鉴接口形态，独立实现（256 分片 + 质押加权 + EPOCH 限速） |
| ERC-8183 | Agentic Commerce Protocol（A2A 商业协议） | 严格对齐核心 4 态状态机 + ArbitrationHook 扩展 |
| x402 | HTTP 原生支付协议 | 接入 settle 接口作为 Job 资金来源 |
| AEP | Agentic Execution Protocol（多级任务分发模型） | 借鉴叙事模型，补齐资金托管 + 交付验证两个不闭环环节 |
| EIP-3009 | Transfer with Authorization（USDC 支持） | x402 settle 依赖的稳定coin 标准 |

### 10.2 关键参数

| 参数 | 值 | 说明 |
|---|---|---|
| 分片数 | 256 | 8 bits，500 并发下生日界限 P(冲突)~30% + 延迟聚合 → 压测验证 |
| 分片键 | `agentId & 0xFF` / `jobId & 0xFF` | 低 8 位，分布均匀 |
| EPOCH | 1 minute | 高并发写和聚合读的时间维度解耦 |
| LRU 缓存 | 1000 条 | 区块头缓存大小 |
| Validator 最低质押 | 100 MON | 防 Sybil |
| unstake 解锁期 | 7 天 | 防止作恶后立即跑路 |
| checkTrust ALLOW 阈值 | reputation > 80%（合约内部比较值 0.8e18，数据库可配置，前端展示百分比） |
| checkTrust DENY 阈值 | reputation < 30%（合约内部比较值 0.3e18，数据库可配置，前端展示百分比） |
| checkTrust REQUIRE_VALIDATION | 30% ≤ reputation ≤ 80%（数据库可配置） |
| API p99 延迟 | < 200ms | 性能指标 |
| Marketplace 首屏 | < 2s | 性能指标 |
| 一键调用响应 | < 3s | 含 Agent 执行 |
| Indexer 同步延迟 | 落后 head < 5 区块 | 健康检查 |
| Evaluator 决策延迟 | 5s 内 | 监听 Submitted/Disputed 事件后 |
| 轮询频率 | 5s 一次 | 前端数据刷新 |
| 路径点击数 | ≤ 3 次 | UX 设计约束 |

### 10.3 技术栈

| 层 | 技术栈 |
|---|---|
| 智能合约 | Solidity 0.8.24+ / Foundry / Cancun EVM |
| Indexer / Evaluator | Go / go-ethereum / zap（结构化日志） |
| Marketplace | Next.js / React 19 / Tailwind CSS v4 / shadcn/ui / Framer Motion / GSAP |
| 部署 | docker-compose / anvil / Monad 测试网 |
| 数据库 | PostgreSQL（Indexer 入库） |

### 10.4 测试net 资源

| 资源 | 地址 / 链接 |
|---|---|
| Monad 测试网 chainId | 10143 |
| Monad 官方 x402 facilitator | https://x402-facilitator.molandak.org |
| testnet USDC 合约（Circle 水龙头） | `0x534b2f3A21130d7a60830c2Df862319e593943A3` |

### 10.5 视觉组件清单

| 组件 | 用途 | 对应需求 |
|---|---|---|
| PrismHologram | 旋转棱镜，项目名字面可视化 | FR-M01 |
| ShardHeatmap | 16×16 网格，把 OCC 抽象变成视觉直觉 | FR-M08 |
| ReorgAwareFeed | 实时事件流带 reorg 标记，证明 infra 可信 | FR-M07 |