# PrismSettle 项目现状评审与操作验证

> 生成：2026-07-07 | 更新：2026-07-08 前端 wiring 完成 | 范围：Phase 0-10 全量代码 + 部署就绪度 | 方法：验证脚本 + go/forge test + tsc + 静态审计

## 0. 新增：前端真实链上操作 wiring（已完成）

3 个写操作页面已从 mock 升级为真实链上调用（wagmi + viem）：

| 页面 | 路径 | 合约调用 | 状态 |
|---|---|---|---|
| 注册 Agent | `/agents/register` | `Registry.registerAgent(agentId, metadata)` | ✅ wired |
| 创建+注资 Job | `/jobs/new` | `Job.createJob` → `ERC20.approve` → `Job.fundViaToken` | ✅ wired |
| 验证者质押 | `/validator` | `Registry.stake` / `unstake` / `withdrawUnstaked` | ✅ wired |

**新增文件**：
- `frontend/lib/abi/PrismSettleRegistry.json` — Registry ABI
- `frontend/lib/abi/PrismSettleJob.json` — Job ABI
- `frontend/lib/abi/ArbitrationHook.json` — Hook ABI
- `frontend/lib/abi/MockERC20.json` — ERC20 ABI
- `frontend/lib/contracts.ts` — 合约地址 + ABI 集中配置

**改写文件**：
- `frontend/components/agent/AgentRegisterForm.tsx` — useWriteContract
- `frontend/app/jobs/new/page.tsx` — createJob + approve + fundViaToken 三步
- `frontend/app/validator/page.tsx` — stake/unstake/withdraw + 链上余额读取

**配置更新**：
- `frontend/tsconfig.json` — target ES2017 → ES2020（BigInt 支持）
- `.env.example` — 新增 5 个 NEXT_PUBLIC_ 合约地址变量
- `docker-compose.yml` — frontend 服务注入合约地址环境变量

**验证**：`npx tsc --noEmit` 通过，0 错误。

## 一、各 Phase 完成度

| Phase | 内容 | 状态 | 验证 |
|---|---|---|---|
| 0 | 项目初始化 + CI | ✅ | ✅ |
| 1 | Registry 合约 | ✅ | ✅ forge test |
| 2 | Job + Hook 合约 | ✅ | ✅ forge test |
| 3 | 集成测试 + 压测 | ✅ | ✅ 106/106 tests |
| 4 | Indexer 核心 | ✅ | ✅ go test |
| 5 | 4 个 Agent 服务 | ✅ | ✅ |
| 6 | Evaluator + Keeper | ✅ | ✅ go test |
| 7 | API + 前端基建 | ✅ | ✅ 16 端点 |
| 8 | 前端核心页面 | ✅ | ✅ 79/79 checks |
| 9 | 集成 + 压测 + 测试网 | ⚠️ 代码完成 | ✅ 脚本通过，6 项手动任务待执行 |
| 10 | Bug + 文档 + 冲刺 | ❌ 未开始 | - |

**测试快照**：合约 106/106 ✅ | offchain go test 全绿 ✅ | frontend 21 vitest ✅ | phase8 79/79 ✅ | phase9 9/9 ✅

**代码审计**：刚修复 P0（evm_listener.go uint 下溢）+ P1（event_source.go SetString 未校验）。其余 slice/topics 访问、context defer、goroutine 退出、errors.Is 均正确。

---

## 二、Phase 9 剩余 6 项手动任务

### 9.1 Docker 全栈启动

**前置**：Docker Engine ≥ 24.0 + Compose v2

```bash
cp .env.example .env && nano .env
docker compose --profile dev up -d
docker compose ps
```

**验证**：
- [ ] 8 容器 Up (healthy)
- [ ] `docker compose logs offchain 2>&1 | grep -iE 'fatal|panic'` 空
- [ ] `curl -s localhost:9527/health | jq .status` = `"ok"`
- [ ] `curl -sI localhost:3000 | head -1` = HTTP 200
- [ ] 4 agent 健康端点（9101-9104）可访问

