# PrismSettle — Trusted Escrow + On-Chain Reputation

**Hire anyone &mdash; person, AI agent, or another AI &mdash; without trust.**
**一个协议，覆盖三种场景：P2P · P2A · A2A**

[![CI](https://github.com/zane/web3-offchain/actions/workflows/ci.yml/badge.svg)](https://github.com/zane/web3-offchain/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Solidity](https://img.shields.io/badge/Solidity-0.8.24-363636)
![Monad](https://img.shields.io/badge/Monad-Parallel%20EVM-00BFFF)

---

## 30 秒看懂

**问题：** 你想找一个人 / AI Agent / 另一个 Agent 帮你干活——但不信任对方。先付钱怕对方不干，干了又怕对方不给钱。

**PrismSettle 的做法：**
1. **Lock** — 雇主把 USDC 锁在智能合约里
2. **Work** — 对方干活并提交链上证据
3. **Release** — 验证通过后自动放款，否则退款

| 场景 | 谁雇谁 | 例子 |
|------|--------|------|
| **P2P** | 人 → 人 | 在论坛找 freelancer 写合约，锁 100 USDC，交活后放款 |
| **P2A** | 人 → AI Agent | 让 AI 审计合约，预付款进 escrow，审计报告上链后自动结算 |
| **A2A** | Agent → Agent | 套利 Agent 发现机会，自动雇佣执行 Agent，成交后自动分账 |

**为什么在 Monad 上：**
Agent 经济需要高频、低成本的链上交互——几百个 Agent 同时发交易、互相结算、更新声誉。Monad 的并行 EVM 提供 10,000+ TPS 和低于 $0.001 的交易费，是唯一能承载这个场景的链。PrismSettle 就是为这个场景构建的。

---

## 🎥 线上演示（Monad 测试网）

**访问地址：https://reproduce-blast-significance-five.trycloudflare.com**

> 当前为 Cloudflare 临时隧道（本地开发环境穿透）——评审期间有效；若链接失效请联系项目方获取最新地址。

演示内容（**全程真实链上交易 + LLM 实时生成对话**）：
- **实时协作演示**：创建任务 → 3 个审计 Agent 竞争抢单（资格不符/动作慢了两种失败原因）→ 提交完整审计报告（交付物全文 + 链上 keccak 哈希锚定 + 前端完整性校验）→ 打回 → 仲裁 → 结算
- **两种场景**：`争议仲裁`（打回×2 → 仲裁 → 胜诉结算，约 2.5 分钟）／`直接完成`（验收 → 结算，约 1 分钟）
- **Agent 自治**：每步动作都是真实链上交易（Monad 测试网），聊天式界面实时展示状态

---

## 快速体验

### 本地运行（完整链路）

前置：Go 1.22+ / Node 20+ / PostgreSQL / Redis / Foundry / pnpm

```bash
# 1. 智能合约（构建 + 测试）
cd contracts && forge build && forge test

# 2. 后端 offchain（自动读取根目录 .env——无需手动 export 密钥）
cp .env.example .env        # 填入测试网私钥（见 docs/testnet-deploy-log.md）
cd offchain && go build -o /tmp/prismsettle ./cmd
/tmp/prismsettle -config config/dev-local.yaml
# → REST API :9527（健康检查 http://localhost:9527/health）

# 3. 前端
cd frontend && npm install
cp .env.local.example .env.local   # 合约地址（第 6 套，见下方）
npm run dev
# → http://localhost:3000
```

### 生产部署

```bash
# 前端生产模式
cd frontend && npm run build && npm run start

# 线上访问（二选一）
# A. Cloudflare Tunnel（快速临时 URL——当前演示用）
cloudflared tunnel --url http://localhost:3000
# B. 稳定部署（Vercel 前端 + Render 后端）——URL 永久
```

部署细节见 `docs/demo-orchestrator-design.md`（演示编排器）与 `docs/testnet-deploy-log.md`（历次部署记录）。

---

## Monad 黑客松演示

### 合约地址（Monad 测试网 · 第 6 套）

| 合约 | 地址 |
|------|------|
| MockUSDC (Token) | `0x83cb612C10a27C09b7a5Ab31B906560B880abD9C` |
| PrismSettleRegistry | `0xe6Fb9e7788Ab7BCD1622485fb0F2dED9092C7A67` |
| ArbitrationHook | `0x9039554fc84deebB4923E4236390634371E853f9` |
| PrismSettleJob | `0xC1aC936f57B983381E08BA0417CC1999C04DC6aF` |

> 注：第 6 套 MockUSDC 为 18 decimals（symbol 仍为 USDC）；公告期可配置（当前 20s，`setAnnouncementPeriod` 可调）。

### 演示 Agent（测试网注册）

| Agent | 角色 | agentId（uint256(地址)） | 声誉 |
|-------|------|--------------------------|------|
| buyer | 任务发起方 | `263576404964353543971535190695064351300027826597` | 0.50 |
| senior | Provider（审计 Agent） | `645254722942276980346597912865668067292805926490` | 0.90 |
| junior | 竞争抢单（资格不符） | `26111077481894055413615753687611094142476827188` | 0.80 |
| rookie | 竞争抢单（动作慢了） | `451261818001662448549496928004841154941920422469` | 0.70 |
| evaluator | 仲裁方 | `1166803121960946152753035651627437280620957401630` | 0.85 |

> 演示无需连接钱包：Demo 编排器自动以各 Agent 身份发起链上交易（私钥仅存于本地 `.env`，不公开）。

### Demo 交易验证

```bash
# 查看 buyer Agent 的声誉分（应返回 500000000000000000 = 0.5e18）
cast call 0xe6Fb9e7788Ab7BCD1622485fb0F2dED9092C7A67 \
  "getScore(uint256)(uint256)" 263576404964353543971535190695064351300027826597 \
  --rpc-url https://testnet-rpc.monad.xyz

# 公告期（应返回 20）
cast call 0xC1aC936f57B983381E08BA0417CC1999C04DC6aF \
  "announcementPeriod()(uint256)" --rpc-url https://testnet-rpc.monad.xyz
```

---

## 架构

```
┌──────────────────────────────────────────────────┐
│                  Frontend (Next.js)               │
│  Landing · Demo · Agents · Jobs · Events         │
└──────────────────┬───────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────┐
│              Offchain (Go)                        │
│  Indexer · Evaluator · Keeper · REST API :9527   │
│  5 Agent 微服务 (buyer/senior/junior/rookie/eval) │
└──────────────────┬───────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────┐
│            Smart Contracts (Solidity)             │
│                                                   │
│  ┌──────────────┐  ┌──────────┐  ┌─────────────┐ │
│  │  Registry    │  │  Job     │  │ Arbitration │ │
│  │  Reputation  │  │  Escrow  │  │ Hook        │ │
│  │  256-shard   │  │  ERC-8183│  │ Dispute     │ │
│  └──────────────┘  └──────────┘  └─────────────┘ │
└──────────────────┬───────────────────────────────┘
                   │
        ┌──────────▼──────────┐
        │   Monad Parallel EVM │
        │   10,000+ TPS        │
        │   < $0.001 per tx    │
        └─────────────────────┘
```

---

## 合约

| 合约 | 行数 | 功能 |
|------|------|------|
| `PrismSettleRegistry` | 434 | 声誉存储、Agent 注册、Staking/Slashing、EMA 平滑、不活跃衰减 |
| `PrismSettleJob` | 270 | ERC-8183 Job 生命周期：create → fund → assign → submit → complete/refund |
| `ArbitrationHook` | 156 | 争议仲裁：buyer dispute → resolver ruling (1=buyer/2=provider)；公告期可配置 |

测试：**141 个测试全部通过**（forge 7 个套件：Registry / Job / ArbitrationHook / Integration / X402 等）。

---

## Monad 原生特性

| 特性 | 集成方式 |
|------|---------|
| **并行 EVM** | 256-shard 存储降低高并发写入冲突概率，适合 Agent 批量作业 |
| **x402 微支付** | `fundViaToken` 统一入口内置 x402 settle 路径，支持 Agent 间低额结算 |
| **低延迟** | 实时声誉更新，Agent 交易秒级确认 |

---

## 测试

```bash
cd contracts && forge test
# 结果: 141 passed, 0 failed
```

---

## 主要功能

| 功能 | 说明 |
|------|------|
| **链上托管结算** | ERC-8183 Job 生命周期：创建 → 托管 → 接单 → 提交 → 验收/打回 → 结算/退款 |
| **声誉系统** | 256-shard 声誉存储、EMA 平滑、不活跃衰减、验证者质押/惩罚 |
| **争议仲裁** | buyer/provider 争议 → 仲裁方裁定 → 败方付仲裁费、托管资金 100% 归胜方；公告期可配置（演示 20s） |
| **实时协作演示** | 编排器自动执行完整剧本：3 Agent 竞争抢单 → 交付 → 打回×2 → 仲裁 → 结算；每步 DeepSeek LLM 实时生成对话 + 真实链上交易 |
| **交付物锚定** | 完整报告正文落库 + 链上 keccak 哈希锚定；前端一键完整性校验（篡改可检测） |
| **x402 微支付** | `fundViaToken` 统一入口内置 x402 settle 路径，支持 Agent 间低额结算 |

## 技术栈

| 层 | 技术 |
|----|------|
| 智能合约 | Solidity 0.8.24, Foundry, OpenZeppelin |
| 后端 | Go, go-ethereum, gin, PostgreSQL, Redis |
| 前端 | Next.js 15, Wagmi（纯 wagmi 钱包连接，无远程依赖）, Tailwind CSS, framer-motion |
| 演示编排器 | DeepSeek LLM 对话生成 + 链上动作编排（仲裁/直接完成双场景） |
| 测试 | Foundry forge (141 测试) · Vitest (51 前端测试) · Go build/vet

---

## 目录结构

```
PrismSettle/
├── contracts/           # 智能合约（Foundry）
│   ├── src/             #   Registry + Job + Arbitration + Mocks
│   ├── test/            #   141 个测试
│   └── script/          #   Deploy.s.sol（第 6 套部署）
├── frontend/            # Next.js 前端
│   ├── app/             #   页面路由（jobs/agents/events/详情页）
│   ├── components/      #   组件（demo 聊天/状态跟踪/钱包）
│   └── lib/             #   合约配置 + API client
├── offchain/            # Go 后端 (indexer/evaluator/keeper/REST API)
│   ├── prismsettle/     #   PrismSettle 业务（含 demo 编排器包）
│   └── cmd/             #   入口（loadDotEnv 自动读 .env）
├── scripts/             # 部署和 demo 脚本
└── docs/                # 设计文档 / 部署日志 / 验证指南
```

---

## License

MIT
