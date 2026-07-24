# Monad Testnet 部署日志

**部署时间：** 2026-07-24 14:41:30 UTC
**部署账户：** 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
**网络：** Monad Testnet (chainId=10143)
**RPC：** https://testnet-rpc.monad.xyz
**Explorer：** https://testnet.monadexplorer.com

## 合约地址

| 合约 | 地址 | Explorer |
|------|------|----------|
| MockERC20 (测试 USDC) | 0xe9ea3854bc57a49749c05190c577f4eCa9358861 | [https://testnet.monadexplorer.com/address/0xe9ea3854bc57a49749c05190c577f4eCa9358861](https://testnet.monadexplorer.com/address/0xe9ea3854bc57a49749c05190c577f4eCa9358861) |
| PrismSettleRegistry | 0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881 | [https://testnet.monadexplorer.com/address/0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881](https://testnet.monadexplorer.com/address/0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881) |
| ArbitrationHook | 0x61595999f64f73188F0C48db59698911491889B4 | [https://testnet.monadexplorer.com/address/0x61595999f64f73188F0C48db59698911491889B4](https://testnet.monadexplorer.com/address/0x61595999f64f73188F0C48db59698911491889B4) |
| PrismSettleJob | 0x548b2385723b8b9EdeEd99ddaD7830F7212C27ff | [https://testnet.monadexplorer.com/address/0x548b2385723b8b9EdeEd99ddaD7830F7212C27ff](https://testnet.monadexplorer.com/address/0x548b2385723b8b9EdeEd99ddaD7830F7212C27ff) |

## 角色配置

| 角色 | 持有人 |
|------|--------|
| REGISTRY_EVALUATOR_ROLE | 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C |
| COMMERCE_EVALUATOR_ROLE | 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C |
| RESOLVER_ROLE | 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C |

## 4 个官方 Agent

所有 4 个 Agent 已注册，初始声誉 0.7e18。
- DeFi Agent (0x1111)
- Labeling Agent (0x2222)
- Translate Agent (0x3333)
- Eval Agent (0x4444)

## 验证步骤

1. 启动 offchain:
   `cd offchain && go run ./cmd -config config/prod.yaml`

2. 前端加载：
   部署后 .env 已填入合约地址，docker compose 直接可用

3. 测试交易：
   ```bash
   cast call $REGISTRY_ADDR "getScore(uint256)(uint256)" 0x1111 --rpc-url $MONAD_RPC
   ```

## 完整部署日志

```
Warning: This is a nightly build of Foundry. It is recommended to use the latest stable version. To mute this warning set `FOUNDRY_DISABLE_NIGHTLY_WARNING` in your environment. 

Compiling 32 files with Solc 0.8.24
Solc 0.8.24 finished in 7.50s
Compiler run successful with warnings:
Warning (2018): Function state mutability can be restricted to view
  --> src/ArbitrationHook.sol:98:5:
   |
98 |     function onSubmitted(uint256 jobId) external {
   |     ^ (Relevant source part starts here and spans across multiple lines).

Script ran successfully.

== Logs ==
  MockERC20: 0xe9ea3854bc57a49749c05190c577f4eCa9358861
  Registry: 0x296d8DfDc0E306e3472a49CE5C9e0B7a68066881
  ArbitrationHook: 0x61595999f64f73188F0C48db59698911491889B4
  PrismSettleJob: 0x548b2385723b8b9EdeEd99ddaD7830F7212C27ff
  Evaluator: 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
  Facilitator: 0x0000000000000000000000000000000000000000

## Setting up 1 EVM.

==========================

Chain 10143

Estimated gas price: 204.994855161 gwei

Estimated total gas used for script: 6149139

Estimated amount required: 1.260541858669856379 MON

==========================


==========================

ONCHAIN EXECUTION COMPLETE & SUCCESSFUL.

Transactions saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/broadcast/Deploy.s.sol/10143/run-latest.json

Sensitive values saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/cache/Deploy.s.sol/10143/run-latest.json
```
