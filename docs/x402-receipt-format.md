# x402 收据格式规范（FR-AP06）

> 对齐 PRD §4.8 + SD §3.3 + ERC-8183 Job 资金托管层。

## 1. 概述

PrismSettleJob 合约通过 `fundViaToken(jobId, amount, x402Receipt)` 统一资金入口接入 x402 微支付协议。当 `x402Receipt` 非空时走 x402 settle 路径，由 Monad 官方 facilitator 校验收据并完成 USDC 划转；为空时走标准 ERC-20 transferFrom 兜底路径。

## 2. 收据格式（FR-AP06）

x402 收据为**不透明 bytes**（`bytes calldata`），其内部结构由 facilitator 定义。PrismSettle 合约层**不解析收据内容**，只做三件事：

1. **非空校验**：`require(x402Receipt.length > 0)` → 进入 x402 路径
2. **防重放**：`keccak256(x402Receipt)` 存入 `usedReceipts` mapping
3. **转发给 facilitator**：`IX402Facilitator(facilitator).settleWithReceipt(buyer, address(this), receipt)`

### 2.1 逻辑格式（facilitator 侧定义）

参考 x402 协议规范，收据逻辑结构为：

```json
{
  "receipt": "0x...",     // bytes — 不透明签名收据，由 facilitator 验签
  "amount": "1000000",    // uint256 — USDC 最小单位（6 decimals）
  "currency": "0x534b..." // address — testnet USDC 合约地址
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `receipt` | `bytes` | facilitator 签名的收据原文，合约层不解析 |
| `amount` | `uint256` | USDC 金额（6 decimals，即 1 USDC = 1000000） |
| `currency` | `address` | testnet USDC：`0x534b2f3A21130d7a60830c2Df862319e593943A3` |

### 2.2 合约层处理

```solidity
// PrismSettleJob.sol — fundViaToken
if (x402Receipt.length > 0) {
    require(facilitator != address(0), "PrismSettle: no facilitator");
    require(amount == 0, "PrismSettle: amount must be 0 with receipt");
    bytes32 receiptHash = keccak256(x402Receipt);
    require(!usedReceipts[receiptHash], "PrismSettle: receipt used");
    usedReceipts[receiptHash] = true;
    funded = IX402Facilitator(facilitator).settleWithReceipt(j.buyer, address(this), x402Receipt);
}
```

- `amount` 参数传 0（金额由 facilitator 从收据中读取）
- 收据哈希防重放，**跨 Job 也防重放**（同一收据只能用一次）
- facilitator 返回实际划转金额，写入 `j.amount`

## 3. Facilitator 接口（IX402Facilitator）

```solidity
interface IX402Facilitator {
    function settleWithReceipt(
        address payer,      // buyer（USDC 来源）
        address receiver,   // Job 合约（USDC 接收方）
        bytes calldata receipt // 不透明收据
    ) external returns (uint256 amount);
}
```

- `payer` = `j.buyer`（创建 Job 时锁定）
- `receiver` = `address(this)`（Job 合约本身）
- `amount` = 实际划转的 USDC 数量

## 4. 防重放保证

| 层级 | 机制 |
|------|------|
| 合约层 | `usedReceipts[keccak256(receipt)]` — 同一收据全局只能用一次 |
| Facilitator 层 | 收据含 nonce + 过期时间，facilitator 内部校验签名 + 时效 |

## 5. 兜底路径（FR-AP09）

当 facilitator 不可用时（`facilitator == address(0)` 或 facilitator 调用 revert），BUYER 走 ERC-20 兜底：

```solidity
// x402Receipt 为空 → ERC-20 路径
} else {
    require(amount > 0, "PrismSettle: zero amount");
    require(paymentToken.transferFrom(j.buyer, address(this), amount), "PrismSettle: transferFrom failed");
    funded = amount;
}
```

- BUYER 需提前 `approve(paymentToken, jobContract, amount)`
- 状态转换与 x402 路径一致：`Created → Funded`
- 触发相同的 `Funded(jobId, buyer, amount)` 事件

## 6. Testnet USDC

| 网络 | USDC 地址 | Decimals |
|------|-----------|----------|
| Monad Testnet | `0x534b2f3A21130d7a60830c2Df862319e593943A3` | 6 |
| Anvil（本地） | MockERC20（Deploy.s.sol 部署） | 18 |

## 7. 参考

- x402 协议规范：https://x402.org
- Monad 官方 facilitator：https://x402-facilitator.molandak.org
- PRD §4.8 x402 支付流
- SD §3.3 PrismSettleJob 合约设计
- 合约代码：[PrismSettleJob.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/src/PrismSettleJob.sol)
- 接口定义：[IX402Facilitator.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/src/interfaces/IX402Facilitator.sol)
