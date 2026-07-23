"use client";

// AgentRegisterForm — registers a new agent on-chain via the PrismSettleRegistry
// contract. DEV-PLAN §Phase 8 任务 8.8 (FR-M06), wired in Phase 9.
//
// Calls Registry.registerAgent(agentId, metadata) where metadata is a JSON
// string: {"endpointUrl":"...","capabilities":"..."}.
// Wallet must be connected to Monad testnet (chainId 10143).

import { useState } from "react";
import { Loader2, UserPlus } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { REGISTRY_ADDRESS, REGISTRY_ABI, contractsReady } from "@/lib/contracts";

interface Props {
  onRegistered?: (agentId: string) => void;
}

export function AgentRegisterForm({ onRegistered }: Props) {
  const { address, chain } = useAccount();
  const { writeContractAsync, isPending: isWriting } = useWriteContract();
  const [agentId, setAgentId] = useState("");
  const [metadata, setMetadata] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);
  const [error, setError] = useState<string | null>(null);

  const { isLoading: isConfirming, isSuccess: isConfirmed } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  const ready = contractsReady() && REGISTRY_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setTxHash(null);
    if (!address) {
      setError("Please connect your wallet first.");
      return;
    }
    if (wrongChain) {
      setError(`Wrong network. Please switch to Monad Testnet (chainId ${monadTestnet.id}).`);
      return;
    }
    if (!REGISTRY_ADDRESS) {
      setError("Registry contract address not configured. Set NEXT_PUBLIC_REGISTRY_ADDRESS.");
      return;
    }
    let agentIdBig: bigint;
    try {
      agentIdBig = BigInt(agentId.startsWith("0x") ? agentId : agentId);
    } catch {
      setError("Invalid Agent ID. Use decimal or 0x-hex.");
      return;
    }
    let metaJson: string;
    try {
      const caps = metadata.trim() ? JSON.parse(metadata) : {};
      metaJson = JSON.stringify({ endpointUrl: endpoint, capabilities: caps });
    } catch {
      setError("Metadata must be valid JSON.");
      return;
    }
    try {
      const hash = await writeContractAsync({
        address: REGISTRY_ADDRESS,
        abi: REGISTRY_ABI,
        functionName: "registerAgent",
        args: [agentIdBig, metaJson],
        chainId: monadTestnet.id,
      });
      setTxHash(hash);
      onRegistered?.(agentId);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="mb-1 block text-sm font-medium text-white/80">Agent ID</label>
        <input
          type="text"
          required
          value={agentId}
          onChange={(e) => setAgentId(e.target.value)}
          placeholder="decimal (e.g. 42) or 0x-hex"
          className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
        />
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-white/80">Endpoint URL</label>
        <input
          type="url"
          required
          value={endpoint}
          onChange={(e) => setEndpoint(e.target.value)}
          placeholder="https://my-agent.example.com/invoke"
          className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
        />
        <p className="mt-1 text-[11px] text-white/40">Where buyers send invoke requests (FR-M11).</p>
      </div>
      <div>
        <label className="mb-1 block text-sm font-medium text-white/80">Capabilities (JSON, optional)</label>
        <textarea
          value={metadata}
          onChange={(e) => setMetadata(e.target.value)}
          placeholder='{"tags":["code","review"]}'
          rows={3}
          className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-xs text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
        />
      </div>

      {error && (
        <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">{error}</div>
      )}
      {txHash && (
        <div className="rounded-lg border border-white/10 bg-prism-surface/60 p-3 text-xs text-white/70">
          <div className="font-mono">tx: {txHash.slice(0, 18)}…{txHash.slice(-8)}</div>
          {isConfirming && <div className="mt-1 text-amber-300">Waiting for confirmation…</div>}
          {isConfirmed && <div className="mt-1 text-emerald-300">Agent registered on-chain.</div>}
        </div>
      )}

      <button
        type="submit"
        disabled={isWriting || isConfirming || !address || !ready || !!wrongChain}
        className="inline-flex items-center gap-2 rounded-lg bg-prism-accent px-5 py-2.5 text-sm font-semibold text-white hover:bg-prism-accent/80 disabled:opacity-50"
      >
        {isWriting || isConfirming ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin" /> Registering…
          </>
        ) : (
          <>
            <UserPlus className="h-4 w-4" /> Register Agent
          </>
        )}
      </button>
      {!address && <p className="text-[11px] text-white/40">Connect wallet (top-right) to register an agent.</p>}
      {address && wrongChain && (
        <p className="text-[11px] text-amber-400">Switch to Monad Testnet in your wallet.</p>
      )}
      {!ready && <p className="text-[11px] text-white/40">Contract addresses not configured.</p>}
    </form>
  );
}
