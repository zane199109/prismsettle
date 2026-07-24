# PrismSettle — 验收与演示手册

> 版本：v1.0 · 2026-07-24
> 适用：Monad 黑客松提交 / 项目验收

---

## 一、先说清楚：这是做什么的

PrismSettle 是一个**链上 Escrow + 声誉协议**，覆盖三种场景：

| 场景 | 谁雇谁 | 一句话 |
|------|--------|--------|
| **P2P** | 人 → 人 | 在论坛找 freelancer，锁 USDC，交活放款 |
| **P2A** | 人 → AI Agent | 让 AI 审计合约，自动结算 |
| **A2A** | Agent → Agent | 两个 Agent 自动协作，链上自动分账 |

**核心流程都一样：Lock → Work → Release**

---

## 二、两种演示方式

### 方式 A：本地 Anvil（推荐首次体验）

```
前提：已安装 Foundry
耗时：约 2 分钟
```

```bash
cd /home/administrator/Documents/trae_projects/PrismSettle

# 1. 一键部署（启动 anvil + 部署合约 + 回填 .env）
bash scripts/deploy-anvil.sh

# 2. 启动前端
cd frontend && npm run dev

# 3. 打开浏览器 → http://localhost:3000
```

> 如果端口 3000 被 Docker 占用：`docker stop prismsettle-frontend`

### 方式 B：Monad 测试网（验证链上可用）

```
前提：.env 已填 DEPLOYER_KEY，钱包有测试 MON
耗时：约 3-5 分钟
```

```bash
bash scripts/deploy_monad_testnet.sh    # 部署合约
bash scripts/demo-testnet.sh             # 跑全链路 Demo
```

---

## 三、演示流程（评审体验路径）

### Step 0：Landing 页 — 30 秒理解价值

**动作：** 打开 `http://localhost:3000`

**评审看到：**
```
标题: "Hire anyone — person, agent, or another AI — without trust."
三个卡片: P2P / P2A / A2A
按钮: "Try Demo (no wallet)" 和 "Browse Agents"
```

**验收标准：**
- [ ] 标题清晰表达"不用信任对方"的核心价值
- [ ] 三张卡片覆盖 P2P/P2A/A2A 三种场景
- [ ] "Try Demo (no wallet)"按钮明显

### Step 1：Demo 页 — 无钱包看完完整流程

**动作：** 点 "Try Demo (no wallet)" 或导航到 `/demo`

**评审看到：**
```
┌─ Agent Marketplace ─────────────────────┐
│  DeFi Analyst    90%     ← 实时链上数据  │
│  Data Labeler    90%                    │
│  Translator      90%                    │
│  Evaluator       90%                    │
└─────────────────────────────────────────┘

┌─ The Job Flow ──────────────────────────┐
│  ① Lock    ② Work    ③ Release         │
│  USDC→合约  提交证据   验证→放款        │
└─────────────────────────────────────────┘

┌─ Demo Transaction Log ──────────────────┐
│  ✅ Create Job     0x2dbc... ← 真实 tx  │
│  ✅ Fund Escrow    0xd4f0...              │
│  ✅ Assign Provider 0x4c33...              │
│  ✅ Submit Proof   0x8be3...              │
│  ✅ Complete       0x8120...              │
└─────────────────────────────────────────┘
```

**验收标准：**
- [ ] 4 个 Agent 显示真实声誉分（应为 90% 或 0.7e18）
- [ ] 三步骤流程卡清晰可读
- [ ] 5 笔交易记录显示，有 tx hash（本地 anvil 或测试网数据）
- [ ] 页面不需要连接钱包

### Step 2：Agents 页 — 查看注册的 Agent

**动作：** 导航到 `/agents`

**评审看到：**
```
Agent Marketplace
4 registered agents · stake-weighted reputation

┌──────────────┐  ┌──────────────┐
│ DeFi Analyst  │  │ Data Labeler │
│ Score: 90%    │  │ Score: 90%   │
│ cap: defi     │  │ cap: labeling│
└──────────────┘  └──────────────┘
┌──────────────┐  ┌──────────────┐
│ Translator   │  │ Evaluator    │
│ Score: 90%   │  │ Score: 90%   │
│ cap: trans   │  │ cap: eval    │
└──────────────┘  └──────────────┘

[Register Agent]  ← 连接钱包后可点
```

**验收标准：**
- [ ] 4 个 Agent 都显示
- [ ] 每个 Agent 有声誉分
- [ ] "Register Agent"表单存在（提示连接钱包）

### Step 3：命令行 Demo（验证链上可用）

**动作：** 运行自动化脚本

```bash
cd /home/administrator/Documents/trae_projects/PrismSettle

# 本地 anvil
bash scripts/demo-minimal.sh

# 或测试网（需已部署）
bash scripts/demo-testnet.sh
```

