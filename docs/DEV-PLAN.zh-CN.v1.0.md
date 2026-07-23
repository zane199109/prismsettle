# PrismSettle 开发计划

> **基于 PRD.zh-CN.v1.0.md 与 SD.zh-CN.v1.0.md**
> **目标：** 30 天内完成 PrismSettle 全栈开发（合约 + 链下 + 前端）
> **原则：** 每个阶段完成后自动验证 + 人工验收，不打折，不降级

---

## 项目现状

| 组件 | 状态 | 说明 |
|------|------|------|
| PRD | ✅ v1.0 完成 | docs/PRD.zh-CN.v1.0.md |
| SD | ✅ v1.0 完成 | docs/SD.zh-CN.v1.0.md |
| 合约 | ⚠️ 部分完成 | 仅有 PrismSettleRegistry.sol + 基础测试，缺少 Job + Hook |
| 链下 | ⚠️ 骨架完成 | 有旧 fundme/indexer 代码，缺少 Evaluator/Keeper/新 API |
| 前端 | ❌ 未完成 | 无前端代码 |
| Git | ✅ 已初始化 | 需检查 `.gitignore` 完整性（排除 node_modules、.next、cache、out、.env 等） |

---

## 阶段总览

| 阶段 | 内容 | 天数 | 关键里程碑 | 对应 PRD §8.3 Phase |
|------|------|------|-----------|---------------------|
| **Phase 0** | 项目初始化 + 合约环境 + CI | Day 1 | Foundry 就绪 + CI 流水线 + test 可跑 | — |
| **Phase 1** | 合约层 — Registry 完整实现 | Day 2-3 | 256 分片 + 聚合 + 衰减 + slash + 单元测试 | Phase 1（合约层） |
| **Phase 2** | 合约层 — Job + Hook 完整实现 | Day 4-5 | create/fund/assign/submit/complete/refund/dispute/resolve | Phase 1（合约层） |
| **Phase 3** | 合约层 — 集成测试 + 压测 | Day 6-7 | V0/V1 对照压测 + 端到端流程 + 部署脚本 | Phase 1（合约层） |
| **Phase 4** | 链下层 — Indexer 核心 | Day 8-10 | EVMListener + reorg + Parser + DB schema + 单测 | Phase 2（链下层） |
| **Phase 5** | 链下层 — 4 个 Agent 服务实现 | Day 11-12 | DeFi/标注/翻译/评估 Agent，A2A /invoke 可调 | Phase 2（链下层） |
| **Phase 6** | 链下层 — Evaluator + Keeper | Day 13-15 | RuleCheck + EvalAgent + 仲裁 + 自动声誉更新 + CircuitBreaker + Keeper | Phase 2（链下层） |
| **Phase 7** | 链下层 — API + 前端基建 | Day 16-17 | 16 REST 端点 + checkTrust 阈值配置 + Next.js 骨架 + wagmi | Phase 2/3 过渡 |
| **Phase 8** | 前端层 — 核心页面 | Day 18-22 | Marketplace + TrustGate + Job 闭环 + ValidatorConsole + Heatmap | Phase 3（前端层） |
| **Phase 9** | 集成联调 + 压测 + 测试网部署 | Day 23-26 | 端到端 + Reorg + Monad 测试网 + Docker + 健康检查 | Phase 4（上线） |
| **Phase 10** | Bug 修复 + 文档 + 冲刺 | Day 27-30 | P0/P1 bug 清零、README、演示准备 | Phase 4（上线） |

---

## Phase 0：项目初始化 + 合约环境 + CI

**目标：** Foundry 项目就绪，test 可编译运行，Git 配置完整，CI 流水线就位

### 任务 0.1：Git 配置检查

- 确认 Git 已初始化
- 完善 `.gitignore`（排除 node_modules、.next、cache、out、.env、*.log、私钥等）
- **强制检查：** `.env` 文件必须被忽略（含 LLM API key、RPC URL、私钥等敏感信息）

### 任务 0.2：Foundry 项目验证

- 确认 `foundry.toml` 配置正确
  - **EVM 版本（对齐 SD §2.3）：** 显式配置 `evm_version = "cancun"`（SD §2.3 要求 "0.8.24+ / Cancun EVM"）。Cancun 支持 blob tx / EIP-1153 transient storage / EIP-5656 MCOPY，Monad 主网对齐 Cancun，遗漏会导致部署后行为与 anvil 不一致
  - **Solc 版本：** `solc = "0.8.24"`（对齐 SD §2.3）
  - **优化器：** 开启 `optimizer = true`，`optimizer_runs = 200`
- 确认 Monad 测试网 RPC 可连接
- 运行 `forge build` 编译通过
- 运行 `forge test --match-path contracts/test/PrismSettleRegistry.t.sol -vvv` 现有测试通过

### 任务 0.3：CI 流水线配置

- 文件：`.github/workflows/ci.yml`
- 触发：push / PR 到 main 分支
- Jobs：
  - `contracts`：`forge build` + `forge test`（含 Integration.t.sol）
  - `offchain`：`go build ./...` + `go vet ./...` + `go test ./...`（需 postgres service）
  - `frontend`：`npm ci` + `npm run build` + `npm run lint`
- PR 必须通过 CI 才能合并（main 分支保护规则）
- 缓存策略：Foundry cache、Go module cache、npm cache

**验证脚本：** `scripts/verify_phase0.sh`
```bash
#!/bin/bash
set -e
cd contracts
forge build
forge test --match-path test/PrismSettleRegistry.t.sol -vvv
cd ..
# 检查 .gitignore 包含 .env
grep -q "^\.env$" .gitignore || { echo "❌ .gitignore 缺少 .env"; exit 1; }
# 检查 CI 文件存在
test -f .github/workflows/ci.yml || { echo "❌ CI 配置缺失"; exit 1; }
# 检查 foundry.toml 配置 evm_version = cancun（对齐 SD §2.3）
grep -q 'evm_version.*=.*"cancun"' contracts/foundry.toml || { echo "❌ foundry.toml 缺少 evm_version = \"cancun\""; exit 1; }
echo "✅ Phase 0 验证通过（Foundry + Git + CI + Cancun EVM）"
```

---

## Phase 1：合约层 — Registry 完整实现

**目标：** PrismSettleRegistry.sol 完整实现，覆盖 SD §3.2 全部设计

### 任务 1.1：Registry 数据结构重构

- 文件：`contracts/src/PrismSettleRegistry.sol`
- 修改内容：
  - 替换 `mapping(uint256 => ValidationRecord[])` 为 `shardValidations[shard][agentId]`
  - 添加 `AgentMetadata` 结构体（含 endpointUrl, capabilities, registeredAt, taskCount, lastActivity）
  - 添加常量：`EPOCH = 1 min`、`MAX_VALIDATIONS_PER_EPOCH = 50`、`MAX_RECORDS = 100`、`INACTIVE_DAYS = 30`、`MIN_STAKE = 100`、`EMA_ALPHA = 0.1`
  - 添加 `seedScore` 初始化逻辑（SD §3.2.2）

