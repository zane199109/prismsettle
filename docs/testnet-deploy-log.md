# Monad Testnet 部署日志

**部署时间：** 2026-08-08 10:46:35 UTC
**部署账户：** 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
**网络：** Monad Testnet (chainId=10143)
**RPC：** https://testnet-rpc.monad.xyz
**Explorer：** https://testnet.monadexplorer.com

## 合约地址

| 合约 | 地址 | Explorer |
|------|------|----------|
| MockERC20 (测试 USDC) | 0x83cb612C10a27C09b7a5Ab31B906560B880abD9C | [https://testnet.monadexplorer.com/address/0x83cb612C10a27C09b7a5Ab31B906560B880abD9C](https://testnet.monadexplorer.com/address/0x83cb612C10a27C09b7a5Ab31B906560B880abD9C) |
| PrismSettleRegistry | 0xe6Fb9e7788Ab7BCD1622485fb0F2dED9092C7A67 | [https://testnet.monadexplorer.com/address/0xe6Fb9e7788Ab7BCD1622485fb0F2dED9092C7A67](https://testnet.monadexplorer.com/address/0xe6Fb9e7788Ab7BCD1622485fb0F2dED9092C7A67) |
| ArbitrationHook | 0x9039554fc84deebB4923E4236390634371E853f9 | [https://testnet.monadexplorer.com/address/0x9039554fc84deebB4923E4236390634371E853f9](https://testnet.monadexplorer.com/address/0x9039554fc84deebB4923E4236390634371E853f9) |
| PrismSettleJob | 0xC1aC936f57B983381E08BA0417CC1999C04DC6aF | [https://testnet.monadexplorer.com/address/0xC1aC936f57B983381E08BA0417CC1999C04DC6aF](https://testnet.monadexplorer.com/address/0xC1aC936f57B983381E08BA0417CC1999C04DC6aF) |

## 角色配置

| 角色 | 持有人 |
|------|--------|
| REGISTRY_EVALUATOR_ROLE | 0xcc6142f3f79Dd1d42FC0446C5B7218C5F520021E |
| COMMERCE_EVALUATOR_ROLE | 0xcc6142f3f79Dd1d42FC0446C5B7218C5F520021E |
| RESOLVER_ROLE | 0xcc6142f3f79Dd1d42FC0446C5B7218C5F520021E |

## 4 个官方 Agent

所有 4 个 Agent 已注册，初始声誉 0.7e18。
- DeFi Agent (0x1111)
- Labeling Agent (0x2222)
- Senior Auditor Agent (0x3333, 声誉 0.9e18)
- Eval Agent (0x4444)

## 附加 Agent

- Provider Agent (0x5555, 声誉 0.7e18) — 用于抢单流程
- Junior Auditor Agent (0x7777, 声誉 0.6e18) — 演示按声誉竞争抢单
- Rookie Auditor Agent (0x8888, 声誉 0.3e18) — 演示按声誉竞争抢单

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

Compiling 1 files with Solc 0.8.28
Solc 0.8.28 finished in 3.85s
Compiler run successful with warnings:
Warning (2018): Function state mutability can be restricted to view
   --> src/ArbitrationHook.sol:226:5:
    |
226 |     function onSubmitted(uint256 jobId) external {
    |     ^ (Relevant source part starts here and spans across multiple lines).

Script ran successfully.

== Logs ==
  MockERC20: 0x83cb612C10a27C09b7a5Ab31B906560B880abD9C
  Registry: 0xe6Fb9e7788Ab7BCD1622485fb0F2dED9092C7A67
  ArbitrationHook: 0x9039554fc84deebB4923E4236390634371E853f9
  PrismSettleJob: 0xC1aC936f57B983381E08BA0417CC1999C04DC6aF
  Evaluator: 0xcc6142f3f79Dd1d42FC0446C5B7218C5F520021E
  Facilitator: 0x7f6a2850669202519f0FE8aa912451238820Db86

## Setting up 1 EVM.

==========================

Chain 10143

Estimated gas price: 204.7143849 gwei

Estimated total gas used for script: 10309312

Estimated amount required: 2.1104644648221888 MON

==========================


==========================

ONCHAIN EXECUTION COMPLETE & SUCCESSFUL.

Transactions saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/broadcast/Deploy.s.sol/10143/run-latest.json

Sensitive values saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/cache/Deploy.s.sol/10143/run-latest.json
```