**预期输出（demo-testnet.sh）：**
```
==============================================
  PrismSettle — Monad Testnet Demo
==============================================

[0] Current buyerNonce: 3
[1] Mint 10000 USDC         ✅
[2] Approve                 ✅
[3] Create Job              ✅  (jobId=...)
[4] Fund 10 USDC            ✅
[5] Assign                  ✅
[6] Submit proof            ✅
[7] Complete                ✅

Verify:
  Job State:  4 (Completed)     ✅
  Balance:    29,990 USDC       ✅
  Reputation: 0.7 (Agent 0x1111) ✅
```

**验收标准：**
- [ ] 所有 7 步打印 ✅
- [ ] Job State = 4（Completed）
- [ ] 余额变化正确（mint - fund + release = 预期值）
- [ ] Agent 声誉分 = 0.7e18

### Step 4：链上验证（核心技术点）

**动作：** 用 cast 直接查询

```bash
# 查看 Agent 0x1111 的声誉分
cast call <REGISTRY_ADDRESS> \
  "getScore(uint256)(uint256)" 0x1111 \
  --rpc-url https://testnet-rpc.monad.xyz
# 预期: 700000000000000000 (0.7e18)

# 查看部署账户的 USDC 余额
cast call <TOKEN_ADDRESS> \
  "balanceOf(address)(uint256)" <DEPLOYER_ADDRESS> \
  --rpc-url https://testnet-rpc.monad.xyz
# 预期: 链上有余额

# 在 Explorer 查看合约代码
open https://testnet.monadexplorer.com/address/<REGISTRY_ADDRESS>
```

**验收标准：**
- [ ] `getScore` 返回 0.7e18
- [ ] Explorer 显示合约字节码
- [ ] Explorer 显示交易历史

---

## 四、黑松那 10 问对照

| # | 问题 | PrismSettle 的回答 | 评审能验证什么 |
|---|------|-------------------|--------------|
| 1 | 用户是谁？ | 想找人/AI Agent 干活但不信任对方的人 | 看 Landing 页三种场景 |
| 2 | 用户遇到了什么具体问题？ | 先付钱怕对方不干，干完活怕不给钱 | Demo 页 Lock→Work→Release |
| 3 | 现有方案为什么不够好？ | 中心化平台不可验证，私下信任无保障 | 在 Demo 页口头解释 |
| 4 | 团队做出了什么？ | 3 合约 + Go 后端 + Next.js 前端 + 测试 | 看 GitHub / 跑 demo |
| 5 | 用户如何完成核心动作？ | Lock(锁 USDC) → Work(交证据) → Release(验证放款) | 跑 demo-testnet.sh |
| 6 | 哪些真实运行，哪些 Mock？ | 合约/demo/测试 ✅  压测数据是理论值 ⚠️ | 看代码 / 跑脚本 |
| 7 | 测试发现了什么？ | 106 测试全过，全链路跑通 | `forge test -vvv` |
| 8 | 根据反馈修改了什么？ | MVP 阶段，测试网部署后收集 | 诚实说还没做 |
| 9 | 为什么适合 Monad？ | 256-shard 解决 OCC 写冲突 | 看合约代码 + 架构文档 |
| 10 | 下一步准备做什么？ | 接入真实用户，跑 OCC 压测 | 看 README |

---

## 五、一键验收清单

提交前检查：

### 代码层面
- [ ] `forge test -vvv` → 106 passed
- [ ] `pnpm build`（如果装过依赖）→ 无错误
- [ ] `bash scripts/deploy-anvil.sh` → 部署成功，合约有代码
- [ ] `bash scripts/demo-minimal.sh` → 9/10 步骤通过

### 测试网层面
- [ ] `bash scripts/deploy_monad_testnet.sh` → ONCHAIN EXECUTION COMPLETE
- [ ] `bash scripts/demo-testnet.sh` → 7 步全 ✅，Job State = 4
- [ ] Explorer 可查合约代码
- [ ] `getScore(0x1111)` 返回 0.7e18

### 前端层面
- [ ] `http://localhost:3000` → Landing 页显示 P2P/P2A/A2A
- [ ] `/demo` → 显示 Agent 声誉分 + 交易日志
- [ ] `/agents` → 4 个 Agent 列表
- [ ] 导航只有 Home / Demo / Agents / Jobs / Events

### 提交材料层面
- [ ] README 有 30 秒看懂 + 3 分钟讲稿
- [ ] README 有测试网合约地址
- [ ] GitHub 仓库公开（提交前转 public）
- [ ] 10 个问题回答了
- [ ] Demo 视频录了（可选但推荐）

---

## 六、常见问题

**Q：前端报 "Contract addresses not configured"？**
A：没读到 `.env.local`。确保 `frontend/.env.local` 存在且内容正确。

**Q：demo-minimal.sh 第 10 步报错？**
A：因为脚本杀掉了 anvil。前 9 步通过就说明核心流程没问题。

**Q：测试网部署说 "not buyer"？**
A：jobId 计算需要 `abi.encodePacked`（非 padding 编码）。脚本 `demo-testnet.sh` 已修正。

**Q：前端连不上钱包？**
A：本地 anvil 需要在 MetaMask 添加网络：RPC `http://127.0.0.1:8545`，ChainID `31337`。测试网用 Monad Testnet。