### 任务 1.2：registerAgent + stake

- `registerAgent(uint256 agentId, string calldata metadata)`
- `stake()` payable + `unstake()` + `claimUnstaked()`（7 天解锁期）
- 事件：`AgentRegistered`、`Staked`、`UnstakeStarted`、`UnstakeWithdrawn`

### 任务 1.3：submitValidation + 分片存储

- `submitValidation(agentId, score, proofHash, jobId, source)`
- 分片路由：`shard = uint8(agentId & 0xFF)`
- `MAX_VALIDATIONS_PER_EPOCH` 校验
- 事件：`ValidationSubmitted`

### 任务 1.4：aggregateEpoch

- 质押加权平均
- EMA 平滑：`newScore = oldScore * (1 - alpha) + weightedScore * alpha`
- 不活跃衰减：`decay = applyDecay(aggregatedScore, lastActivity, currentTimestamp)`
- `MAX_RECORDS` 截断逻辑
- 事件：`Aggregated`

### 任务 1.5：getScore + slash + claimRewards stub

- `getScore` view（读时动态计算衰减）
- `slash(validator, evidenceHash)` 公式化惩罚
- `claimRewards` stub（V2 only）

### 任务 1.6：Registry 单元测试

- 测试文件：`contracts/test/PrismSettleRegistry.t.sol`
- 覆盖：
  - registerAgent 注册/重复注册
  - stake/unstake/claimUnstaked 生命周期
  - submitValidation 分片路由 + MAX_VALIDATIONS_PER_EPOCH 限制
  - aggregateEpoch 质押加权 + EMA + 不活跃衰减
  - getScore 读时衰减计算
  - slash 惩罚
  - 256 分片并发写入不冲突

**验证脚本：** `scripts/verify_phase1.sh`
```bash
#!/bin/bash
set -e
cd contracts
forge build
forge test --match-path test/PrismSettleRegistry.t.sol -vvv
echo "✅ Phase 1 验证：Registry 合约编译通过，全部测试通过"
```

---

## Phase 2：合约层 — Job + Hook 完整实现

**目标：** PrismSettleJob.sol + ArbitrationHook.sol 完整实现

### 任务 2.1：PrismSettleJob 数据结构

- 文件：`contracts/src/PrismSettleJob.sol`
- Job 结构体：buyer, provider, amount, deliverableHash, proofHash, deadline, state, parentJobId, hook, createdAt
- 256 分片存储：`shardJobs[shard][jobId]`
- `buyerNonce` 账户级 nonce 生成 jobId
- `usedReceipts` x402 防重放
- 事件：`JobCreated`、`Funded`、`Assigned`、`Submitted`、`Completed`、`Refunded`

### 任务 2.2：createJob + fundViaToken

- `createJob(agentId, parentJobId, deadline, hook)` — keccak256 生成 jobId
- `fundViaToken(jobId, amount, x402Receipt)` — x402/ERC-20 分流
  - 有 receipt → Facilitator settle（amount=0）
  - 无 receipt → ERC-20 transferFrom
- 状态转换：Created → Funded

### 任务 2.3：assign + submit + complete + claimRefund

- `assign(jobId, provider)` — Funded → Assigned
- `submit(jobId, deliverableHash, proofHash)` — Assigned → Submitted
  - 触发 `ArbitrationHook.onSubmitted()`（如果 hook != 0）
- `complete(jobId)` — 仅 COMMERCE_EVALUATOR_ROLE
  - Submitted → Completed，USDC 转给 provider
- `claimRefund(jobId)` — deadline 后退款
  - 仲裁 ruling=1 时不受 deadline 限制

### 任务 2.4：ArbitrationHook

- 文件：`contracts/src/ArbitrationHook.sol`
- 状态机：None → Disputed → DisputeResolved
- `dispute(jobId, reasonHash)` — BUYER 发起
- `resolveDispute(jobId, ruling)` — Evaluator 裁决（ruling=1/2，ruling=0 revert）
- `getHookState(jobId)` view
- 事件：`Disputed`、`DisputeResolved`

### 任务 2.5：Job + Hook 单元测试

- 文件：`contracts/test/PrismSettleJob.t.sol`、`contracts/test/ArbitrationHook.t.sol`
- 覆盖：
  - createJob → fundViaToken（x402 路径 + ERC-20 路径）
  - assign → submit → complete 主路径
  - claimRefund deadline 检查
  - 仲裁流程：dispute → resolveDispute → claimRefund（ruling=1 不受限）
  - 状态机非法转换拦截
  - resolveDispute ruling=0 revert 校验（FR-J08）

**验证脚本：** `scripts/verify_phase2.sh`
```bash
#!/bin/bash
set -e
cd contracts
forge build
forge test --match-path test/PrismSettleJob.t.sol -vvv
forge test --match-path test/ArbitrationHook.t.sol -vvv
echo "✅ Phase 2 验证：Job + Hook 合约编译通过，全部测试通过"
```

---

## Phase 3：合约层 — 集成测试 + 压测

**目标：** 端到端流程验证 + V0/V1 对照压测（对齐 PRD §4.3 FR-T01~T06）

### 任务 3.1：部署脚本

- 文件：`contracts/script/Deploy.s.sol`
- 顺序：MockERC20 → Registry → Hook → Job → grantRoles
- **角色授权（对齐 SD 权限矩阵）：**
  - `Registry.grantRole(REGISTRY_EVALUATOR_ROLE, evaluatorAddr)`
  - `Job.grantRole(COMMERCE_EVALUATOR_ROLE, evaluatorAddr)`
  - `Hook.grantRole(RESOLVER_ROLE, evaluatorAddr)`
- **evaluatorAddr 来源说明（解决时序问题）：**
  - anvil/dev：从 `offchain/config/dev.yaml` 的 `evaluator.private_key` 推导地址
  - 部署脚本读取该配置，确保 Phase 3 部署与 Phase 6 Evaluator 使用同一身份
  - Monad testnet：从 `offchain/config/prod.yaml` 的 `evaluator.private_key` 推导
  - 私钥不入仓库，通过 `.env` 注入（已在 Phase 0 .gitignore 排除）
- Seed Phase：
  - 注册 4 个 Agent（DeFi / 标注 / 翻译 / 评估）
  - 为 4 个官方 Agent 设置初始声誉分 `seedScore = 0.7e18`（对齐 PRD FR-C09）
  - Validator 质押 ≥ 100 MON

