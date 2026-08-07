// Centralized contract configuration for PrismSettle frontend.
// Addresses are injected via env vars set at deploy time (see .env.example).
// ABIs are extracted from forge build artifacts (contracts/out/*.json).

import registryAbi from "@/lib/abi/PrismSettleRegistry.json";
import jobAbi from "@/lib/abi/PrismSettleJob.json";
import hookAbi from "@/lib/abi/ArbitrationHook.json";
import erc20Abi from "@/lib/abi/MockERC20.json";
import wmonAbi from "@/lib/abi/wmon.json";

export const REGISTRY_ADDRESS = process.env.NEXT_PUBLIC_REGISTRY_ADDRESS as `0x${string}` | undefined;
export const JOB_CONTRACT_ADDRESS = process.env.NEXT_PUBLIC_JOB_CONTRACT_ADDRESS as `0x${string}` | undefined;
export const HOOK_CONTRACT_ADDRESS = process.env.NEXT_PUBLIC_HOOK_CONTRACT_ADDRESS as `0x${string}` | undefined;
export const PAYMENT_TOKEN_ADDRESS = process.env.NEXT_PUBLIC_PAYMENT_TOKEN_ADDRESS as `0x${string}` | undefined;

// Official Monad testnet Wrapped MON (WMON, WETH9-style: deposit/withdraw).
// Verified on-chain: name "Wrapped MON", symbol WMON, decimals 18.
export const WMON_ADDRESS = "0xFb8bf4c1CC7a94c73D209a149eA2AbEa852BC541" as `0x${string}`;

export const REGISTRY_ABI = registryAbi;
export const JOB_ABI = jobAbi;
export const HOOK_ABI = hookAbi;
export const ERC20_ABI = erc20Abi;
export const WMON_ABI = wmonAbi;

/// Payment token for a currency choice: USDC → address(0) (contract default),
/// MON → WMON address (per-job token, ERC-20).
export function paymentTokenFor(currency: "usdc" | "mon"): `0x${string}` {
  return currency === "mon" ? WMON_ADDRESS : ("0x0000000000000000000000000000000000000000" as `0x${string}`);
}

// Monad testnet chain id (wagmi builtin: monadTestnet).
export const MONAD_TESTNET_CHAIN_ID = 10143;
export const MONAD_TESTNET_RPC = process.env.NEXT_PUBLIC_RPC_URL ?? "https://testnet-rpc.monad.xyz";

// Backend chain name — must match web3.chains[].chain_name in prod.yaml.
// Used by the /agent/invoke proxy (FR-M11) to resolve the agent's endpoint
// from agent_registry. Defaults to the PrismSettleRegistry listener name.
export const CHAIN_NAME = process.env.NEXT_PUBLIC_CHAIN_NAME ?? "monad_testnet";

// Guard: if addresses are missing, components should disable write actions
// and show a "contracts not deployed" hint instead of sending to address(0).
export function contractsReady(): boolean {
  return Boolean(REGISTRY_ADDRESS && JOB_CONTRACT_ADDRESS && HOOK_CONTRACT_ADDRESS);
}
