# PrismSettle 全流程验证指南

> 验证时间：2026-08-06 | 网络：Monad Testnet (chainId=10143)
> 部署账户：`0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C`
> 更新：2026-08-06 — complete 新签名（Buyer 评分）+ source=3 变更点

> ⚠️ 重要前提：本指南对应**当前工作区合约代码**（`complete(uint256,uint96)` + Buyer 评分 source=3）。链上部署的合约版本如不包含这些函数，请先重新部署（见 `scripts/deploy_monad_testnet.sh`），否则下述 complete 步骤会 revert。

---

## 前置条件

### 1.1 钱包准备

使用 **Rabby Wallet** 或浏览器扩展钱包，切换到 **Monad Testnet**：

| 配置项 | 值 |
|--------|-----|
| RPC URL | `https://testnet-rpc.monad.xyz` |
| Chain ID | `10143` |
| 浏览器 | `https://testnet.monadexplorer.com` |

### 1.2 领取测试币

| 代币 | 用途 | 领取方式 |
|------|------|---------|
| MON | Gas 费 | https://faucet.monad.xyz/ |
| USDC | 托管 Job 资金 | 以下命令 mint |

```bash
# USDC 合约地址（MockERC20，decimals=18）
USDC=0x252e44550f8B9997901e5540FC0E1dA52Ab099C6

# 给你的钱包 mint 1000 USDC（decimals=18，所以 1000e18 = 1000000000000000000000）
# 替换 YOUR_WALLET 为你的钱包地址
cast send $USDC "mint(address,uint256)" YOUR_WALLET 1000000000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key YOUR_PRIVATE_KEY
```

### 1.3 验证合约状态

```bash
# 检查 4 个官方 Agent 的声誉
REGISTRY=0xA82937ad81e8aB775c9B32F363CE5E8564207739
cast call $REGISTRY "getScore(uint256)(uint256)" 0x1111 --rpc-url https://testnet-rpc.monad.xyz
# 预期: 700000000000000000 (0.7e18)

# 检查 Facilitator 地址
JOB=0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB
cast call $JOB "facilitator()(address)" --rpc-url https://testnet-rpc.monad.xyz
# 预期: 0x7f6a2850669202519f0FE8aa912451238820Db86
```

---

## 2. 启动项目

### 2.1 确认服务状态

项目已通过 docker-compose 启动，访问以下地址验证：

| 服务 | 地址 | 预期状态 |
|------|------|----------|
| 前端 Dashboard | http://localhost:3000 | 页面正常加载 |
| Offchain API | http://localhost:9527/health | 返回 JSON 状态 |
| Postgres | localhost:5433 | 数据库连接正常 |
| Redis | localhost:6380 | 缓存服务正常 |

### 2.2 验证 Offchain API

```bash
# 健康检查
curl http://localhost:9527/health
# 预期: {"evaluator_state":"running","status":"degraded","sync_lag":...}

# 查询 Agent 列表（需要 API Token）
curl -s -H "X-API-TOKEN: dev-token" \
  "http://localhost:9527/api/v1/prismsettle/agents?chainName=monad_testnet&page=1&size=20"
# 预期: 返回 8 个 Agent 列表
```

---

## 3. 创建 Job（Buyer 操作）

### 3.1 通过前端创建

1. 打开 http://localhost:3000
2. 点击右上角 **Connect Wallet**，连接你的钱包
3. 导航到 **Jobs** → **Create Job**
4. 填写表单：
   - **Description**: 智能合约安全审计 — 检查重入漏洞
   - **Min Provider Reputation**: `0.5`（最低 0.5e18）
   - **Amount**: `100 USDC`
5. 点击 **Create Job**，钱包签名确认交易
6. 等待交易确认，前端自动刷新列表

### 3.2 通过命令行（备选）

