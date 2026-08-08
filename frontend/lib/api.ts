// Base fetcher for the PrismSettle backend. All API clients go through this
// so error handling, query serialization, and the {code,message,data} envelope
// are handled in one place.
//
// SD §7.4: the frontend talks to /api/v1/prismsettle/* which next.config.js
// rewrites to the Go offchain service. In production the reverse proxy
// (nginx/ingress) handles routing instead.

import type { ApiEnvelope, Paginated } from "./types";

export class ApiError extends Error {
  code: number;
  constructor(code: number, message: string) {
    super(`[${code}] ${message}`);
    this.code = code;
    this.name = "ApiError";
  }
}

const API_BASE = "/api/v1/prismsettle";

// Internal: perform a fetch and unwrap the envelope. Throws ApiError on
// non-zero code or network failure.
async function request<T>(
  path: string,
  init?: RequestInit & { query?: Record<string, string | number | undefined> },
): Promise<T> {
  const { query, ...rest } = init || {};
  let url = `${API_BASE}${path}`;
  if (query) {
    const qs = Object.entries(query)
      .filter(([, v]) => v !== undefined && v !== "")
      .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
      .join("&");
    if (qs) url += `?${qs}`;
  }

  let resp: Response;
  try {
    resp = await fetch(url, {
      ...rest,
      headers: {
        "Content-Type": "application/json",
        // Backend auth (dev-local.yaml auth.tokens). Overridable via env.
        "X-API-TOKEN": process.env.NEXT_PUBLIC_API_TOKEN || "dev-token",
        ...(rest.headers || {}),
      },
    });
  } catch (e) {
    // Network error / backend unreachable.
    throw new ApiError(-1, `network error: ${(e as Error).message}`);
  }

  if (!resp.ok) {
    throw new ApiError(resp.status, `HTTP ${resp.status}`);
  }

  const env = (await resp.json()) as ApiEnvelope<T>;
  if (env.code !== 0) {
    throw new ApiError(env.code, env.message);
  }
  return env.data;
}

// Public helpers — the rest of the app should use these rather than calling
// `request` directly, so the HTTP method is obvious at the call site.
export const api = {
  get: <T>(path: string, query?: Record<string, string | number | undefined>) =>
    request<T>(path, { method: "GET", query }),

  post: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "POST",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
};

// Convenience wrapper for paginated endpoints. The backend envelope uses
// `data.list` for the array; map it to the typed `items` shape.
export async function getPaginated<T>(
  path: string,
  params: { page?: number; size?: number; chainName?: string; state?: string },
): Promise<Paginated<T>> {
  const data = await api.get<Paginated<T> & { list?: T[] }>(path, params);
  if (data && !data.items && Array.isArray(data.list)) {
    return { items: data.list, total: data.total, page: data.page, size: data.size };
  }
  return data as Paginated<T>;
}