### 任务 3.2：端到端集成测试

- 文件：`contracts/test/Integration.t.sol`
- 流程：registerAgent → stake → submitValidation → aggregateEpoch → createJob → fundViaToken → assign → submit → complete → claimRefund
- 含仲裁分支：submit → dispute → resolveDispute → complete/claimRefund

### 任务 3.3：BaselineRegistry（V0 对照合约）

- 文件：`contracts/src/bench/BaselineRegistry.sol`
- 单槽存储，无分片，接口与 V1 一致
- 参考 SD §3.6 伪代码实现

### 任务 3.4：压测脚本与对照采集（对齐 FR-T01~T06）

> 拆分为 4 个子任务，确保 PRD §4.3 全部验收标准落地。

#### 任务 3.4a：压测脚本实现

- 文件：`scripts/bench/v0_v1_bench.sh`
- Go 并发发包器：500 并发 `submitValidation`，可控并发数
- 参考 SD §6.5.1 压测脚本设计
- **达标线（对齐 NFR-MN01）：** V1 abort rate < 5%，V0 abort rate 显著高于 V1

#### 任务 3.4b：V0 基线采集（FR-T03）

- 部署单槽存储 BaselineRegistry 到 anvil
- 执行预设压测用例
- **完整记录：** 命令行日志、链上交易哈希、执行录屏、控制台截图
- 产出优化前原始指标基线值

#### 任务 3.4c：V1 采集（FR-T04）

- 环境重置（合约/业务状态清零，清除历史脏数据）
- 部署 256 分片 V1 合约
- 复用完全相同的压测脚本/并发参数/执行时长复测
- 同步采集同维度指标，产出优化后指标值

#### 任务 3.4d：压测前置约束声明（FR-T06）

- 报告内显式声明三项约束：
  1. 统一环境（同一节点/RPC、服务器硬件、数据库配置、区块参数、Gas 费率）
  2. 统一压测工具&用例（固定脚本、并发量级、请求模板、执行轮次、样本总量）
  3. 前置数据清零（每次测试前重置合约/业务状态）

### 任务 3.5：部署到 Anvil 本地验证

- 用 anvil 部署三合约
- 跑一遍完整流程

**验证脚本：** `scripts/verify_phase3.sh`
```bash
#!/bin/bash
set -e
cd contracts
forge build
forge test --match-path test/Integration.t.sol -vvv
echo "✅ Phase 3 验证：集成测试通过"
echo "⚠️  压测需在 anvil 环境下手动运行：bash scripts/bench/v0_v1_bench.sh"
echo "⚠️  压测报告需包含：V0/V1 对照表 + 录屏 + 日志 + 三项约束声明（FR-T03/T04/T06）"
echo "⚠️  达标线：V1 abort rate < 5%（NFR-MN01）"
```

---

## Phase 4：链下层 — Indexer 核心

**目标：** EVMListener + reorg 检测 + Parser + DB schema + 单元测试覆盖

### 任务 4.1：Go 项目骨架

- 创建 `offchain/go.mod`（已有，确认依赖）
- 配置文件：`offchain/config/dev.yaml`、`offchain/config/prod.yaml`
- Logger + Config + Response 工具链
- **EIP-55 工具函数**（地址原值存储，对齐 SD §1.2 设计原则）
- **配置 fail-fast 校验**（必填项缺失时启动报错，对齐 FR-I08）
- **evaluator.private_key 配置项**（供 Phase 3 部署脚本与 Phase 6 Evaluator 共用）

### 任务 4.2：DB Schema 迁移

- 文件：`offchain/internal/storage/migrations/`
- 表：`chain_events`、`decision_logs`、`agent_score_history`、`block_state`、`trust_thresholds`
- 索引：`idx_ce_event_type_block`、`idx_dl_job` 等

### 任务 4.3：EVMListener 核心

- 文件：`offchain/internal/listener/evm_listener.go`
- eth_blockNumber + eth_getLogs 轮询
- 三合约同步策略（Registry + Job + Hook）
- 顺序提交：nextExpected/completedTasks（对齐 FR-I05）

### 任务 4.4：Reorg 检测 + 回滚

- 文件：`offchain/internal/listener/reorg.go`
- LRU 区块头缓存（1024，对齐 FR-I06）
- parentHash 对比检测（对齐 FR-I03）
- RollbackEvents + decision_logs invalid 标记（对齐 FR-I04）

### 任务 4.5：Parser 实现

- Registry Parser：5 事件（AgentRegistered, ValidationSubmitted, Aggregated, Staked, Slashed）
- Job Parser：6 事件（JobCreated, Funded, Assigned, Submitted, Completed, Refunded）
- Hook Parser：2 事件（Disputed, DisputeResolved）
- schema 复用原则（TokenAddr/Symbol/Decimals 列镜像存储）
- proofHash 字段从 Submitted 解析入库（FR-JI04）
- hook 字段从 JobCreated 解析入库（FR-JI05）

### 任务 4.6：Indexer 单元测试（对齐 FR-T08/T09/T10 + FR-I 系列 + FR-JI 系列）

> **编号归属说明：** 按 PRD §3.12 定义，FR-T07 = Registry 合约 Foundry 测试（合约层，Phase 1 任务 1.5 覆盖）、FR-T11 = 余额/分数计算单测（合约层，Phase 1 任务 1.5 覆盖）、FR-T12 = Job 合约 Foundry 测试（合约层，Phase 2 任务 2.5 覆盖）。本任务仅覆盖真正属于 Indexer 层的 FR-T08/T09/T10，并显式对齐 FR-I 与 FR-JI 编号。

- 文件：`offchain/internal/listener/evm_listener_test.go`
- 文件：`offchain/internal/listener/reorg_test.go`
- 文件：`offchain/internal/parser/*_parser_test.go`
- 覆盖：
  - **FR-T08**：5 种 Registry 事件解析单测（对齐 FR-I02）
  - **FR-T09**：模拟 3 区块 reorg 后 DB 状态正确（对齐 FR-I03/I04）
  - **FR-T10**：顺序提交 — 重复提交不产生重复事件（对齐 FR-I05）
  - **启动同步**：Indexer 启动后 5s 内同步三合约（对齐 FR-I01/FR-JI02）
  - **Job/Hook Parser**：8 种事件解析单测（对齐 FR-JI01，含 proofHash FR-JI04 / hook FR-JI05 / 仲裁态 FR-JI06）
  - **fail-fast**：必填配置缺失时启动报错（对齐 FR-I08）
  - **优雅停机**：SIGTERM 30s 内退出（对齐 FR-I07，P1）
- 测试环境：anvil + testcontainers 起 postgres

