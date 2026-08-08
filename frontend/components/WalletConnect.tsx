"use client";

// Fully custom wallet connect control styled to match the project's
// prism theme (dark glass + violet accent) instead of RainbowKit's
// default look. Same connection logic, project-native appearance.

import { ConnectButton } from "@rainbow-me/rainbowkit";
import { Wallet } from "lucide-react";

export function WalletConnect() {
  return (
    <ConnectButton.Custom>
      {({
        account,
        chain,
        openAccountModal,
        openChainModal,
        openConnectModal,
        authenticationStatus,
        mounted,
      }) => {
        const ready = mounted && authenticationStatus !== "loading";
        const connected =
          ready &&
          account &&
          chain &&
          (!authenticationStatus || authenticationStatus === "authenticated");

        return (
          <div
            {...(!ready && { "aria-hidden": true })}
            className={!ready ? "pointer-events-none select-none opacity-0" : ""}
          >
            {(() => {
              if (!connected) {
                return (
                  <button
                    onClick={openConnectModal}
                    className="inline-flex items-center gap-2 rounded-lg bg-prism-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-prism-accent/80"
                  >
                    <Wallet className="h-4 w-4" />
                    Connect Wallet
                  </button>
                );
              }

              if (chain.unsupported) {
                return (
                  <button
                    onClick={openChainModal}
                    className="inline-flex items-center gap-2 rounded-lg border border-prism-warn/50 bg-prism-warn/10 px-4 py-2 text-sm font-semibold text-prism-warn hover:bg-prism-warn/20"
                  >
                    Wrong network
                  </button>
                );
              }

              return (
                <div className="flex items-center gap-2">
                  <button
                    onClick={openChainModal}
                    className="hidden items-center gap-1.5 rounded-lg border border-white/10 bg-prism-surface/60 px-3 py-2 text-xs text-white/60 hover:border-white/20 sm:inline-flex"
                    title={chain.name}
                  >
                    {chain.hasIcon && (
                      <span
                        className="h-3 w-3 rounded-full"
                        style={{
                          background: chain.iconBackground,
                          backgroundImage: chain.iconUrl,
                        }}
                      />
                    )}
                    {chain.name}
                  </button>
                  <button
                    onClick={openAccountModal}
                    className="inline-flex items-center gap-2 rounded-lg border border-prism-accent/40 bg-prism-accent/10 px-4 py-2 font-mono text-sm text-white transition-colors hover:bg-prism-accent/20"
                  >
                    {account.displayName}
                    {account.displayBalance && (
                      <span className="text-xs text-white/50">
                        {account.displayBalance}
                      </span>
                    )}
                  </button>
                </div>
              );
            })()}
          </div>
        );
      }}
    </ConnectButton.Custom>
  );
}
