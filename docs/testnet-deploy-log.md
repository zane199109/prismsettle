# Monad Testnet 部署日志

**部署时间：** 2026-08-07 15:16:37 UTC
**部署账户：** 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
**网络：** Monad Testnet (chainId=10143)
**RPC：** https://testnet-rpc.monad.xyz
**Explorer：** https://testnet.monadexplorer.com

## 合约地址

| 合约 | 地址 | Explorer |
|------|------|----------|
| MockERC20 (测试 USDC) | 0x252e44550f8B9997901e5540FC0E1dA52Ab099C6 | [https://testnet.monadexplorer.com/address/0x252e44550f8B9997901e5540FC0E1dA52Ab099C6](https://testnet.monadexplorer.com/address/0x252e44550f8B9997901e5540FC0E1dA52Ab099C6) |
| PrismSettleRegistry | 0xA82937ad81e8aB775c9B32F363CE5E8564207739 | [https://testnet.monadexplorer.com/address/0xA82937ad81e8aB775c9B32F363CE5E8564207739](https://testnet.monadexplorer.com/address/0xA82937ad81e8aB775c9B32F363CE5E8564207739) |
| ArbitrationHook | 0x740c2969e537706A4f4757166e5eBEeD0E4DAD15 | [https://testnet.monadexplorer.com/address/0x740c2969e537706A4f4757166e5eBEeD0E4DAD15](https://testnet.monadexplorer.com/address/0x740c2969e537706A4f4757166e5eBEeD0E4DAD15) |
| PrismSettleJob | 0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB | [https://testnet.monadexplorer.com/address/0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB](https://testnet.monadexplorer.com/address/0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB) |

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

Compiling 1 files with Solc 0.8.28
Solc 0.8.28 finished in 3.51s
Compiler run successful with warnings:
Warning (2018): Function state mutability can be restricted to view
   --> src/ArbitrationHook.sol:224:5:
    |
224 |     function onSubmitted(uint256 jobId) external {
    |     ^ (Relevant source part starts here and spans across multiple lines).

Script ran successfully.

== Logs ==
  MockERC20: 0x252e44550f8B9997901e5540FC0E1dA52Ab099C6
  Registry: 0xA82937ad81e8aB775c9B32F363CE5E8564207739
  ArbitrationHook: 0x740c2969e537706A4f4757166e5eBEeD0E4DAD15
  PrismSettleJob: 0x4B09DB038dF842277f3f1aD4500b9BEDBFcB47cB
  Evaluator: 0xcc6142f3f79Dd1d42FC0446C5B7218C5F520021E
  Facilitator: 0x7f6a2850669202519f0FE8aa912451238820Db86

## Setting up 1 EVM.

==========================

Chain 10143

Estimated gas price: 204.602518377 gwei

Estimated total gas used for script: 10102191

Estimated amount required: 2.066933719725464007 MON

==========================


==========================

ONCHAIN EXECUTION COMPLETE & SUCCESSFUL.

Transactions saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/broadcast/Deploy.s.sol/10143/run-latest.json

Sensitive values saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/cache/Deploy.s.sol/10143/run-latest.json
```