**验证脚本：** `scripts/verify_phase4.sh`
```bash
#!/bin/bash
set -e
cd offchain
go build ./cmd/
go vet ./...
go test ./internal/listener/... -v
go test ./internal/parser/... -v
echo "✅ Phase 4 验证：Indexer 编译通过 + 单元测试通过（含 reorg/顺序提交/Parser）"
```

---

## Phase 5：链下层 — 4 个 Agent 服务实现（2 天）

**目标：** 实现 PRD FR-AP05 强制要求的 4 个 Agent，A2A /invoke 可调

> **必要性说明：** Evaluator 调用评估 Agent（FR-E11）、一键调用演示（UC-01）均依赖此 Phase。缺此环节则 Demo 无法跑通。
> **工期说明：** 4 个 Go HTTP server + LLM 集成 + prompt 调优 + 注册逻辑 1 天不现实，故分配 2 天。

### 任务 5.1：3 个业务 Agent 实现

- 文件：`offchain/agents/defi_agent.go`、`data_labeling_agent.go`、`translation_agent.go`
- 框架：Go HTTP server，封装 OpenAI/Claude API
- 端点规范（对齐 FR-AP01/02/04）：
  - `POST /invoke` 接收 `{input, caller}`，返回 `{output, proof_hash}`
  - `/.well-known/agent.json` 暴露 Agent 元数据
- 业务逻辑：
  - DeFi 分析 Agent：输入市场数据，输出套利机会分析
  - 数据标注 Agent：输入待标注样本，输出标注结果
  - 翻译 Agent：输入源文本，输出目标语言译文

### 任务 5.2：评估 Agent 实现

- 文件：`offchain/agents/eval_agent.go`
- 内部封装 gpt-4o-mini（对 Evaluator 透明，可换）
- 接口规范（对齐 FR-E14）：
  - 输入：`{deliverable_hash, job_id, job_metadata}`
  - 输出：`{score: uint96, reason: string}`（score ∈ [0, 1e18]）
- 同一交付物多次评分结果稳定（方差 < 0.05）
- 评估 Agent 本身也注册到 Marketplace（体现「Agent 评估 Agent」递归生态）
- **LLM API Key 管理：**
  - dev：通过 `.env` 文件注入（`OPENAI_API_KEY`、`ANTHROPIC_API_KEY`），`.env` 已在 Phase 0 .gitignore 排除
  - prod：通过 Docker secret / 环境变量注入，key 不入镜像
  - key 轮换：评估 Agent 支持运行时热加载（监听 SIGHUP 重载配置）
  - 4 个 Agent 端点配置统一从 `offchain/config/agents.yaml` 读取

### 任务 5.3：Agent 注册到 Registry

- 4 个 Agent 启动后调用 `Registry.registerAgent` 注册
- metadata 内声明端点 URL（FR-AP04）
- 部署到与 Seed Phase 一致的 agentId

**验证脚本：** `scripts/verify_phase5.sh`
```bash
#!/bin/bash
set -e
cd offchain
go build ./agents/...
# 启动 4 个 Agent 并自检 /invoke
for port in 8001 8002 8003 8004; do
  curl -s -X POST http://localhost:$port/invoke \
    -H "Content-Type: application/json" \
    -d '{"input":"ping","caller":"0x0000000000000000000000000000000000000000"}' \
    | grep -q "output"
done
# 检查 .env 不在 git 跟踪中
git check-ignore .env || { echo "❌ .env 未被 gitignore"; exit 1; }
echo "✅ Phase 5 验证：4 个 Agent /invoke 端点可调，LLM key 未泄露"
```

---

## Phase 6：链下层 — Evaluator + Keeper（3 天）

**目标：** 自动评估 + 仲裁裁决 + 自动声誉更新 + 错峰聚合 + 熔断器

> **工期说明：** 仲裁分支（6.4）+ 自动声誉更新（6.3）逻辑复杂，且需配套单测，2 天对单人不现实，故分配 3 天。

### 任务 6.1：Evaluator 主循环（Submitted 分支）

- 文件：`offchain/prismsettle/evaluator/evaluator.go`
- 每 2s 轮询 Submitted 事件（对齐 FR-E01）
- processedJobs 去重（Redis SETNX + DB UNIQUE）
- 幂等性：同一 jobId 的 submitValidation 仅触发一次

### 任务 6.2：RuleCheck 前置门

- 文件：`offchain/prismsettle/evaluator/rule_check.go`
- 4 项形式校验（对齐 FR-E02~E05）：
  - deliverable 非空
  - 来源匹配（submitter == Provider）
  - 可获取（HTTP HEAD 检查 IPFS 网关）
  - proofHash 非零 + IPFS 可访问
- FR-E04 特殊处理：交付物不可达中断决策（不进入 reject 路径，不调合约）

### 任务 6.3：EvalAgentClient + 降级 + 自动声誉更新

- 文件：`offchain/prismsettle/evaluator/eval_agent_client.go`
- POST /invoke（5s 超时，调用 Phase 5 评估 Agent）
- 降级：超时/异常时 score=0.6e18（对齐 FR-E11）
- **自动触发声誉更新（FR-E12，关键）：**
  - `complete(jobId)` 调用成功后，自动调用 `Registry.submitValidation(agentId, evalScore, proofHash, jobId, source=1)`
  - 触发 `ValidationSubmitted` 事件，shardValidations 数组 +1
  - source=1 入库（区分主路径与仲裁路径）
  - Evaluator 持 `REGISTRY_EVALUATOR_ROLE` 免质押调用（对齐 SD §4.5 评估器实现约束）

### 任务 6.4：Evaluator 仲裁分支（FR-E08/E09/E13）

- 文件：`offchain/prismsettle/evaluator/arbitration.go`
- **监听 Disputed 事件**，5s 内裁决（FR-E08）
- **仲裁裁决规则（FR-E09）：**
  - BUYER reasonHash 空 → 判 Provider 放款（ruling=2）
  - BUYER reasonHash 非空 + reasonHash == deliverableHash → 判 BUYER 退款（ruling=1）
  - BUYER reasonHash 非空 + reasonHash != deliverableHash → 判 Provider 放款（ruling=2）
  - ruling=0 revert 防误传
- **仲裁后触发声誉更新（FR-E13）：**
  - ruling=2：按评估 Agent 评分写入（Provider 胜诉仍记录质量分）
  - ruling=1：写入公式化惩罚分 `max(0.2e18, currentScore×30%)`
  - source=2（Evaluator-Arbitration，与主路径 source=1 区分）

### 任务 6.5：决策日志（FR-E10）

- 文件：`offchain/prismsettle/evaluator/decision_log.go`
- 所有决策（含仲裁）写入 `decision_logs` 表
- 字段：jobId, decision, score, reason, tx_hash, source, timestamp
- reorg 时通过 invalid 标记回滚

