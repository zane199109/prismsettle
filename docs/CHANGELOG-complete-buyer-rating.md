# complete() Buyer Rating 变更说明

## 概述

将 `PrismSettleJob.complete()` 的调用权限从 **Evaluator**（持 `COMMERCE_EVALUATOR_ROLE`）改为 **Buyer**（雇主），并引入 Buyer 直接评分机制。Evaluator 不再参与主路径，仅在仲裁时介入。

## 变更点

### 1. 调用者变更

| 项目 | 修改前 | 修改后 |
|------|--------|--------|
| 调用者 | Evaluator（持 `COMMERCE_EVALUATOR_ROLE`） | Buyer（`msg.sender == j.buyer`） |
| 权限检查 | `hasRole(COMMERCE_EVALUATOR_ROLE, msg.sender)` | `require(msg.sender == j.buyer, "...")` |

### 2. 函数签名

```solidity
// 修改前
function complete(uint256 jobId) external;

// 修改后
function complete(uint256 jobId, uint96 score) external;
```

`score` 参数：Buyer 对 Provider 的评分，取值范围 0..1e18（fixed-point）。

### 3. 声誉更新（source=3）

`complete()` 调用 `Registry.submitValidation` 记录 Buyer 评分，source=3 标识 Buyer 渠道：

```solidity
if (registry != address(0) && j.providerAgentId != 0) {
    IRegistryWriter(registry).submitValidation(
        j.providerAgentId, score, j.proofHash, jobId, 3
    );
}
```

### 4. Registry 新增接口

| 函数 | 说明 |
|------|------|
| `setTrustedJob(address job, bool trusted)` | 管理员设置可信 Job 合约（允许 source=3 调用） |
| `trustedJobs(address) → bool` | 查询 Job 合约是否被信任 |

`submitValidation` 中 source=3 的权限检查：

```solidity
} else if (source == 3) {
    require(trustedJobs[msg.sender], "PrismSettle: not trusted job");
}
```

### 5. Evaluator 职责变更

| 场景 | 修改前 | 修改后 |
|------|--------|--------|
| 主路径（无争议） | Evaluator 调用 `complete()` + `submitValidation(source=1)` | Buyer 调用 `complete(score)` + 自动 `submitValidation(source=3)` |
| 仲裁路径（有争议） | Evaluator 裁决 + `submitValidation(source=2)` | 不变 |

## 影响范围

| 模块 | 变更 |
|------|------|
| `PrismSettleJob.sol` | `complete()` 签名和逻辑修改 |
| `PrismSettleRegistry.sol` | 新增 `trustedJobs` 映射 + `setTrustedJob` |
| 链下 Evaluator | 移除主路径处理（`pollSubmitted`/`processSubmitted`），仅保留仲裁 |
| 前端 | 调用 `complete(jobId, score)` 需传入评分参数 |
| 测试 | 所有 `complete()` 调用更新为 buyer + score 参数 |

## 评分源标识

| source | 调用方 | 说明 |
|--------|--------|------|
| 0 | Validator | 质押验证者评分 |
| 1 | Evaluator | 评估者评分（主路径，已移除） |
| 2 | Evaluator | 仲裁评分 |
| 3 | Buyer | 买方评分（新增） |