**回滚**：`docker compose down -v`

### 9.3 端到端全链路（前端真实操作）

**前置**：9.7 完成（合约已部署到 Monad testnet，地址已写入 `.env`）

#### 第一步：MetaMask 配置

1. 安装 MetaMask 浏览器扩展
2. 添加 Monad Testnet 网络：
   - 网络名称：`Monad Testnet`
   - RPC URL：`https://testnet-rpc.monad.xyz`
   - Chain ID：`10143`
   - 货币符号：`MON`
   - 区块浏览器：`https://testnet.monadexplorer.com`
3. 在 https://faucet.monad.xyz/ 领币（每个地址 ≥ 1 MON）
4. 导入测试账户（可选，用于 provider 角色）

#### 第二步：启动前端

```bash
cd /home/administrator/Documents/trae_projects/PrismSettle
docker compose up -d frontend offchain postgres redis
# 或本地 dev：cd frontend && npm run dev
```

打开 http://localhost:3000

#### 第三步：前端操作流程

| 步骤 | 页面 | 操作 | 预期结果 |
|---|---|---|---|
| 1 | 右上角 | 点击 "Connect Wallet" → 选 MetaMask → 切到 Monad Testnet | 钱包地址显示 |
| 2 | `/agents/register` | 输入 Agent ID (如 `42`) + Endpoint URL → 点 "Register Agent" → MetaMask 确认 | 链上 AgentRegistered 事件 |
| 3 | `/validator` | 输入 5.0 ETH → 点 "Stake" → MetaMask 确认 | 链上 Staked 事件，余额显示更新 |
| 4 | `/jobs/new` | 输入 Agent ID + Amount + Deadline → TrustGate 检查通过 → 点 "Create & Fund Job" | 三步链上交易：createJob → approve → fundViaToken |
| 5 | `/jobs/[jobId]` | 查看状态 Timeline + 资金流向图 | Job 状态 Funded，资金流向图显示 |
| 6 | (provider) | 用 provider 账户在 MetaMask 中调用 `Job.assign(jobId, providerAddr)` | 可用 cast 或 Remix |
| 7 | (provider) | 调用 `Job.submit(jobId, deliverableHash, proofHash)` | 链上 Submitted 事件 |
| 8 | 自动 | Evaluator 检测 Submitted → 自动 complete + submitValidation | Job 状态 Completed |
| 9 | 自动 | Keeper 触发 aggregateEpoch | provider 声誉分更新 |
| 10 | `/jobs/[jobId]` | buyer 点 "Claim Refund"（如超时）或等待自动完成 | 退款到账或资金转 provider |

#### 仲裁分支测试

| 步骤 | 操作 | 预期 |
|---|---|---|
| 1 | 正常流程到 Submitted | Job 状态 Submitted |
| 2 | buyer 在前端或 cast 调用 `Hook.dispute(jobId, reasonHash)` | 链上 Disputed 事件 |
| 3 | Evaluator 调用 `Hook.resolveDispute(jobId, 1)` (buyer 胜) | 链上 DisputeResolved |
| 4 | buyer 调用 `Job.claimRefund(jobId)` | 立即退款（无需等 deadline） |
| 5 | 查看 `decision_logs` 表 | source=2（仲裁路径）记录 |

**验证清单**：
- [ ] MetaMask 连接成功，显示 Monad Testnet
- [ ] 注册 Agent 交易上链确认
- [ ] Stake 交易上链，余额实时更新
- [ ] Create & Fund Job 三步交易全部确认
- [ ] `/jobs/[jobId]` 显示完整 Timeline
- [ ] `decision_logs` 表有 source=1（main path）记录
- [ ] 仲裁后 `decision_logs` 表有 source=2（arb path）记录
- [ ] provider 声誉分在 aggregateEpoch 后变化

### 9.4 Reorg 模拟

**前置**：9.3 完成

