"use client";

// Pagination — previous/next + total counter for list pages.

import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";

interface Props {
  page: number;
  size: number;
  total: number;
  onPageChange: (page: number) => void;
}

export function Pagination({ page, size, total, onPageChange }: Props) {
  const totalPages = Math.max(1, Math.ceil(total / Math.max(1, size)));
  if (totalPages <= 1 && total <= size) {
    return (
      <div className="flex items-center justify-between px-1 pt-3 text-[11px] text-white/40">
        <span>共 {total} 条</span>
      </div>
    );
  }
  return (
    <div className="flex items-center justify-between px-1 pt-3">
      <span className="text-[11px] text-white/40">
        共 {total} 条 · 第 {page} / {totalPages} 页
      </span>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={page <= 1}
          onClick={() => onPageChange(page - 1)}
          className="h-7 gap-1 border-white/15 bg-transparent px-2 text-[11px] text-white/70 hover:bg-white/10"
        >
          <ChevronLeft className="h-3 w-3" /> 上一页
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
          className="h-7 gap-1 border-white/15 bg-transparent px-2 text-[11px] text-white/70 hover:bg-white/10"
        >
          下一页 <ChevronRight className="h-3 w-3" />
        </Button>
      </div>
    </div>
  );
}
