# x402 Facilitator 接入指南（FR-AP08）

> 对齐 PRD §4.8 + §8.3。本文档说明如何将 Monad 官方 x402 facilitator 接入 PrismSettleJob 合约。

## 1. 概述

PrismSettle 采用 **Monad 官方 x402 facilitator**，不自建。facilitator 负责：
1. 校验 BUYER 提交的 x402 收据（签名 + 防重放 + 时效）
2. 从 BUYER 账户拉取 testnet USDC 到 Job 合约
3. 返回实际划转金额

| 项目 | 值 |
|------|-----|
| Facilitator URL | https://x402-facilitator.molandak.org |
| Testnet USDC | `0x534b2f3A21130d7a60830c2Df862319e593943A3` |
| USDC decimals | 6 |
| 收据格式 | 见 [x402-receipt-format.md](file:///home/administrator/Documents/trae_projects/PrismSettle/docs/x402-receipt-format.md) |

## 2. 接入步骤

### 2.1 部署时注入 facilitator 地址

PrismSettleJob 合约构造函数接受 `facilitator` 地址参数：

```solidity
constructor(address token, address hookFacilitator) {
    // ...
    facilitator = hookFacilitator; // may be address(0)
}
```

部署脚本 [Deploy.s.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/script/Deploy.s.sol) 通过环境变量 `FACILITATOR_ADDRESS` 注入：

```bash
# Monad testnet 部署时（facilitator 启用）
export FACILITATOR_ADDRESS=0x...  # Monad 官方 facilitator 合约地址
forge script Deploy --rpc-url https://testnet-rpc.monad.xyz --broadcast

# Anvil 本地 / facilitator 不可用时（ERC-20 兜底）
# 不设置 FACILITATOR_ADDRESS，默认 address(0)
forge script Deploy --rpc-url http://127.0.0.1:8545 --broadcast
```

### 2.2 facilitator 必须实现 IX402Facilitator 接口

```solidity
interface IX402Facilitator {
    function settleWithReceipt(
        address payer,      // buyer
        address receiver,   // Job 合约
        bytes calldata receipt
    ) external returns (uint256 amount);
}
```

Monad 官方 facilitator 已实现此接口。若 facilitator 接口不兼容，需在 facilitator 外包一层 adapter 合约。

### 2.3 BUYER 端流程

1. **BUYER 调用 facilitator 的支付接口**（链下，HTTP 402 协议）
   - BUYER 通过 HTTP 请求 facilitator，获取 x402 receipt
   - facilitator 返回 `{receipt: bytes, amount: uint256, currency: address}`

2. **BUYER 调用 `fundViaToken(jobId, 0, receipt)`**
   - amount 传 0（金额由 facilitator 从收据中读取）
   - 合约转发 receipt 给 facilitator.settleWithReceipt
   - facilitator 校验收据后，从 BUYER 拉取 USDC 到 Job 合约
   - Job 合约状态：Created → Funded

3. **approve 预授权**
   - BUYER 必须先 `approve(facilitator, amount)` 授权 facilitator 拉取 USDC
   - 因为 facilitator 调用 `transferFrom(buyer, jobContract, amount)`

## 3. 兜底路径（FR-AP09）

当 facilitator 不可用时（`facilitator == address(0)` 或 facilitator 调用 revert），BUYER 走 ERC-20 兜底：

```solidity
// x402Receipt 为空 → ERC-20 路径
} else {
    require(amount > 0, "PrismSettle: zero amount");
    require(paymentToken.transferFrom(j.buyer, address(this), amount), "PrismSettle: transferFrom failed");
    funded = amount;
}
```

兜底流程：
1. BUYER `approve(jobContract, amount)` 授权 Job 合约
2. BUYER `fundViaToken(jobId, amount, "")` — receipt 传空
3. 合约直接 `transferFrom` 拉取 USDC

## 4. 测试验证

### 4.1 本地验证（anvil + Mock）

```bash
./scripts/verify_x402_integration.sh
```

覆盖：
- FR-AP06: 收据格式（opaque bytes）
- FR-AP07: fundViaToken 双路径分流
- FR-AP08: facilitator 接入（MockX402Facilitator）
- FR-AP09: 兜底路径 + 重放保护

### 4.2 Testnet 验证

部署到 Monad testnet 后：

```bash
# 1. 获取 x402 receipt（通过 facilitator HTTP 接口）
RECEIPT=$(curl -s https://x402-facilitator.molandak.org/pay \
  -d '{"amount":"1000000","currency":"0x534b2f3A21130d7a60830c2Df862319e593943A3"}')

# 2. BUYER 调用 fundViaToken（x402 路径）
cast send <JOB_ADDR> "fundViaToken(uint256,uint256,bytes)" <jobId> 0 "$RECEIPT" \
  --rpc-url https://testnet-rpc.monad.xyz --private-key <BUYER_KEY>

# 3. 验证 Job 余额
cast call <JOB_ADDR> "getJobState(uint256)" <jobId> --rpc-url https://testnet-rpc.monad.xyz

# 4. 兜底路径验证（facilitator 不可用场景）
cast send <JOB_ADDR> "fundViaToken(uint256,uint256,bytes)" <jobId2> 1000000 0x \
  --rpc-url https://testnet-rpc.monad.xyz --private-key <BUYER_KEY>
```

## 5. 故障处理

| 故障场景 | 处理方式 |
|---------|---------|
| facilitator HTTP 不可达 | BUYER 走 ERC-20 兜底路径（receipt 传空） |
| facilitator 合约 revert | fundViaToken 整笔 revert，BUYER 重试或走兜底 |
| 收据过期 | facilitator revert，BUYER 重新获取收据 |
| 收据重放 | 合约 `usedReceipts` mapping 拦截，revert "receipt used" |
| USDC approve 不足 | facilitator/合约 `transferFrom` revert |

## 6. 参考

- x402 协议规范：https://x402.org
- Monad 官方 facilitator：https://x402-facilitator.molandak.org
- 收据格式：[docs/x402-receipt-format.md](file:///home/administrator/Documents/trae_projects/PrismSettle/docs/x402-receipt-format.md)
- 合约接口：[IX402Facilitator.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/src/interfaces/IX402Facilitator.sol)
- 合约实现：[PrismSettleJob.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/src/PrismSettleJob.sol)
- 集成测试：[X402Integration.t.sol](file:///home/administrator/Documents/trae_projects/PrismSettle/contracts/test/X402Integration.t.sol)
- 验证脚本：[scripts/verify_x402_integration.sh](file:///home/administrator/Documents/trae_projects/PrismSettle/scripts/verify_x402_integration.sh)
