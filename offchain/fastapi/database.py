# Database connection — asyncpg pool, configured via env vars.
# Centralized logger for query timing and diagnostics.

import logging
import os
import time
from typing import Optional

import asyncpg

# ── Logger ───────────────────────────────────────────────────────────────────

logger = logging.getLogger("prismsettle.db")
logger.setLevel(logging.DEBUG)

# Console handler (structured key=value format for log aggregation)
_handler = logging.StreamHandler()
_handler.setFormatter(
    logging.Formatter(
        fmt="%(asctime)s level=%(levelname)s module=%(name)s %(message)s",
        datefmt="%Y-%m-%dT%H:%M:%S",
    )
)
logger.addHandler(_handler)

# ── Connection pool ──────────────────────────────────────────────────────────

DSN = os.getenv(
    "PRISM_DB_DSN",
    "postgresql://prism:prism@2026@localhost:5433/prismsettle",
)
_MIN_POOL = int(os.getenv("PRISM_DB_POOL_MIN", "2"))
_MAX_POOL = int(os.getenv("PRISM_DB_POOL_MAX", "10"))

_pool: Optional[asyncpg.Pool] = None
_pool_stats = {"acquired": 0, "released": 0, "wait_ms": 0.0}


async def get_pool() -> asyncpg.Pool:
    global _pool
    if _pool is None:
        logger.info(
            "creating pool dsn=%s min=%d max=%d",
            DSN.replace(DSN.split("@")[0] if "@" in DSN else "", "***"),
            _MIN_POOL,
            _MAX_POOL,
        )
        _pool = await asyncpg.create_pool(DSN, min_size=_MIN_POOL, max_size=_MAX_POOL)
        logger.info("pool created size=%d", _pool.get_size() if hasattr(_pool, "get_size") else 0)
    return _pool


async def close_pool() -> None:
    global _pool
    if _pool is not None:
        logger.info("closing pool acquired=%d released=%d wait_ms=%.1f",
                     _pool_stats["acquired"], _pool_stats["released"], _pool_stats["wait_ms"])
        await _pool.close()
        _pool = None


# ── Query timing decorator ───────────────────────────────────────────────────


class TimedQuery:
    """Context manager that logs query execution time and result size."""

    def __init__(self, pool: asyncpg.Pool, query_name: str, **params):
        self.pool = pool
        self.query_name = query_name
        self.params = params
        self.start = 0.0
        self.conn: Optional[asyncpg.Connection] = None

    async def __aenter__(self) -> asyncpg.Connection:
        t0 = time.monotonic()
        self.conn = await self.pool.acquire()
        elapsed = (time.monotonic() - t0) * 1000
        _pool_stats["acquired"] += 1
        _pool_stats["wait_ms"] += elapsed
        self.start = time.monotonic()
        logger.debug(
            "query=%s pool_wait_ms=%.1f params=%s",
            self.query_name, elapsed, _fmt_params(self.params),
        )
        return self.conn

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        elapsed = (time.monotonic() - self.start) * 1000
        if self.conn:
            await self.pool.release(self.conn)
            _pool_stats["released"] += 1
        if exc_type is None:
            logger.info(
                "query=%s duration_ms=%.1f params=%s",
                self.query_name, elapsed, _fmt_params(self.params),
            )
        else:
            logger.error(
                "query=%s duration_ms=%.1f error=%s params=%s",
                self.query_name, elapsed, exc_val, _fmt_params(self.params),
            )


def _fmt_params(params: dict) -> str:
    """Format query params for logging — mask sensitive values, truncate long strings."""
    parts = []
    for k, v in params.items():
        if k in ("agent_id", "to_addr", "contract_addr"):
            v = str(v)[:42]  # hex address truncation
        elif isinstance(v, str) and len(v) > 64:
            v = v[:64] + "..."
        parts.append(f"{k}={v}")
    return " ".join(parts)


# ── Helpers ──────────────────────────────────────────────────────────────────


def score_display(raw: str) -> str:
    """Convert 18-decimal raw score to human-readable string, e.g. '0.90'."""
    if not raw:
        return "0.00"
    try:
        val = int(raw)
        whole = val // 10**18
        frac = (val % 10**18) // 10**16
        return f"{whole}.{frac:02d}"
    except (ValueError, TypeError):
        logger.warning("score_display: unparseable raw=%s", raw)
        return "0.00"


def shard_of(agent_id: str) -> int:
    """Compute shard slot from agent ID: agent_id & 0xFF."""
    try:
        return int(agent_id, 16) & 0xFF
    except (ValueError, TypeError):
        logger.warning("shard_of: unparseable agent_id=%s", agent_id)
        return 0