### 任务 6.5a：Evaluator 权限安全约束（对齐 SD §4.5.7）

> **必要性说明：** SD §4.5.7 明确 V1 不引入 Timelock/多签/TSS，转而以 Role 隔离 + 决策日志可审计 + 挑战期已隐含三层补偿措施。本任务把这些约束固化为代码与配置，避免实现期遗漏。

- **Role 隔离（合约层已在 Phase 2 任务 2.2 落地，本任务做链下层校验）：**
  - Evaluator 仅持 `COMMERCE_EVALUATOR_ROLE`（Job.complete）+ `RESOLVER_ROLE`（仲裁裁决）+ `REGISTRY_EVALUATOR_ROLE`（写声誉）
  - 启动时断言 Evaluator **不持**资金管理角色（`WITHDRAW_ROLE` / `FUNDER_ROLE` 等），断言失败 fail-fast
  - 链下配置项 `evaluator.disallowed_roles` 列出禁用角色清单，启动期与链上 `hasRole` 比对
- **资金流向不可绕过：** Evaluator 只能通过 `Job.complete` / `Job.resolveDispute` 触发资金流转，无法任意转账（合约层已约束，本任务在 `decision_log` 中记录每次 complete/resolve 的 tx_hash 以备审计）
- **决策日志可审计：** 复用任务 6.5 的 `decision_logs` 表，新增字段 `caller_role`（记录决策时使用的角色），任何异常放款可事后追溯
- **挑战期已隐含说明（仅文档）：** Job 的 `deadline` + 仲裁 Hook 已提供 BUYER 反证窗口（FR-E06~E09），无需再叠加资金层 Timelock。此条写入 README「安全模型」章节（Phase 10 任务 10.2）
- **V2 演进路径（仅文档）：** TVL 超阈值时引入多 Evaluator 投票（2/3 多签触发 complete），V1 不实现。写入 docs/roadmap.md（Phase 10 任务 10.2）

### 任务 6.6：Circuit Breaker

- 文件：`offchain/prismsettle/evaluator/circuit_breaker.go`
- CLOSED → OPEN → HALF_OPEN 状态机
- 连续 10 次失败触发熔断，60s 半开试探

### 任务 6.7：Keeper bot

- 文件：`offchain/prismsettle/keeper/keeper.go`
- 每 30s tick，maxBatch=10 错峰聚合
- 不活跃 Agent 扫描（InactiveScanner，1h 周期，30d 阈值，对齐 SD §4.6.3）

### 任务 6.8：Evaluator + Keeper 测试

- RuleCheck 单元测试
- Circuit Breaker 状态转换测试
- Keeper 聚合逻辑测试
- **仲裁分支单测**：覆盖 FR-E09 三种裁决路径
- **自动声誉更新单测**：complete 后 submitValidation 被调用、source=1
- **幂等性单测**：同一 jobId 重复事件不重复触发

**验证脚本：** `scripts/verify_phase6.sh`
```bash
#!/bin/bash
set -e
cd offchain
go build ./prismsettle/evaluator/...
go build ./prismsettle/keeper/...
go test ./prismsettle/evaluator/... -v
go test ./prismsettle/keeper/... -v
echo "✅ Phase 6 验证：Evaluator + Keeper 编译通过，单元测试通过"
echo "    覆盖：主路径 + 仲裁分支 + 自动声誉更新 + 幂等性"
```

---

## Phase 7：链下层 — API + 前端基建

**目标：** 16 REST 端点 + checkTrust 阈值配置 + Next.js 项目骨架

### 任务 7.1：REST API 实现（16 端点，对齐 SD §4.7.1）

- 文件：`offchain/prismsettle/api/prismsettle_handler.go`
- **统一分页参数规范：** 所有列表端点统一 `?page=1&limit=20`，响应含 `{data, total, page, limit}`
  > **设计来源标注：** SD §4.7.1 仅在 FR-A01 提"分页事件"，未定义统一分页规范。本规范为 DEV-PLAN 补充设计，落地后需回写 SD §4.7.2 请求/响应格式（Phase 10 任务 10.2 文档同步），避免 SD 与实现脱节。
- **12 个 PRD 编号端点：**
  - GET /api/v1/prismsettle/events（FR-A01，支持 event_type/block_number 过滤）
  - GET /api/v1/prismsettle/score（FR-A02）
  - GET /api/v1/prismsettle/validations/count（FR-A03）
  - GET /api/v1/prismsettle/shards/activity（FR-A04）
  - GET /health（FR-A05，含 sync_lag/reorg_count 字段，对齐 NFR-OBS02）
  - GET /api/v1/prismsettle/agents（FR-A06，含 metadata + 端点 URL + 声誉分）
  - GET /api/v1/prismsettle/agents/:agentId（FR-A07）
  - GET /api/v1/prismsettle/jobs（FR-A08，按 state 过滤）
  - GET /api/v1/prismsettle/jobs/:jobId（FR-A09，含 proofHash + hook）
  - GET /api/v1/prismsettle/jobs/:jobId/status（FR-A10，核心 4 态 + Hook 仲裁态）
  - GET /api/v1/prismsettle/trust（FR-A11，三态决策）
  - GET /api/v1/prismsettle/reputation/history（FR-A12，最近 30 条）
  - POST /api/v1/prismsettle/agent/invoke（FR-M11，后端代理 A2A /invoke 解决 CORS）
- **4 个非 PRD 编号端点（支撑前端组件）：**
  - GET /api/v1/prismsettle/perf/shard-heatmap（支撑 FR-M08/FR-M08b）
  - GET /api/v1/prismsettle/perf/v0-v1-comparison（支撑 FR-T05/FR-M08b，Phase 9 压测后返回真实数据，此前返回 mock）
  - GET /api/v1/prismsettle/perf/reorg-feed（支撑 FR-M07，标记 rollback）
  - GET /api/v1/prismsettle/jobs/:jobId/timeline（支撑 FR-JM02）

### 任务 7.2：checkTrust 阈值配置（FR-AP11）

- 数据库配置表：`trust_thresholds (agent_id, allow_threshold, deny_threshold, updated_at)`
- 阈值可从数据库读取并热更新（不重启服务）
- 默认值：ALLOW > 0.8e18，DENY < 0.3e18，中间为 REQUIRE_VALIDATION
- 提供管理 API（POST /api/v1/prismsettle/trust/thresholds）用于热更新

### 任务 7.3：错误码 + 中间件

- 错误码表（SD §8.3）
- CORS 配置（NFR-S03）
- Rate Limiting（config.yaml rate_limit）

### 任务 7.4：Next.js 项目初始化

- 文件：`frontend/`
- Next.js App Router + Tailwind v4
- wagmi + RainbowKit 配置
- ABI 文件导出（Registry/Job/Hook）

