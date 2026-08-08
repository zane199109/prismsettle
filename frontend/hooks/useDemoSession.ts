"use client";

// useDemoSession — drives the chat-style collaboration demo on the job
// detail page: create a session, then poll the message stream every 3s.

import { useState } from "react";
import { usePoll } from "./usePoll";
import {
  createDemoSession,
  getDemoMessages,
  getDemoSession,
  getDemoSessionByJob,
  type DemoMessageVO,
  type DemoSessionVO,
} from "@/lib/prismsettle";

export function useDemoSession() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const start = async (params: {
    title: string;
    description?: string;
    amount: string;
    provider_agent?: string;
    scenario?: string;
  }) => {
    setCreating(true);
    setCreateError(null);
    try {
      const res = await createDemoSession(params);
      setSessionId(res.session_id);
      return res.session_id;
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : "创建演示失败");
      return null;
    } finally {
      setCreating(false);
    }
  };

  // Load the most recent demo session for a job (history view on the detail
  // page). Leaves the session unset when the job was never demo-driven.
  const loadByJob = async (jobId: string) => {
    try {
      const res = await getDemoSessionByJob(jobId);
      if (res.session?.session_id) setSessionId(res.session.session_id);
    } catch {
      // ignore — just show the start panel
    }
  };

  // Poll the session summary (state) once a session exists.
  const { data: session } = usePoll<DemoSessionVO | null>(
    sessionId ? `demo-session:${sessionId}` : null,
    async () => (sessionId ? getDemoSession(sessionId) : null),
    { intervalMs: 4000 },
  );

  // Poll the message stream.
  const { data: messages } = usePoll<DemoMessageVO[]>(
    sessionId ? `demo-messages:${sessionId}` : null,
    async () => {
      const res = await getDemoMessages(sessionId!);
      return res.messages ?? [];
    },
    { intervalMs: 3000 },
  );

  const msgList = messages ?? [];
  // Live state: the last non-system-note message's job state (the chain of
  // states in the script), falling back to the session summary. This follows
  // the flow step-by-step instead of jumping only at the end.
  const lastActionMsg = [...msgList].reverse().find((m) => m.action !== "system_note" && m.state);
  const liveState = lastActionMsg?.state ?? session?.state ?? null;
  const rejectCount = msgList.filter((m) => m.action === "reject").length;

  return {
    sessionId,
    session: session ?? null,
    messages: msgList,
    liveState,
    rejectCount,
    creating,
    createError,
    start,
    loadByJob,
    reset: () => {
      setSessionId(null);
      setCreateError(null);
    },
  };
}
