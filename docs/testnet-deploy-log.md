# Monad Testnet 部署日志

**部署时间：** 2026-08-08 06:56:15 UTC
**部署账户：** 0x8EB3Fe3dDe56Cab0CDf32db3e6E5bA865596BE2C
**网络：** Monad Testnet (chainId=10143)
**RPC：** https://testnet-rpc.monad.xyz
**Explorer：** https://testnet.monadexplorer.com

## 合约地址

| 合约 | 地址 | Explorer |
|------|------|----------|
| MockERC20 (测试 USDC) | 0x2Bb06A30D464cA8e62563081f024E6380f0EB70b | [https://testnet.monadexplorer.com/address/0x2Bb06A30D464cA8e62563081f024E6380f0EB70b](https://testnet.monadexplorer.com/address/0x2Bb06A30D464cA8e62563081f024E6380f0EB70b) |
| PrismSettleRegistry | 0xC55Eb9d42fE23D82C2830Fb3a2488DB6442c3bc7 | [https://testnet.monadexplorer.com/address/0xC55Eb9d42fE23D82C2830Fb3a2488DB6442c3bc7](https://testnet.monadexplorer.com/address/0xC55Eb9d42fE23D82C2830Fb3a2488DB6442c3bc7) |
| ArbitrationHook | 0x26562169d70137095d8090Cee258721CB2F496F9 | [https://testnet.monadexplorer.com/address/0x26562169d70137095d8090Cee258721CB2F496F9](https://testnet.monadexplorer.com/address/0x26562169d70137095d8090Cee258721CB2F496F9) |
| PrismSettleJob | 0xD66Af51c864b83ae4dB0F35898708C184E212a16 | [https://testnet.monadexplorer.com/address/0xD66Af51c864b83ae4dB0F35898708C184E212a16](https://testnet.monadexplorer.com/address/0xD66Af51c864b83ae4dB0F35898708C184E212a16) |

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
Solc 0.8.28 finished in 3.77s
Compiler run successful with warnings:
Warning (2018): Function state mutability can be restricted to view
   --> src/ArbitrationHook.sol:224:5:
    |
224 |     function onSubmitted(uint256 jobId) external {
    |     ^ (Relevant source part starts here and spans across multiple lines).

Script ran successfully.

== Logs ==
  MockERC20: 0x2Bb06A30D464cA8e62563081f024E6380f0EB70b
  Registry: 0xC55Eb9d42fE23D82C2830Fb3a2488DB6442c3bc7
  ArbitrationHook: 0x26562169d70137095d8090Cee258721CB2F496F9
  PrismSettleJob: 0xD66Af51c864b83ae4dB0F35898708C184E212a16
  Evaluator: 0xcc6142f3f79Dd1d42FC0446C5B7218C5F520021E
  Facilitator: 0x7f6a2850669202519f0FE8aa912451238820Db86

## Setting up 1 EVM.

==========================

Chain 10143

Estimated gas price: 218.984332878 gwei

Estimated total gas used for script: 10177582

Estimated amount required: 2.228731004581140996 MON

==========================


==========================

ONCHAIN EXECUTION COMPLETE & SUCCESSFUL.

Transactions saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/broadcast/Deploy.s.sol/10143/run-latest.json

Sensitive values saved to: /home/administrator/Documents/trae_projects/PrismSettle/contracts/cache/Deploy.s.sol/10143/run-latest.json
```