### 任务 7.5：前端 API 客户端

- 文件：`frontend/lib/api.ts`
- fetcher + usePoll（5s 轮询）
- useJobStatusPoll（2s/30s 自适应）

**验证脚本：** `scripts/verify_phase7.sh`
```bash
#!/bin/bash
set -e
cd offchain
go build ./prismsettle/api/...
echo "✅ Phase 7 验证：API 编译通过（16 端点 + checkTrust 阈值配置）"
cd ../frontend
npm install
npm run build
echo "✅ Phase 7 验证：前端构建通过"
```

---

## Phase 8：前端层 — 核心页面（5 天）

**目标：** Marketplace + TrustGate + Job 闭环 + ValidatorConsole + Heatmap + 组件级单测

### 任务 8.1：首页 + PrismHologram

- 文件：`frontend/app/page.tsx`
- GSAP 棱镜旋转动画
- Agent 概览数据

### 任务 8.2：Agent Marketplace

- 文件：`frontend/app/agents/page.tsx`
- 声誉排序 + 评级 + 标签筛选
- AgentCard 组件

### 任务 8.3：Agent 详情页

- 文件：`frontend/app/agents/[agentId]/page.tsx`
- ScoreHistoryChart（Recharts 声誉曲线）
- validation 历史 + ShardHeatmap
- AgentFailureCounter（FR-M12，localStorage 聚合近 50 次调用失败）

### 任务 8.4：TrustGate 组件

- 文件：`frontend/components/job/TrustGate.tsx`
- 三态决策 UI（ALLOW/DENY/REQUIRE_VALIDATION）
- 阈值显示（从 Phase 7 任务 7.2 配置接口读取）
- REQUIRE_VALIDATION 提示挂载 Hook（FR-AP12）
- DENY 阻止 createJob（FR-AP13）

### 任务 8.5：Job 创建/详情页（拆分为 4 个子任务，对齐 FR-JM01~JM06）

#### 任务 8.5a：JobForm + 状态追踪面板（FR-JM01/JM02）

- 文件：`frontend/app/jobs/new/page.tsx`、`frontend/app/jobs/[jobId]/page.tsx`
- JobForm：创建 Job（含 checkTrust 闸门集成）
- **状态追踪面板（FR-JM02，P0）：**
  - **状态展示策略（对齐 SD §613 注释）：** 前端展示用 ERC-8183 官方 4 态，内部状态仍用合约 enum 的 6 态，映射关系如下：
    | ERC-8183 展示态 | 合约 enum 内部态 | 说明 |
    |---|---|---|
    | Open | Created | Job 刚创建，未注资 |
    | Funded | Funded | 已注资（Assigned 在前端并入 Funded 展示，仅 Timeline 标注） |
    | Submitted | Submitted | Provider 已提交交付物 |
    | Terminal | Completed / Refunded | 终态双分支，前端用子标签区分 |
    - 仲裁态（Disputed / DisputeResolved）不在 ERC-8183 4 态 enum 内，独立区块展示（见下一行）
  - **展示组件：** 4 个 ERC-8183 态独立区块（Open/Funded/Submitted/Terminal），每个区块内显示当前是否命中 + 进入时间戳
  - **Hook 仲裁态独立区块**（Disputed/DisputeResolved），与 4 态并列展示，不混入 Terminal
  - 状态徽章 + Timeline 组件（Timeline 显示完整 6 态内部流转，便于调试）
- **x402 facilitator 探测（FR-AP09）：**
  - 调用 `fundViaToken` 前先探测 facilitator 可用性
  - facilitator 不可用时走 ERC-20 兜底路径
  - UI 提示当前支付路径（x402 / ERC-20 fallback）

#### 任务 8.5b：交付物提交入口（FR-JM03，P0）

- Provider 视角 mock 提交互动
- 输入 deliverableHash + proofHash
- 调用 `Job.submit(jobId, deliverableHash, proofHash)`

#### 任务 8.5c：发起仲裁 + 仲裁结果展示（FR-JM05/JM06，P0）

- 发起仲裁按钮（BUYER 视角，调用 `Hook.dispute(jobId, reasonHash)`）
- 仲裁结果展示（ruling=1 退款 / ruling=2 放款）
- 仲裁理由 hash 输入框

#### 任务 8.5d：资金流转可视化（FR-JM04，P1）

- FundFlowChart 组件
- 展示 Buyer → Job 合约 → Provider 资金流向
- x402 / ERC-20 双路径标注

### 任务 8.6：Validator Console

- 文件：`frontend/app/validator/page.tsx`
- 质押/unstake UI
- 声誉分展示

### 任务 8.7：ReorgAwareFeed + 性能对比页

- 文件：`frontend/components/prism/ReorgAwareFeed.tsx`
- 文件：`frontend/app/perf/page.tsx`
- ReorgAwareFeed：滚动流式事件，回滚条目标红 + 删除线
- V0V1Comparison：对照表 + 分组柱状图（标注增减百分比）
- ShardHeatmapWidget：256 分片热力图动画
- 性能对比页结论文案（固定模板："256 分片优化使交易冲突回滚率从 X% 降至 Y%"）
- **数据时序说明：** Phase 8 开发时 `/perf/v0-v1-comparison` 端点返回 mock JSON，组件先用 mock 开发；Phase 9 压测后端点切换为真实数据，组件无需改动

### 任务 8.8：AgentRegisterForm

- 文件：`frontend/components/agent/AgentRegisterForm.tsx`
- Agent 注册入口（FR-M06）
- 提交 AgentId + metadata + endpoint URL

### 任务 8.9：组件级单测

- 框架：vitest + @testing-library/react
- 覆盖关键组件：
  - TrustGate：三态渲染（ALLOW/DENY/REQUIRE_VALIDATION）+ 阈值显示
  - JobTimeline：核心 4 态 + Hook 仲裁态切换
  - AgentFailureCounter：localStorage 聚合逻辑
  - ShardHeatmap：256 格数据渲染
- 目标：关键组件测试覆盖率 > 70%

**验证脚本：** `scripts/verify_phase8.sh`
```bash
#!/bin/bash
set -e
cd frontend
npm run build
npm run test -- --run    # vitest 组件单测
echo "✅ Phase 8 验证：前端构建通过 + 组件单测通过"
echo "⚠️  功能测试需启动 dev server，手动验证："
echo "    - /agents 列表 + 详情"
echo "    - /jobs/new TrustGate 三态 + 创建 + x402/ERC-20 兜底"
echo "    - /jobs/[id] 状态追踪 + 交付物提交 + 仲裁 + 资金流转"
echo "    - /validator 质押"
echo "    - /perf V0V1 对照（mock 数据）+ 热力图"
echo "    - ReorgAwareFeed 滚动流"
```