```bash
# 创建 Job（签名：createJob(uint256 agentId, uint256 parentJobId, uint64 deadline, address hook, uint96 minProviderReputation)）
JOB=0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB
REGISTRY=0xA82937ad81e8aB775c9B32F363CE5E8564207739
HOOK=0x740c2969e537706A4f4757166e5eBEeD0E4DAD15
BUYER_KEY=5138c7d2e167ec1039616451b01a5b2a5644c138d271843a84961ab2f6c9227b

# agentId=0x1111, parentJobId=0, deadline=1小时后, hook=ArbitrationHook, minProviderReputation=0.5e18
DEADLINE=$(( $(date +%s) + 3600 ))
cast send $JOB "createJob(uint256,uint256,uint64,address,uint96)" \
  0x1111 0 $DEADLINE $HOOK 500000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $BUYER_KEY

# 查看 Job 事件（获取 jobId）
cast logs --rpc-url https://testnet-rpc.monad.xyz \
  --address $JOB \
  --event "JobCreated(uint256,uint256,address,uint64,address,uint96)" \
  --from-block latest
```

---

## 4. 托管资金（Buyer 操作）

### 4.1 先授权 USDC

```bash
USDC=0x252e44550f8B9997901e5540FC0E1dA52Ab099C6
JOB=0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB

# 授权 Job 合约使用 USDC（100 USDC @ 18 decimals）
cast send $USDC "approve(address,uint256)" $JOB 100000000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $BUYER_KEY
```

### 4.2 托管资金

```bash
# 调用 fundViaToken(jobId, amount, x402Receipt)
# 注意：x402Receipt 为空（0x）时走 ERC-20 兜底路径；
#      有 receipt 时走 x402 路径（amount 传 0，由 receipt 决定金额）
# 命令行演示走 ERC-20 兜底路径

# 先获取 jobId（从上一步的事件日志中获取）
JOB_ID=1  # 替换为实际 jobId

cast send $JOB "fundViaToken(uint256,uint256,bytes)" \
  $JOB_ID 100000000000000000000 0x \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $BUYER_KEY
```

---

## 5. Provider 抢单

### 5.1 验证 Provider 声誉

```bash
# 检查各 Agent 的声誉
echo "Senior Auditor (0x3333):"
cast call $REGISTRY "getScore(uint256)(uint256)" 0x3333 --rpc-url https://testnet-rpc.monad.xyz
# 预期: 0.9e18

echo "Junior Auditor (0x7777):"
cast call $REGISTRY "getScore(uint256)(uint256)" 0x7777 --rpc-url https://testnet-rpc.monad.xyz
# 预期: 0.6e18

echo "Rookie Auditor (0x8888):"
cast call $REGISTRY "getScore(uint256)(uint256)" 0x8888 --rpc-url https://testnet-rpc.monad.xyz
# 预期: 0.3e18
```

### 5.2 抢单

```bash
# 使用 Provider 私钥抢单
PROVIDER_KEY=3336f7ede5e02528e96cb43308c8b784c512889d0788cf6215a1e8bde827d65d

cast send $JOB "grabJob(uint256,uint256)" $JOB_ID 0x3333 \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $PROVIDER_KEY
```

---

## 6. 提交交付物（Provider 操作）

### 6.1 提交审计报告

```bash
# 提交交付物（签名：submit(jobId, deliverableHash, proofHash)）
# 注意：只有当前 Job 的 Provider 才能调用 submit
DELIVERABLE_HASH=0xabc123...  # 交付物哈希
PROOF_HASH=0xdef456...        # 执行证明哈希

cast send $JOB "submit(uint256,bytes32,bytes32)" $JOB_ID $DELIVERABLE_HASH $PROOF_HASH \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $PROVIDER_KEY
```

---

## 7. Buyer 评分 + complete 新签名（变更点）

### 7.0 变更说明（2026-08-06）

