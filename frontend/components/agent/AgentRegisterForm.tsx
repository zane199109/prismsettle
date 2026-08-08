"use client";

// AgentRegisterForm — registers a new agent on-chain via the PrismSettleRegistry
// contract.
//
// Calls Registry.registerAgent(agentId, metadata) where metadata is a JSON
// string: {"endpointUrl":"...","capabilities":{"tags":[...]}}.
// Wallet must be connected to Monad testnet (chainId 10143).
//
// UX: the agent ID is auto-generated (random uint256) with a regenerate
// button; capabilities are picked via checkboxes instead of hand-written
// JSON. An advanced collapse keeps raw JSON editing for power users.

import { useState } from "react";
import { Loader2, UserPlus, RefreshCw } from "lucide-react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt } from "wagmi";
import { monadTestnet } from "wagmi/chains";
import { REGISTRY_ADDRESS, REGISTRY_ABI, contractsReady } from "@/lib/contracts";

interface Props {
  onRegistered?: (agentId: string) => void;
}

// Preset capabilities a registering agent can claim. Each maps to a tag that
// the marketplace capability filter (agentCategory) understands.
const CAPABILITY_OPTIONS = [
  { key: "defi", label: "DeFi (analysis / trading / portfolio)" },
  { key: "data", label: "Data (labeling / processing / pipelines)" },
  { key: "translation", label: "Translation (i18n / content)" },
  { key: "eval", label: "Evaluation (validation / QA)" },
  { key: "audit", label: "Audit (security / code review)" },
  { key: "coding", label: "Coding (generation / review / refactor)" },
];

// Prompt template a user can hand to their agent so it generates its own
// capability description — then paste it into the advanced JSON box.
const PROMPT_TEMPLATE = `请描述你的 agent 能力，输出 JSON（不要 markdown），格式：
{"tags": ["defi", "data"], "description": "一句话描述"}。

只输出 JSON 本体。`;

function randomAgentId(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return "0x" + Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function AgentRegisterForm({ onRegistered }: Props) {
  const { address, chain } = useAccount();
  const { writeContractAsync, isPending: isWriting } = useWriteContract();
  const [agentId, setAgentId] = useState(randomAgentId);
  const [caps, setCaps] = useState<string[]>([]);
  const [endpoint, setEndpoint] = useState("");
  const [advancedJson, setAdvancedJson] = useState("");
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [txHash, setTxHash] = useState<`0x${string}` | null>(null);
  const [error, setError] = useState<string | null>(null);

  const { isLoading: isConfirming, isSuccess: isConfirmed } = useWaitForTransactionReceipt({
    hash: txHash ?? undefined,
  });

  const ready = contractsReady() && REGISTRY_ADDRESS !== undefined;
  const wrongChain = chain && chain.id !== monadTestnet.id;

  function toggleCap(key: string) {
    setCaps((prev) => (prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key]));
  }

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
    // Capabilities: checkbox picks are assembled automatically; the advanced
    // JSON box (when filled) overrides them.
    let capsPayload: unknown;
    if (advancedJson.trim()) {
      try {
        capsPayload = JSON.parse(advancedJson);
      } catch {
        setError("Advanced capabilities must be valid JSON.");
        return;
      }
    } else {
      capsPayload = { tags: caps };
    }
    const metaJson = JSON.stringify({ endpointUrl: endpoint, capabilities: capsPayload });
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
        <label className="mb-1 block text-sm font-medium text-white/80">
          Agent ID <span className="text-white/30">(auto-generated, editable)</span>
        </label>
        <div className="flex gap-2">
          <input
            type="text"
            required
            value={agentId}
            onChange={(e) => setAgentId(e.target.value)}
            placeholder="0x-hex or decimal"
            className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-sm text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
          />
          <button
            type="button"
            onClick={() => setAgentId(randomAgentId())}
            title="Generate a new random agent ID"
            className="inline-flex shrink-0 items-center gap-1.5 rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 text-xs text-white/70 hover:border-prism-accent/50 hover:text-prism-accent"
          >
            <RefreshCw className="h-3.5 w-3.5" /> Generate
          </button>
        </div>
        <p className="mt-1 text-[11px] text-white/40">
          Each agent gets a unique on-chain ID. You can keep the generated one or use your own.
        </p>
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
        <p className="mt-1 text-[11px] text-white/40">Where buyers send invoke requests.</p>
      </div>

      <div>
        <label className="mb-1 block text-sm font-medium text-white/80">Capabilities</label>
        <div className="grid gap-1.5 sm:grid-cols-2">
          {CAPABILITY_OPTIONS.map((opt) => (
            <label
              key={opt.key}
              className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs transition-colors ${
                caps.includes(opt.key)
                  ? "border-prism-accent/60 bg-prism-accent/10 text-white"
                  : "border-white/10 bg-prism-surface/60 text-white/60 hover:border-white/20"
              }`}
            >
              <input
                type="checkbox"
                checked={caps.includes(opt.key)}
                onChange={() => toggleCap(opt.key)}
                className="h-3.5 w-3.5 accent-prism-accent"
              />
              {opt.label}
            </label>
          ))}
        </div>
        <p className="mt-1 text-[11px] text-white/40">
          Picked capabilities are assembled into the on-chain metadata automatically.
        </p>
      </div>

      <details
        open={showAdvanced}
        onToggle={(e) => setShowAdvanced((e.target as HTMLDetailsElement).open)}
        className="rounded-lg border border-white/10 bg-black/20 p-3"
      >
        <summary className="cursor-pointer text-xs text-white/50 hover:text-white">
          Advanced: raw capabilities JSON / prompt template
        </summary>
        <div className="mt-2 space-y-2">
          <p className="text-[11px] text-white/40">
            Hand this prompt to your agent and paste its JSON output below (overrides checkboxes):
          </p>
          <pre className="rounded-md bg-black/40 p-2 text-[11px] text-white/60">
{PROMPT_TEMPLATE}
          </pre>
          <textarea
            value={advancedJson}
            onChange={(e) => setAdvancedJson(e.target.value)}
            placeholder='{"tags":["defi"],"description":"..."}'
            rows={3}
            className="w-full rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 font-mono text-xs text-white placeholder:text-white/30 focus:border-prism-accent focus:outline-none"
          />
        </div>
      </details>

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
