# Agent 注册 + 竞争演示流程

## 能力选择：Smart Contract Auditor

选择一个 Web3 原生的能力，方便演示和理解：

| Agent | 声誉 | 角色描述 |
|-------|------|---------|
| 0x3333 (auditor-senior) | **0.9e18** | 资深审计师，从业多年，声誉最高 |
| 0x7777 (auditor-junior) | **0.6e18** | 普通审计师，有一定经验 |
| 0x8888 (auditor-rookie) | **0.3e18** | 新手审计师，刚注册 |

## 完整流程

### 1. Agent 注册（前端页面）

买方在平台注册成为 Smart Contract Auditor：
- 填写 Name、Endpoint URL、Wallet Address
- 点击提交 → `registerAgent(agentId, '{"endpointUrl":"...","capabilities":"smart_contract_audit"}')`
- 注册信息上链，Registry 返回 Agent ID

> 演示时：3 个角色（资深/普通/新手）已预先注册好，声誉不同

### 2. 创建 Job

买方创建审计任务：
- 任务描述："Audit the contract at 0xABCD... for reentrancy vulnerabilities"
- `minProviderReputation = 0.5e18`
- 托管 100 USDC

### 3. Agent 自扫描 + 抢单（自动竞争）

每个 Agent 服务启动后，后台 goroutine 周期性轮询 offchain API：
```
GET /api/jobs?status=Funded&minReputation={my_rep}&capability=smart_contract_audit
```

发现匹配 Job 后，Agent 用自己的钱包调用 `grabJob`：

```
Agent 0x8888 (rep 0.3e18) → grabJob(jobId, 0x8888)
  → 合约检查: 0.3e18 < 0.5e18 → REVERT ❌

Agent 0x7777 (rep 0.6e18) → grabJob(jobId, 0x7777)
  → 合约检查: 0.6e18 ≥ 0.5e18 → OK ✅
  → Job 状态变为 Assigned, provider = 0x7777

Agent 0x3333 (rep 0.9e18) → grabJob(jobId, 0x3333)
  → 合约检查: 已分配 → REVERT ❌
```

**演示效果：**
- 前端显示 3 个 Agent 同时抢单
- 0x8888 灰显 + "Reputation too low"
- 0x7777 显示 "抢单成功"
- 0x3333 显示 "已被抢走，下次更快"

### 4. 执行 + 提交

抢到单的 Agent (0x7777) 自动执行：
1. 调用自身 HTTP `/invoke` 做审计（mock 返回结果）
2. 调用 `submit(jobId, deliverableHash, proofHash)`

### 5. 验证 + 结算

Evaluator 检测到 Submitted 状态：
1. 调用 agent-eval 评分
2. score ≥ 阈值 → 调用 `complete(jobId)`
3. 资金释放给 0x7777
4. 声誉更新

## 实现要点

### Agent 服务改动

每个 worker agent 在 HTTP 服务基础上增加后台 goroutine：

```go
func (a *WorkerAgent) Start(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                a.scanAndGrab(ctx)
            }
        }
    }()
    a.Serve(ctx, a.port) // HTTP 服务
}

func (a *WorkerAgent) scanAndGrab(ctx context.Context) {
    // 1. 轮询 offchain API 获取开放 Job
    // 2. 过滤能力匹配 + 声誉门槛
    // 3. 调用 grabJob
    // 4. 如果成功，执行工作并 submit
}
```

### 需要新增的代码

| 文件 | 改动 |
|------|------|
| `offchain/agents/worker_agent.go` | 新增 WorkerAgent 实现（HTTP + 后台扫描） |
| `offchain/cmd/agent/main.go` | 新增 `worker` 类型 |
| `offchain/config/agents.yaml` | 新增 3 个 worker 配置 |
| `docker-compose.yml` | 新增 3 个 worker 服务 |
| `contracts/script/Deploy.s.sol` | 修改声誉种子值 |
| `frontend/components/...` | 新增 Agent 注册页面 |

## 图示

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ 0x8888       │     │ 0x7777       │     │ 0x3333       │
│ Auditor      │     │ Auditor      │     │ Auditor      │
│ Rep: 0.3e18  │     │ Rep: 0.6e18  │     │ Rep: 0.9e18  │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                     │                     │
       │   ┌─────────────────────────────────────┐ │
       └───│  Offchain API: 新 Job 可用          │─┘
           │  minReputation=0.5e18               │
           └─────────────────────────────────────┘
       │                     │                     │
       ▼                     ▼                     ▼
  grabJob()             grabJob()             grabJob()
  REVERT ✅              SUCCESS ✅            REVERT ✅
  "声誉不足"              "抢单成功"             "已被抢走"