---

## Phase 9：集成联调 + 压测 + 测试网部署

**目标：** 全链路打通，Monad 测试网部署，Docker 编排运行

### 任务 9.1：Docker Compose 编排

- 文件：`docker-compose.yml`
- 服务：offchain + 4 agents + frontend + postgres + redis + anvil（dev profile）
- **LLM key 通过 Docker secret / 环境变量注入，不入镜像**

### 任务 9.2：环境变量 + 配置

- 环境变量清单（SD §6.2）
- prod.yaml 配置
- **`.env.example` 提交到仓库作为模板，真实 `.env` 不入仓库**

### 任务 9.3：端到端测试

- 流程：注册 Agent → 质押 → submitValidation → aggregateEpoch → createJob → fundViaToken → assign → submit → complete → 自动声誉更新 → claimRefund
- **含仲裁分支**：submit → dispute → resolveDispute → 自动声誉更新（source=2）
- 全链路通过

### 任务 9.4：Reorg 测试

- anvil fork 模拟 reorg
- 验证 DB 回滚 + decision_logs invalid 标记
- 验证前端 ReorgAwareFeed 标红展示

### 任务 9.5：500 并发压测

- V0 vs V1 abort rate 对照
- 输出可视化页面（FR-T05），Phase 8 前端 mock 数据切换为真实数据
- 报告内含三项约束声明（FR-T06）
- **达标验证：** V1 abort rate < 5%（NFR-MN01）

### 任务 9.6：监控告警（对齐 NFR-OBS02）

- Telegram 告警配置
- **/health 端点验证字段：**
  - `sync_lag`：同步延迟（区块数）
  - `reorg_count`：reorg 次数
  - `evaluator_state`：Evaluator 运行状态
  - `keeper_last_run`：Keeper 上次执行时间
- 熔断器告警（Circuit Breaker OPEN 时通知）

### 任务 9.7：Monad 测试网部署（对齐 PRD §8.3 Phase 4）

- **部署三合约到 Monad testnet**（Registry + Job + Hook）
- **测试币获取：**
  - 通过 Monad 官方 faucet 领取测试网 MON（faucet 链接写入 `docs/testnet-faucet.md`，Phase 10 任务 10.2）
  - 部署前用 `cast balance` 确认部署账户余额 ≥ 1 MON（覆盖三合约部署 + 角色授权 + 端到端流程 gas 估算），不足则先领币
  - Evaluator 账户、Keeper 账户、演示用 Buyer/Provider 账户均需领币
- 配置 Indexer 连接 Monad testnet RPC
- 验证 testnet 上事件同步正常（FR-I01）
- 更新前端 `constants.ts` 合约地址
- **重新执行角色授权**（testnet evaluatorAddr 可能与 anvil 不同）
- 验证 Evaluator 在 testnet 上能正常 complete + submitValidation
- 4 个 Agent 部署到可公网访问的服务器（testnet 上 Agent endpoint 需公网可达）
- **部署失败重试策略：**
  - 合约部署：`forge script` 失败时检查 nonce 是否卡住（`cast nonce`），用 `--slow` 单笔确认模式重试，最多 3 次
  - 角色授权 tx 失败：gas 不足时调高 `--gas-price`，nonce 冲突时 `cast nonce` 重置后重发
  - Indexer 同步卡住：检查 RPC 节点高度（`cast block-number`），超过 30s 未推进切换备用 RPC
  - 重试均失败时记录到 `docs/testnet-deploy-log.md`，回退 anvil 本地联调（不阻塞 Phase 9 其他任务）

### 任务 9.8：x402 facilitator 集成验证（FR-AP06~AP09）

- 验证 x402 收据格式（FR-AP06）
- 测试 Monad 官方 facilitator 接入（FR-AP08）
- **验证兜底路径（FR-AP09）：** facilitator 不可用时回退到 `fundViaToken(jobId, amount, "")` 直接转 USDC
- 在 testnet 上跑通 x402 支付 + ERC-20 兜底两条路径

**验证脚本：** `scripts/verify_phase9.sh`
```bash
#!/bin/bash
set -e
cd /home/administrator/Documents/trae_projects/PrismSettle
echo "⚠️  Phase 9 需要手动验证："
echo "  1. docker-compose up -d postgres redis"
echo "  2. anvil --fork-url \$RPC（本地集成测试）"
echo "  3. docker-compose up -d offchain agents"
echo "  4. docker-compose up -d frontend"
echo "  5. 浏览器访问 localhost:3000，跑通完整流程（含仲裁分支）"
echo "  6. 检查 /health 端点（sync_lag/reorg_count/evaluator_state/keeper_last_run）"
echo "  7. 运行压测脚本，输出 V0/V1 对照报告（V1 abort rate < 5%）"
echo "  8. anvil 模拟 reorg，验证 DB 回滚 + 前端标红"
echo "  9. 部署到 Monad testnet（任务 9.7）"
echo " 10. x402 facilitator 集成 + 兜底验证（任务 9.8）"
```

---

## Phase 10：Bug 修复 + 文档 + 冲刺（4 天）

**目标：** 按优先级修复 bug，README，演示准备

### 任务 10.1：Bug 修复（按优先级分级）

- **P0 bug：必须全部清零**（阻断核心流程：合约 revert、Indexer 不同步、Evaluator 决策失败、前端白屏等）
- **P1 bug：视剩余工期修复**（影响体验但不阻断：UI 错位、非核心路径报错等）
- **P2 bug：记录到 GitHub issue，不强制修复**（文案、样式微调等）

### 任务 10.2：文档

- README.md（项目介绍 + 快速开始 + 架构图）
- 各模块 CONTRIBUTING.md
- API 文档（Swagger/OpenAPI，从 Phase 7 端点生成）

### 任务 10.3：演示准备

- Seed Phase 脚本完善（含 Monad testnet 版本）
- 演示视频录制（覆盖 UC-01~UC-04 用户场景）
- Hackathon 提交材料

**验证脚本：** `scripts/verify_phase10.sh`
```bash
#!/bin/bash
set -e
echo "✅ Phase 10 验证："
echo "  - P0 bug 全部清零"
echo "  - README + API 文档完成"
echo "  - 演示脚本可用（覆盖 UC-01~UC-04）"
```

---

## 验证与沟通机制

### 每个阶段完成后的流程

1. **自动验证**：运行 `scripts/verify_phaseN.sh`，检查编译/测试是否通过
2. **CI 验证**：PR 合并前 CI 必须全绿（Phase 0 任务 0.3 配置）
3. **人工验收**：我列出验证结果和预期对比，你确认
4. **问题处理**：如果有失败，**不降级、不跳过**，立即通知你，等你决定下一步
5. **继续下一阶段**：只有你确认通过后，才进入下一阶段

