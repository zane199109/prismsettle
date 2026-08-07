# Monad Testnet 部署日志

**部署时间：** 2026-08-07 12:34:02 UTC
**部署账户：** 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
**网络：** Monad Testnet (chainId=10143)
**RPC：** https://testnet-rpc.monad.xyz
**Explorer：** https://testnet.monadexplorer.com

## 合约地址

| 合约 | 地址 | Explorer |
|------|------|----------|
| MockERC20 (测试 USDC) | 0x90570a62436E201570C58B19B99d6eF77f76a62D | [https://testnet.monadexplorer.com/address/0x90570a62436E201570C58B19B99d6eF77f76a62D](https://testnet.monadexplorer.com/address/0x90570a62436E201570C58B19B99d6eF77f76a62D) |
| PrismSettleRegistry | 0x9B4F5056AB82d5E3b75D28a9288cFE754a3e84E9 | [https://testnet.monadexplorer.com/address/0x9B4F5056AB82d5E3b75D28a9288cFE754a3e84E9](https://testnet.monadexplorer.com/address/0x9B4F5056AB82d5E3b75D28a9288cFE754a3e84E9) |
| ArbitrationHook | 0xa230ceb08D3dF2BAEeFF44260e9a118f96271eD8 | [https://testnet.monadexplorer.com/address/0xa230ceb08D3dF2BAEeFF44260e9a118f96271eD8](https://testnet.monadexplorer.com/address/0xa230ceb08D3dF2BAEeFF44260e9a118f96271eD8) |
| PrismSettleJob | 0x4DD3275b169b386596034d2066430971A1F07f1e | [https://testnet.monadexplorer.com/address/0x4DD3275b169b386596034d2066430971A1F07f1e](https://testnet.monadexplorer.com/address/0x4DD3275b169b386596034d2066430971A1F07f1e) |

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

No files changed, compilation skipped
Script ran successfully.

== Logs ==
  MockERC20: 0x90570a62436E201570C58B19B99d6eF77f76a62D
  Registry: 0x9B4F5056AB82d5E3b75D28a9288cFE754a3e84E9
  ArbitrationHook: 0xa230ceb08D3dF2BAEeFF44260e9a118f96271eD8
  PrismSettleJob: 0x4DD3275b169b386596034d2066430971A1F07f1e
  Evaluator: 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
  Facilitator: 0x7f6a2850669202519f0FE8aa912451238820Db86

## Setting up 1 EVM.

==========================

Chain 10143

Estimated gas price: 203.048828126 gwei

Estimated total gas used for script: 10030052

Estimated amount required: 2.036590304642842552 MON

==========================


==========================

ONCHAIN EXECUTION COMPLETE & SUCCESSFUL.

Transactions saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/broadcast/Deploy.s.sol/10143/run-latest.json

Sensitive values saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/cache/Deploy.s.sol/10143/run-latest.json
```