- **complete 新签名：** `complete(uint256 jobId, uint96 score)` — 原无参版本作废
- **调用方变更：** 原来由 Evaluator（`COMMERCE_EVALUATOR_ROLE`）自动放款；现在**仅 Buyer 可调用**（`require(msg.sender == j.buyer)`），有 pending dispute 时 revert
- **score 参数：** Buyer 给 Provider 的评分，固定点 `0..1e18`（如 `900000000000000000` = 0.9e18）
- **Buyer 评分（source=3）：** complete 内部调用 `Registry.submitValidation(providerAgentId, score, proofHash, jobId, 3)`，把 Buyer 满意度写入 Provider 声誉（每单一次，不受 epoch 配额限制；不参与加权平均，走**双因子**：完成奖励 +0.005e18/单 + 评分微调 (score−0.5e18)×0.05，各自 epoch 封顶）
- **前置条件：** Registry 需通过 `setTrustedJob(job, true)` 把 Job 合约加入 `trustedJobs` 白名单，否则 complete 会 revert "not trusted job"
- **Evaluator 角色变化：** 主路径不再自动放款；仅在争议流程中介入（`resolveDispute` + `setAggregatedScore`）

### 7.1 Buyer 完成 Job + 评分

```bash
# Buyer 调用 complete(jobId, score)；无争议时 escrow 全额放款给 Provider（无扣费）
cast send $JOB "complete(uint256,uint96)" $JOB_ID 900000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $BUYER_KEY
```

### 7.2 查看链上状态

```bash
# 检查 Job 状态
cast call $JOB "getJobState(uint256)(uint8)" $JOB_ID \
  --rpc-url https://testnet-rpc.monad.xyz
# 0=Created, 1=Funded, 2=Assigned, 3=Submitted, 4=DisputeResolved, 5=Completed, 6=Refunded

# complete 成功后状态应为 5 (Completed)
```

### 7.3 查看前端 Dashboard

1. 打开 http://localhost:3000/dashboard
2. 查看 **Shard Activity** 热力图
3. 查看 **Agent 排行榜**
4. 查看 **实时事件流**

### 7.4 查看 Offchain 数据

```bash
# 查询 Job 事件
curl -s -H "X-API-TOKEN: dev-token" \
  "http://localhost:9527/api/v1/prismsettle/events?chainName=monad_testnet&page=1&size=10"

# 查询 Agent 声誉历史
curl -s -H "X-API-TOKEN: dev-token" \
  "http://localhost:9527/api/v1/prismsettle/reputation/history?chainName=monad_testnet&agentId=0x3333"

# 查询 Trust 预检查
curl -s -H "X-API-TOKEN: dev-token" \
  "http://localhost:9527/api/v1/prismsettle/trust?chainName=monad_testnet&agentId=0x3333"
```

---

## 8. 验证 x402 支付路径

### 8.1 检查 Facilitator 状态

```bash
# 验证 Facilitator HTTP 服务在线
curl -s https://x402-facilitator.molandak.org/supported | python3 -m json.tool

# 验证 Job 合约的 Facilitator 地址
cast call $JOB "facilitator()(address)" --rpc-url https://testnet-rpc.monad.xyz
# 预期: 0x7f6a2850669202519f0FE8aa912451238820Db86
```

### 8.2 前端验证

1. 打开 http://localhost:3000/jobs/new
2. 填写 Job 信息
3. 在资金面板中应看到 **x402** 标签
4. 点击创建，前端会尝试通过 x402 路径托管资金
5. 如果 x402 失败，自动降级为 ERC-20 approve + transferFrom

---

## 9. 验证推荐流程

### 9.1 快速演示流（命令行）

