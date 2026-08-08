"use client";

// Custom wallet connect control styled to the project's prism theme.
// Uses plain wagmi + injected connector (browser extension wallet) — no
// WalletConnect Cloud / RainbowKit remote config, so it works fully offline
// and never hits api.web3modal.org (the placeholder projectId 403s).

import { useAccount, useChainId, useConnect, useDisconnect, useSwitchChain } from "wagmi";
import { injected } from "wagmi/connectors";
import { monadTestnet } from "wagmi/chains";
import { Wallet } from "lucide-react";
import { cn } from "@/lib/utils";

function shortAddr(a: string | undefined): string {
  if (!a) return "";
  return `${a.slice(0, 6)}…${a.slice(-4)}`;
}

export function WalletConnect() {
  const { address, isConnected } = useAccount();
  const chainId = useChainId();
  const { connect, isPending } = useConnect();
  const { disconnect } = useDisconnect();
  const { switchChain, isPending: isSwitching } = useSwitchChain();

  const isMonad = chainId === monadTestnet.id;
  const chainLabel = isMonad ? "Monad Testnet" : chainId ? `Unknown (${chainId})` : "Unknown chain";

  if (!isConnected || !address) {
    return (
      <button
        onClick={() => connect({ connector: injected() })}
        disabled={isPending}
        className="inline-flex items-center gap-2 rounded-lg bg-prism-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-prism-accent/80 disabled:opacity-60"
      >
        <Wallet className="h-4 w-4" />
        {isPending ? "Connecting…" : "Connect Wallet"}
      </button>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <span
        className={cn(
          "hidden rounded-lg border px-3 py-2 text-xs sm:inline-flex",
          isMonad
            ? "border-white/10 bg-prism-surface/60 text-white/60"
            : "border-prism-warn/40 bg-prism-warn/10 text-prism-warn",
        )}
      >
        {chainLabel}
      </span>
      {!isMonad && (
        <button
          onClick={() => switchChain({ chainId: monadTestnet.id })}
          disabled={isSwitching}
          className="inline-flex items-center gap-1.5 rounded-lg border border-prism-warn/50 bg-prism-warn/10 px-3 py-2 text-xs font-semibold text-prism-warn transition-colors hover:bg-prism-warn/20 disabled:opacity-60"
        >
          {isSwitching ? "Switching…" : "Switch to Monad"}
        </button>
      )}
      <button
        onClick={() => disconnect()}
        title="Disconnect"
        className="inline-flex items-center gap-2 rounded-lg border border-prism-accent/40 bg-prism-accent/10 px-4 py-2 font-mono text-sm text-white transition-colors hover:bg-prism-accent/20"
      >
        <Wallet className="h-4 w-4 text-prism-accent" />
        {shortAddr(address)}
      </button>
    </div>
  );
}
