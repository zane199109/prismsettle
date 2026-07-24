# PrismSettle — Trusted Escrow + On-Chain Reputation

**Hire anyone &mdash; person, AI agent, or another AI &mdash; without trust.**
**一个协议，覆盖三种场景：P2P · P2A · A2A**

[![CI](https://github.com/zane/web3-offchain/actions/workflows/ci.yml/badge.svg)](https://github.com/zane/web3-offchain/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
![Solidity](https://img.shields.io/badge/Solidity-0.8.24-363636)
![Monad](https://img.shields.io/badge/Monad-Optimistic%20Parallel%20EVM-00BFFF)

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

**技术亮点：** 256-shard 声誉存储——利用 Monad 的并行 EVM 特性，将声誉数据分散到 256 个分片，消除高并发下的写入冲突（理论冲突概率降低 255/256）。

---

## 快速体验（免钱包、1 分钟）

```bash
# 1. 启动本地链 + 部署合约
bash scripts/deploy-anvil.sh

# 2. 启动前端
bash scripts/start-frontend.sh

# 3. 打开浏览器 → http://localhost:3000
#    点击 "Demo Mode" 全程模拟，无需安装钱包
```

**或直接跑命令行 demo：**
```bash
bash scripts/demo-minimal.sh
```

---

## Monad 黑客松演示

### 3 分钟讲稿

```
(0:00) 大家好，这是 PrismSettle——链上 Escrow + 声誉协议。

      它解决一个古老的问题：你要付钱让别人帮你干活，但你不信任对方。
      不管对方是真人、AI Agent、还是另一个 Agent—— PrismSettle 都管用。

(0:30) 三个场景，一个协议：

      P2P：你在论坛找了一个 freelancer 写合约。
           锁 100 USDC，对方交活，你确认，钱自动放款。

      P2A：你让 AI Agent 审计这份合约。
           预付款进 escrow，Agent 提交审计报告，验证通过后自动结算。

      A2A：两个 Agent 自动协作。
           套利 Agent A 发现机会，自动雇佣执行 Agent B，B 执行并提交 tx hash，
           链上验证后自动分账。全程无人类参与。

(1:30) 核心技术：256-shard 声誉存储。
      Monad 的并行 EVM 有个问题——多人同时修改同一个合约状态会冲突(abort)。
      我们把声誉数据拆成 256 个分片，99.6% 的情况下写入不冲突。

(2:00) 当前状态：
      - 3 个智能合约已部署到 Monad 测试网
      - 106 个单元测试全部通过
      - 测试网 Demo 已跑通（mint → createJob → fund → submit → complete）
      - Go 后端 + Next.js 前端

(2:30) 下一步：接入真实用户场景，运行 OCC 压测验证 60%→5%。
```

### 合约地址（Monad 测试网）

| 合约 | 地址 |
|------|------|
| MockERC20 | `0xe9ea3854bc57a49749c05190c577f4eCa9358861` |
| PrismSettleRegistry | `0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881` |
| ArbitrationHook | `0x61595999f64f73188F0C48db59698911491889B4` |
| PrismSettleJob | `0x548b2385723b8b9EdeEd99ddaD7830F7212C27ff` |

### Demo 交易验证

```bash
# 查看 Agent 0x1111 的声誉分（应返回 700000000000000000 = 0.7）
cast call 0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512 \
  "getScore(uint256)(uint256)" 0x1111 \
  --rpc-url http://127.0.0.1:8545
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
└──────────────────┬───────────────────────────────┘
                   │
┌──────────────────▼───────────────────────────────┐
│            Smart Contracts (Solidity)             │
│                                                   │
│  ┌──────────────┐  ┌──────────┐  ┌─────────────┐ │
│  │  Registry    │  │  Job     │  │ Arbitration │ │
│  │  256-shard   │  │  Escrow  │  │ Hook        │ │
│  │  Reputation  │  │  ERC-8183│  │ Dispute     │ │
│  └──────────────┘  └──────────┘  └─────────────┘ │
└──────────────────┬───────────────────────────────┘
                   │
        ┌──────────▼──────────┐
        │   Monad EVM         │
        │   (parallel OCC)    │
        └─────────────────────┘
```

---

## 合约

| 合约 | 行数 | 功能 |
|------|------|------|
| `PrismSettleRegistry` | 434 | 256-shard 声誉存储、Agent 注册、Staking/Slashing、EMA 平滑、不活跃衰减 |
| `PrismSettleJob` | 270 | ERC-8183 Job 生命周期：create → fund → assign → submit → complete/refund |
| `ArbitrationHook` | 156 | 争议仲裁：buyer dispute → resolver ruling (1=buyer/2=provider) |
| `BaselineRegistry` | 327 | V0 对照合约（单槽存储），用于后续 OCC 压测对比 |

测试：**106 个测试全部通过**（Registry 45 + Job 34 + ArbitrationHook 13 + Integration 8 + Baseline 6）

---

## 测试

```bash
cd contracts && forge test -vvv
# 结果: 106 passed, 0 failed
```

---

## 技术栈

| 层 | 技术 |
|----|------|
| 智能合约 | Solidity 0.8.24, Foundry, OpenZeppelin |
| 后端 | Go, Ethereum go-ethereum, chi router |
| 前端 | Next.js 15, RainbowKit, Wagmi, Tailwind CSS |
| 基础设施 | Docker Compose (8 服务), PostgreSQL, Redis |
| 测试 | Foundry forge (106 测试) |

---

## 目录结构

```
PrismSettle/
├── contracts/           # 智能合约（Foundry）
│   ├── src/             #   Registry + Job + Arbitration + Mocks
│   ├── test/            #   106 个测试
│   └── script/          #   Deploy.s.sol
├── frontend/            # Next.js 前端
│   ├── app/             #   页面路由
│   ├── components/      #   组件
│   └── lib/             #   合约配置 + ABI
├── offchain/            # Go 后端 (indexer/evaluator/keeper)
├── scripts/             # 部署和 demo 脚本
└── docs/                # 文档
```

---

## License

MIT