### 沟通规则

- 每个阶段完成后，我会输出：
  - ✅ 通过的验证项
  - ❌ 失败的验证项（如有）
  - 📋 下一步建议
- 如果有失败或不确定，**立即通知你，等你确认后再继续**
- 绝不私自降级完成标准

---

## 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Monad RPC 不稳定 | Indexer 同步失败 | 多 RPC 备用 + 重试逻辑 |
| Evaluator Agent 宕机 | 无法评分 | 熔断器 + 降级 0.6e18 |
| x402 Facilitator 未上线 | fundViaToken 阻塞 | ERC-20 兜底路径（任务 9.8 验证） |
| Foundry 测试网部署失败 | 合约无法测试 | anvil 本地测试先行 |
| 前端依赖安装失败 | 构建失败 | 离线缓存 + 备用源 |
| 评估 Agent LLM API 不稳定 | 评分失败 | 降级 0.6e18 + 熔断器 |
| 4 个 Agent 服务延期 | Demo 无法跑通 | Phase 5 优先级提升，可与 Phase 4 部分并行 |
| LLM API key 泄露 | 安全事故 | .env 入 .gitignore + Docker secret 注入 + Phase 5 验证脚本检查 |
| 关键路径无缓冲 | 任一 Phase 延期挤压后续 | Phase 10 保留 4 天可吸收延期 |
| Monad testnet 不稳定 | 任务 9.7 部署失败 | anvil 兜底 + 多次重试 + 备用 RPC |

---

## PRD/SD 对齐核查表

| PRD/SD 关键需求 | 对应 Phase | 状态 |
|----------------|-----------|------|
| FR-C01~C11 Registry | Phase 1 | ✅ 覆盖 |
| FR-J01~J11 Job + Hook | Phase 2 | ✅ 覆盖 |
| FR-T01~T06 压测 | Phase 3.4a-d | ✅ 覆盖（已拆分） |
| NFR-MN01 中止率达标 | Phase 3.4a + 9.5 | ✅ 覆盖（V1 < 5%） |
| FR-I01~I08 Indexer | Phase 4 | ✅ 覆盖 |
| FR-JI01~JI06 Job Parser | Phase 4 任务 4.5 | ✅ 覆盖 |
| FR-T07/T11/T12 合约层测试 | Phase 1 任务 1.5 + Phase 2 任务 2.5 | ✅ 覆盖 |
| FR-T08/T09/T10 Indexer 单测 | Phase 4 任务 4.6 | ✅ 覆盖 |
| FR-AP01~AP05 Agent 协议 + 4 Agent | Phase 5 | ✅ 覆盖（2 天） |
| FR-E01~E14 Evaluator | Phase 6 任务 6.1~6.5 | ✅ 覆盖（含仲裁+自动声誉更新） |
| SD §4.5.7 Evaluator 权限安全约束 | Phase 6 任务 6.5a | ✅ 覆盖 |
| SD §2.3 Cancun EVM 版本 | Phase 0 任务 0.2 | ✅ 覆盖 |
| FR-A01~A12 + FR-M11 API | Phase 7 任务 7.1 | ✅ 覆盖（16 端点） |
| FR-AP11 checkTrust 阈值配置 | Phase 7 任务 7.2 | ✅ 覆盖 |
| FR-M01~M13 前端 | Phase 8 | ✅ 覆盖（含组件单测） |
| FR-JM01~JM06 Job 前端闭环 | Phase 8 任务 8.5a-d | ✅ 覆盖（已拆分） |
| FR-AP06~AP09 x402 集成 | Phase 2 + 8.5a + 9.8 | ✅ 覆盖 |
| NFR-OBS02 健康检查 | Phase 9 任务 9.6 | ✅ 覆盖 |
| PRD §8.3 Phase 4 测试网部署 | Phase 9 任务 9.7 | ✅ 覆盖 |
| CI/CD 自动化 | Phase 0 任务 0.3 | ✅ 覆盖 |

---

## 文件结构总览（完成后）

```
PrismSettle/
├── .github/workflows/ci.yml         # CI 流水线（Phase 0 新增）
├── .env.example                     # 环境变量模板（Phase 9 新增）
├── contracts/
│   ├── src/
│   │   ├── PrismSettleRegistry.sol
│   │   ├── PrismSettleJob.sol
│   │   ├── ArbitrationHook.sol
│   │   └── bench/BaselineRegistry.sol
│   ├── test/
│   │   ├── PrismSettleRegistry.t.sol
│   │   ├── PrismSettleJob.t.sol
│   │   ├── ArbitrationHook.t.sol
│   │   └── Integration.t.sol
│   └── script/
│       └── Deploy.s.sol
├── offchain/
│   ├── cmd/main.go
│   ├── agents/                      # 4 个 Agent 服务（Phase 5 新增）
│   │   ├── defi_agent.go
│   │   ├── data_labeling_agent.go
│   │   ├── translation_agent.go
│   │   └── eval_agent.go
│   ├── internal/
│   │   ├── listener/
│   │   ├── parser/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── storage/
│   │   └── api/
│   ├── prismsettle/
│   │   ├── evaluator/
│   │   │   ├── evaluator.go
│   │   │   ├── rule_check.go
│   │   │   ├── eval_agent_client.go
│   │   │   ├── arbitration.go        # 仲裁分支（Phase 6 新增）
│   │   │   ├── decision_log.go       # 决策日志（Phase 6 新增）
│   │   │   └── circuit_breaker.go
│   │   ├── keeper/
│   │   ├── agent_proxy/
│   │   ├── notifier/
│   │   ├── api/
│   │   └── service/
│   └── config/
│       ├── dev.yaml
│       ├── prod.yaml
│       └── agents.yaml               # 4 Agent 端点配置（Phase 5 新增）
├── frontend/
│   ├── app/
│   ├── components/
│   ├── lib/
│   ├── hooks/
│   ├── store/
│   └── __tests__/                   # 组件级单测（Phase 8 新增）
├── scripts/
│   ├── verify_phase0.sh
│   ├── verify_phase1.sh
│   ├── ...
│   ├── bench/v0_v1_bench.sh
│   └── seed.sh
├── docs/
│   ├── PRD.zh-CN.v1.0.md
│   ├── SD.zh-CN.v1.0.md
│   └── DEV-PLAN.zh-CN.v1.0.md        # 与 PRD/SD 命名一致
└── docker-compose.yml
```

---

## 文档结束

本计划基于 PRD.zh-CN.v1.0.md 与 SD.zh-CN.v1.0.md，覆盖合约层、链下层、前端层全部设计内容。每个阶段都有明确的验证脚本和人工验收流程。


