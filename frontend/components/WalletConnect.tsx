"use client";

import { ConnectButton } from "@rainbow-me/rainbowkit";

// Thin wrapper around RainbowKit's ConnectButton so Phase 8 layout can place
// the wallet connect control anywhere without re-importing the lib.
export function WalletConnect() {
  return <ConnectButton />;
}
