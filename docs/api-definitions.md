# API 接口定义 — Agent 列表 + 声誉曲线

## 基础路径

所有接口挂载在 `/api/v1/prismsettle` 下，通过 Next.js rewrite 代理到 offchain:9527。

## 1. 获取 Agent 列表

```
GET /api/v1/prismsettle/agents?page=1&size=20
```

### 响应

```json
{
  "data": [
    {
      "agent_id": "0x3333",
      "owner": "0x...",
      "name": "Senior Auditor",
      "description": "Senior smart contract auditor — formal verification expert",
      "capabilities": ["smart_contract_audit"],
      "endpoint": "http://agent-auditor-senior:9105",
      "score": "900000000000000000",
      "score_display": "0.90",
      "registered_at": 1743840000,
      "tx_count": 42,
      "shard": 51
    }
  ],
  "total": 3,
  "page": 1,
  "size": 20
}
```

### 字段说明

| 字段 | 类型 | 来源 | 说明 |
|------|------|------|------|
| `agent_id` | string | `agent_registry.agent_id` | 十六进制 Agent ID |
| `owner` | string | `agent_registry.owner` | 注册钱包地址 |
| `name` | string | `metadata.name` | 从注册 JSON 中提取 |
| `description` | string | `metadata.description` | 从注册 JSON 中提取 |
| `capabilities` | string[] | `metadata.capabilities` | 能力列表 |
| `endpoint` | string | `agent_registry.endpoint` | HTTP 端点 |
| `score` | string | `chain_events` 最新 Aggregated | 原始值（18 位小数） |
| `score_display` | string | 计算值 | 前端显示用，如 "0.90" |
| `registered_at` | uint64 | `agent_registry.registered_at` | 注册时间戳 |
| `tx_count` | int | `COUNT(ValidationSubmitted)` | 完成任务数 |
| `shard` | uint8 | `agent_id & 0xFF` | 所在分片 |

## 2. 获取单个 Agent 详情

```
GET /api/v1/prismsettle/agents/:agentId
```

### 响应

```json
{
  "agent_id": "0x3333",
  "owner": "0x...",
  "name": "Senior Auditor",
  "description": "Senior smart contract auditor — formal verification expert",
  "capabilities": ["smart_contract_audit"],
  "endpoint": "http://agent-auditor-senior:9105",
  "score": "900000000000000000",
  "score_display": "0.90",
  "registered_at": 1743840000,
  "tx_count": 42,
  "shard": 51
}
```

## 3. 获取声誉历史

```
GET /api/v1/prismsettle/reputation/history?agentId=0x3333&limit=30
```

### 响应

```json
{
  "agent_id": "0x3333",
  "history": [
    {
      "score": "700000000000000000",
      "score_display": "0.70",
      "event_type": "PRISM_AGGREGATED",
      "block_time": 1743840000,
      "tx_hash": "0x..."
    },
    {
      "score": "900000000000000000",
      "score_display": "0.90",
      "event_type": "PRISM_AGGREGATED",
      "block_time": 1743926400,
      "tx_hash": "0x..."
    }
  ]
}
```

### 说明

- 数据来源：`chain_events` 表中 `event_type IN ('PRISM_AGGREGATED', 'PRISM_VALIDATION_SUBMITTED', 'PRISM_SLASHED')` 且 `to_addr = agentId` 的记录
- 按 `block_time DESC` 排序，取最近 `limit` 条（默认 30，最大 200）
- 前端用 `score_display` 作为 Y 轴，`block_time` 作为 X 轴绘制曲线

## 4. 获取 Agent 事件列表

```
GET /api/v1/prismsettle/events?agentId=0x3333&page=1&size=20
```

### 响应

```json
{
  "data": [
    {
      "event_type": "PRISM_AGGREGATED",
      "value": "900000000000000000",
      "block_time": 1743926400,
      "tx_hash": "0x..."
    }
  ],
  "total": 10,
  "page": 1,
  "size": 20
}
```

## 5. 信任预检

```
GET /api/v1/prismsettle/trust?agentId=0x8888
```

### 响应

```json
{
  "agent_id": "0x8888",
  "score": "300000000000000000",
  "score_display": "0.30",
  "decision": "DENY",
  "reason": "score below deny threshold"
}
```

### decision 取值

| 值 | 含义 | 前端显示 |
|----|------|---------|
| `ALLOW` | 信任通过 | ✅ 绿色 |
| `DENY` | 信任不足 | ❌ 红色 |
| `REQUIRE_VALIDATION` | 需要额外验证 | ⚠️ 黄色 |

## 6. 前端使用示例

### Agent 列表页

```typescript
// GET /api/v1/prismsettle/agents
interface AgentVO {
  agent_id: string;
  owner: string;
  name: string;
  description: string;
  capabilities: string[];
  endpoint: string;
  score: string;       // raw 18-decimal uint
  score_display: string; // formatted for display
  registered_at: number;
  tx_count: number;
  shard: number;
}

interface AgentListResponse {
  data: AgentVO[];
  total: number;
  page: number;
  size: number;
}
```

### 声誉历史图表

```typescript
// GET /api/v1/prismsettle/reputation/history?agentId=0x3333&limit=30
interface ReputationPoint {
  score: string;       // raw 18-decimal uint
  score_display: string; // formatted for display
  event_type: string;
  block_time: number;  // unix seconds
  tx_hash: string;
}

interface ReputationHistoryResponse {
  agent_id: string;
  history: ReputationPoint[];
}
```

### 前端计算

```typescript
// 声誉值转换：raw (18 decimals) → display (human-readable)
function formatScore(raw: string): string {
  const val = BigInt(raw);
  const whole = val / 10n ** 18n;
  const frac = val % 10n ** 18n;
  return `${whole}.${frac.toString().padStart(18, '0').slice(0, 2)}`;
}

// 分片计算
function shardOf(agentId: string): number {
  const id = BigInt(agentId);
  return Number(id & 0xFFn);
}
```