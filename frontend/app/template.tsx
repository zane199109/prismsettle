"use client";

// Page transition wrapper — runs on every navigation.
// Uses Framer Motion for smooth opacity + translateY transitions.
// Respects prefers-reduced-motion via Framer's built-in support.

import { motion } from "framer-motion";

export default function Template({ children }: { children: React.ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.2, ease: "easeOut" }}
    >
      {children}
    </motion.div>
  );
}
