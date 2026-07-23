"use client";

// Theme provider — wraps next-themes for dark/light mode switching.
// Dark mode is the default for PrismSettle (Web3 dashboard).

import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ThemeProviderProps } from "next-themes";

export function ThemeProvider({ children, ...props }: ThemeProviderProps) {
  return <NextThemesProvider {...props}>{children}</NextThemesProvider>;
}
