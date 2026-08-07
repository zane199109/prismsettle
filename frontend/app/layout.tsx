import type { Metadata } from "next";
import { Orbitron, Exo_2 } from "next/font/google";
import "./globals.css";
import { Providers } from "./providers";
import { PageHeader } from "@/components/PageHeader";

const orbitron = Orbitron({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-orbitron",
  display: "swap",
});

const exo2 = Exo_2({
  subsets: ["latin"],
  weight: ["300", "400", "500", "600", "700"],
  variable: "--font-exo2",
  display: "swap",
});

export const metadata: Metadata = {
  title: {
    default: "PrismSettle",
    template: "%s | PrismSettle",
  },
  description:
    "PrismSettle — 256-shard reputation registry for AI agents on Monad.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      className={`${orbitron.variable} ${exo2.variable}`}
      suppressHydrationWarning
    >
      <body className="min-h-screen bg-[#0a0a0f]">
        <a href="#main" className="skip-link">
          Skip to main content
        </a>
        <Providers>
          <PageHeader />
          {children}
        </Providers>
      </body>
    </html>
  );
}