```bash
SNAP=$(cast rpc --rpc-url http://localhost:8545 evm_snapshot)
# 推进几个区块让 indexer 同步
cast rpc --rpc-url http://localhost:8545 evm_revert $SNAP
docker compose logs offchain 2>&1 | grep -i reorg
```

**验证**：
- [ ] 日志出现 `reorg detected` + `rolling back events`
- [ ] `reorg_events` 表有新记录，`rolled_back_rows` 非 0
- [ ] `chain_events` 受影响区块事件已删除
- [ ] `decision_logs` 相关记录 `valid` = false
- [ ] `/health` `reorg_count` 递增
- [ ] 前端 ReorgAwareFeed 标红 + 删除线

### 9.5 500 并发压测

**前置**：9.1 完成

```bash
# 安装 k6，编写 scripts/loadtest.js（POST /api/v1/prismsettle/jobs，500 VUs，60s）
k6 run --vus 500 --duration 60s scripts/loadtest.js
# 结果写入 perf_results 表
```

**验证**：
- [ ] **V1 abort rate < 5%**（NFR-MN01）
- [ ] `perf_results` 表有 V0 + V1 记录
- [ ] 前端 `/perf` `source` = `"benchmark"`（非 `"mock"`）
- [ ] 报告含三项约束声明（FR-T06）

### 9.7 Monad Testnet 部署（前端操作的前置条件）

**前置**：部署钱包（AI 不持私钥）+ 三账户在 https://faucet.monad.xyz/ 领币 ≥ 1 MON

```bash
cast wallet new  # 生成 Deployer / Evaluator / Keeper
cast balance <ADDR> --rpc-url https://testnet-rpc.monad.xyz
chmod +x scripts/deploy_monad_testnet.sh && ./scripts/deploy_monad_testnet.sh
# 验证
cast call <REGISTRY> "hasEvaluatorRole(address) returns (bool)" <EVALUATOR_ADDR> \
  --rpc-url https://testnet-rpc.monad.xyz  # 期望 true
cast call <REGISTRY> "agentCount() returns (uint256)" \
  --rpc-url https://testnet-rpc.monad.xyz  # 期望 4
```

**关键：部署后必须把合约地址写入 `.env`**

```bash
# 从部署脚本输出中复制 3 个合约地址 + paymentToken 地址
nano .env
# 填入：
#   NEXT_PUBLIC_REGISTRY_ADDRESS=0x...
#   NEXT_PUBLIC_JOB_CONTRACT_ADDRESS=0x...
#   NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS=0x...
#   NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS=0x...
# 同时填入 offchain/config/prod.yaml 的合约地址 + evaluator 私钥
docker compose up -d
```

**验证**：
- [ ] 3 合约在 https://testnet.monadexplorer.com/ 可查
- [ ] Evaluator 持 `REGISTRY_EVALUATOR_ROLE`
- [ ] 4 agent seed
- [ ] offchain 日志 3 listener 连接 testnet
- [ ] `sync_lag` < 64，`evaluator_state` = `"running"`
- [ ] 前端 `/agents/register` 页面不再显示 "Contract addresses not configured"
- [ ] 前端 `/jobs/new` 页面 FundingPathBadge 正确显示路径

**失败重试**：nonce 卡住用 `--slow`，gas 不足调 `--gas-price`，RPC 卡切换备用，全失败回退 anvil。

### 9.8 x402 Facilitator 集成

**前置**：9.7 完成

**验证**：
- [ ] x402 路径：fundViaX402 成功，facilitator 托管
- [ ] ERC-20 兜底：fundViaToken 成功，资金直接进 Job 合约
- [ ] 前端 `FundingPathBadge` 正确显示路径
- [ ] `FundFlowChart` 标注双路径

---

## 三、Phase 10 任务（Phase 9 完成后）

- **10.1 Bug 修复**：P0 必清零 / P1 视工期 / P2 记 issue
- **10.2 文档**：README + API 文档（Swagger）+ CONTRIBUTING
- **10.3 演示**：Seed 脚本 + 视频（UC-01~UC-04）+ Hackathon 材料

---

