"use client";

// PrismHologram — the signature visual of PrismSettle.
// A triangular prism rendered with CSS 3D transforms that slowly rotates,
// splitting a "white light" beam into a spectrum. The aggregate validation
// score floats in the center.
//
// Why CSS instead of three.js:
//   - Lighter bundle (~0 KB vs ~600 KB for three)
//   - Sufficient for the visual metaphor (prism = project name)
//   - No WebGL context to manage on SSR
//
// The spectrum below the prism maps to the A/B/C/D grade colors so the
// visual identity (SD §4.6) is reinforced every time the user sees it.

import { motion } from "framer-motion";
import { formatScore, gradeColor, gradeFromScore } from "@/lib/utils";

interface PrismHologramProps {
  // Aggregate score as a decimal string (uint256 scaled by 1e18).
  score?: string;
  // Size in pixels. Default 256.
  size?: number;
  // Optional label under the score (e.g. "Aggregate Score").
  label?: string;
}

export function PrismHologram({
  score = "0",
  size = 256,
  label,
}: PrismHologramProps) {
  const display = formatScore(score);
  const grade = gradeFromScore(score);
  const gradeCls = gradeColor(grade);

  return (
    <div
      className="relative"
      style={{ width: size, height: size, perspective: 1000 }}
    >
      {/* Glow halo — pulses to signal "live" data */}
      <motion.div
        className="absolute inset-0 rounded-full bg-prism-accent/20 blur-3xl"
        animate={{ opacity: [0.4, 0.7, 0.4], scale: [1, 1.05, 1] }}
        transition={{ duration: 3, repeat: Infinity, ease: "easeInOut" }}
      />

      {/* Rotating prism — 3 triangular faces at 0°/120°/240° */}
      <motion.div
        className="relative h-full w-full"
        style={{ transformStyle: "preserve-3d" }}
        animate={{ rotateY: 360 }}
        transition={{ duration: 20, repeat: Infinity, ease: "linear" }}
      >
        {[0, 120, 240].map((rotateY) => (
          <div
            key={rotateY}
            className="absolute inset-0 border border-prism-accent/40 bg-gradient-to-b from-prism-accent/10 to-prism-glow/10 backdrop-blur-sm"
            style={{
              transform: `rotateY(${rotateY}deg) translateZ(${size * 0.31}px)`,
              clipPath: "polygon(50% 0%, 100% 100%, 0% 100%)",
            }}
          />
        ))}
      </motion.div>

      {/* Spectrum projection below the prism — refracted light band */}
      <div
        className="absolute left-1/2 -translate-x-1/2 h-2 w-48 rounded-full opacity-60 blur-md"
        style={{
          bottom: -size * 0.05,
          background:
            "linear-gradient(90deg, #ef4444 0%, #f59e0b 33%, #10b981 66%, #7c3aed 100%)",
        }}
      />

      {/* Center score overlay (does NOT rotate, stays readable) */}
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span
          className="font-mono text-4xl font-bold text-white"
          style={{
            textShadow: "0 0 12px rgba(124,58,237,0.8)",
          }}
        >
          {display}
        </span>
        {label && (
          <span className="mt-1 text-xs uppercase tracking-wider text-white/50">
            {label}
          </span>
        )}
        <span className={`mt-1 text-xs font-semibold ${gradeCls}`}>
          Grade {grade}
        </span>
      </div>
    </div>
  );
}