```bash
# 1. 创建 Job（agentId=0x1111, hook=ArbitrationHook, minProviderReputation=0.2e18）
DEADLINE=$(( $(date +%s) + 3600 ))
TX=$(cast send $JOB "createJob(uint256,uint256,uint64,address,uint96)" \
  0x1111 0 $DEADLINE $HOOK 200000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz \
  --private-key $BUYER_KEY --json)
JOB_ID=$(echo "$TX" | jq -r '.logs[0].topics[2]')  # JobCreated indexed jobId

# 2. 托管资金（ERC-20 兜底路径，100 USDC @ 18 decimals）
USDC=0x252e44550f8B9997901e5540FC0E1dA52Ab099C6
cast send $USDC "approve(address,uint256)" $JOB 100000000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz --private-key $BUYER_KEY
cast send $JOB "fundViaToken(uint256,uint256,bytes)" $JOB_ID 100000000000000000000 0x \
  --rpc-url https://testnet-rpc.monad.xyz --private-key $BUYER_KEY

# 3. 抢单（Senior Auditor 0x3333 声誉 0.9e18，应成功）
cast send $JOB "grabJob(uint256,uint256)" $JOB_ID 0x3333 \
  --rpc-url https://testnet-rpc.monad.xyz --private-key $PROVIDER_KEY

# 4. 提交（deliverableHash + proofHash）
cast send $JOB "submit(uint256,bytes32,bytes32)" $JOB_ID 0xabc123 0xdef456 \
  --rpc-url https://testnet-rpc.monad.xyz --private-key $PROVIDER_KEY

# 5. Buyer 完成 + 评分（0.9e18）
cast send $JOB "complete(uint256,uint96)" $JOB_ID 900000000000000000 \
  --rpc-url https://testnet-rpc.monad.xyz --private-key $BUYER_KEY

# 6. 查看状态（应为 5 = Completed）
cast call $JOB "getJobState(uint256)(uint8)" $JOB_ID \
  --rpc-url https://testnet-rpc.monad.xyz
```

---

## 10. 故障排查

### 10.1 常见问题

| 问题 | 原因 | 解决方法 |
|------|------|---------|
| 前端显示 404 | API 路由错误 | 确认 offchain 服务正在运行 |
| 交易卡住 | Gas 不足 | 检查钱包余额，领取 MON |
| Agent 显示 score=0 | 索引未完成 | 等待 offchain 同步完成 |
| x402 路径失败 | Facilitator 连接问题 | 自动降级为 ERC-20 路径 |
| 抢单失败 | 声誉不足 | 确认 minProviderReputation 设置正确 |
| complete revert "not trusted job" | Registry 未把 Job 加入 `trustedJobs` 白名单 | 部署后调用 `registry.setTrustedJob(job, true)` |
| complete revert 空数据 | 链上合约是旧版（无 `complete(uint256,uint96)`） | 用当前工作区代码重新部署合约 |

### 10.2 查看日志

```bash
# Offchain 日志
docker compose logs -f offchain

# 前端日志
docker compose logs -f frontend

# Agent 日志
docker compose logs -f agent-auditor-senior
```

### 10.3 验证合约地址

```bash
# 确认当前部署的合约地址
echo "Token: 0x252e44550f8B9997901e5540FC0E1dA52Ab099C6"
echo "Registry: 0xA82937ad81e8aB775c9B32F363CE5E8564207739"
echo "Hook: 0x740c2969e537706A4f4757166e5eBEeD0E4DAD15"
echo "Job: 0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB"
echo "Facilitator: 0x7f6a2850669202519f0FE8aa912451238820Db86"
```

---

## 附录：合约地址速查

| 合约 | 地址 | 浏览器 |
|------|------|--------|
| MockERC20 (USDC) | `0x252e44550f8B9997901e5540FC0E1dA52Ab099C6` | [查看](https://testnet.monadexplorer.com/address/0x252e44550f8B9997901e5540FC0E1dA52Ab099C6) |
| PrismSettleRegistry | `0xA82937ad81e8aB775c9B32F363CE5E8564207739` | [查看](https://testnet.monadexplorer.com/address/0xA82937ad81e8aB775c9B32F363CE5E8564207739) |
| ArbitrationHook | `0x740c2969e537706A4f4757166e5eBEeD0E4DAD15` | [查看](https://testnet.monadexplorer.com/address/0x740c2969e537706A4f4757166e5eBEeD0E4DAD15) |
| PrismSettleJob | `0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB` | [查看](https://testnet.monadexplorer.com/address/0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB) |
| x402 Facilitator | `0x7f6a2850669202519f0FE8aa912451238820Db86` | [查看](https://testnet.monadexplorer.com/address/0x7f6a2850669202519f0FE8aa912451238820Db86) |