## 四、立即需要你完成的事项

### 🔴 P0 立即

1. **Git 首次 commit**：当前仓库**无任何 commit**，所有文件 untracked，无法回滚。
   ```bash
   git add . && git commit -m "Phase 0-9: full-stack implementation complete"
   ```
2. **执行 9.1 Docker 启动**：需本机 Docker daemon，AI 无法操作。

### 🟡 P1 本周内

3. **9.7 Monad testnet 部署**：需你提供钱包私钥（本机终端输入）。**这是前端操作的前置条件。**
4. **9.3 前端端到端**：9.7 完成后，在浏览器真实操作全流程（注册 Agent → 质押 → 创建 Job → 注资 → 完成）。
5. **9.4 Reorg 测试**：仅 anvil 可模拟，需切换到 dev profile。
6. **9.5 压测**：依赖 9.1，验证 NFR-MN01。

### 🟢 P2 按需

7. **9.8 x402 集成**：需 x402 facilitator 地址。完整 16 章清单见 [phase9-deploy-checklist.md](phase9-deploy-checklist.md)。
8. **Phase 10**：9.x 全部完成后启动。

---

## 五、关键文件索引

| 文件 | 用途 |
|---|---|
| [DEV-PLAN.zh-CN.v1.0.md](DEV-PLAN.zh-CN.v1.0.md) | 30 天开发计划 |
| [phase9-deploy-checklist.md](phase9-deploy-checklist.md) | Phase 9 上线 16 章清单 |
| [PROJECT-STATUS-REVIEW.md](PROJECT-STATUS-REVIEW.md) | 本文档 |
| [../docker-compose.yml](../docker-compose.yml) | 全栈编排 |
| [../.env.example](../.env.example) | 环境变量模板 |
| [../offchain/config/prod.example.yaml](../offchain/config/prod.example.yaml) | 生产配置模板 |
| [../scripts/verify_phase8.sh](../scripts/verify_phase8.sh) | Phase 8 验证 |
| [../scripts/verify_phase9.sh](../scripts/verify_phase9.sh) | Phase 9 验证 |
| [../scripts/deploy_monad_testnet.sh](../scripts/deploy_monad_testnet.sh) | testnet 部署脚本 |

---

## 六、验证命令速查

```bash
cd /home/administrator/Documents/trae_projects/PrismSettle
cd contracts && forge build && forge test && cd ..
cd offchain && go build ./... && go vet ./... && go test -count=1 ./... && cd ..
cd frontend && npm run build && npx vitest run && cd ..
scripts/verify_phase8.sh
scripts/verify_phase9.sh
docker compose --profile dev up -d && docker compose ps
curl -s http://localhost:9527/health | jq .
```

---

## 七、风险与建议

### 最大风险

1. **Git 无 commit**：所有工作未版本化。**建议立即 git commit**。
2. **Monad testnet 部署依赖人工**：AI 不能持私钥。
3. **压测脚本未提供**：仓库未含 `scripts/loadtest.js`，需编写。

### 建议执行顺序

```
git commit（立即）
  ↓
9.1 Docker 启动（本机）
  ↓
9.7 Monad testnet 部署（需钱包，部署后地址写入 .env）
  ↓
9.3 前端端到端（浏览器真实操作：注册 Agent → 质押 → 创建 Job → 完成）
  ↓
9.4 Reorg 测试（切 anvil dev profile）
  ↓
9.5 压测（验证 NFR-MN01）
  ↓
9.8 x402 集成
  ↓
Phase 10（文档 + 演示）
```

**关键变化**：由于前端 wiring 已完成，9.3 现在是浏览器真实操作流程（不再是脚本），9.7 是其前置条件。

### AI 可协助

- 编写压测脚本（k6/wrk）
- 编写端到端/reorg 自动化脚本
- 修复手动验证发现的 bug
- 撰写 README/API 文档

### AI 不可协助

- 持有/传输私钥
- 操作 Docker daemon
- faucet 领币
- 访问 Monad explorer
- 录制演示视频
