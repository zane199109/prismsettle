"use client";

// MetricCard — small stat tile used in the dashboard header row.
// Animated count-up via Framer Motion's useMotionValue + animate() so
// numbers feel "live" instead of jumping when the poll returns.

import { useEffect, useRef } from "react";
import { motion, useMotionValue, useTransform, animate } from "framer-motion";
import { cn } from "@/lib/utils";

interface MetricCardProps {
  label: string;
  value: number | string;
  // Optional delta indicator (e.g. "+12 today"). Renders in green/red.
  delta?: string;
  // When value is numeric, animate from previous to current on change.
  animateNumber?: boolean;
  // Optional icon element (lucide-react) rendered to the left of the label.
  icon?: React.ReactNode;
  className?: string;
}

export function MetricCard({
  label,
  value,
  delta,
  animateNumber = false,
  icon,
  className,
}: MetricCardProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const motionVal = useMotionValue(
    typeof value === "number" ? value : 0,
  );
  const rounded = useTransform(motionVal, (v) => Math.floor(v).toLocaleString());

  useEffect(() => {
    if (!animateNumber || typeof value !== "number") return;
    const controls = animate(motionVal, value, {
      duration: 0.8,
      ease: "easeOut",
    });
    // Subscribe to render the rounded value into the DOM node.
    const unsub = rounded.on("change", (v) => {
      if (ref.current) ref.current.textContent = v;
    });
    return () => {
      controls.stop();
      unsub();
    };
  }, [value, animateNumber, motionVal, rounded]);

  const isPositiveDelta = delta?.trim().startsWith("+");

  return (
    <div
      className={cn(
        "card-glow rounded-xl p-5 backdrop-blur-sm",
        className,
      )}
    >
      <div className="flex items-center gap-2 text-xs uppercase tracking-wider text-white/40">
        {icon}
        <span>{label}</span>
      </div>
      <div className="mt-2 text-2xl font-semibold text-white">
        {animateNumber && typeof value === "number" ? (
          <motion.span ref={ref}>{value.toLocaleString()}</motion.span>
        ) : (
          <span>{value}</span>
        )}
      </div>
      {delta && (
        <div
          className={cn(
            "mt-1 text-xs",
            isPositiveDelta ? "text-emerald-400" : "text-red-400",
          )}
        >
          {delta}
        </div>
      )}
    </div>
  );
}
