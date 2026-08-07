# Agent list and detail endpoints.
# GET /api/v1/prismsettle/agents
# GET /api/v1/prismsettle/agents/:agentId

import logging
from typing import Optional

from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel

from ..database import get_pool, score_display, shard_of, TimedQuery

logger = logging.getLogger("prismsettle.api.agents")

router = APIRouter(tags=["agents"])


# ── Response models ──────────────────────────────────────────────────────────


class AgentVO(BaseModel):
    agent_id: str
    owner: str
    name: str
    description: str
    capabilities: list[str]
    endpoint: str
    score: str
    score_display: str
    registered_at: int
    tx_count: int
    shard: int


class AgentListResponse(BaseModel):
    data: list[AgentVO]
    total: int
    page: int
    size: int


# ── Helpers ──────────────────────────────────────────────────────────────────


def _parse_metadata(metadata: Optional[str]) -> dict:
    """Parse metadata JSON with fallback to empty dict."""
    if not metadata:
        return {}
    import json
    try:
        return json.loads(metadata)
    except (json.JSONDecodeError, TypeError):
        logger.warning("parse_metadata: invalid JSON metadata=%s", str(metadata)[:128])
        return {}


async def _latest_score(agent_id: str, chain_name: str) -> str:
    """Fetch the latest Aggregated score for an agent."""
    pool = await get_pool()
    async with TimedQuery(pool, "latest_score", agent_id=agent_id, chain_name=chain_name) as conn:
        row = await conn.fetchrow(
            """
            SELECT value FROM chain_events
            WHERE chain_name = $1
              AND event_type = 'PRISM_AGGREGATED'
              AND LOWER(to_addr) = LOWER($2)
            ORDER BY block_number DESC, log_index DESC
            LIMIT 1
            """,
            chain_name, agent_id,
        )
        score = row["value"] if row else "0"
        logger.debug("latest_score agent=%s score=%s found=%s", agent_id, score, row is not None)
        return score


async def _tx_count(agent_id: str, chain_name: str) -> int:
    """Count ValidationSubmitted events for an agent."""
    pool = await get_pool()
    async with TimedQuery(pool, "tx_count", agent_id=agent_id, chain_name=chain_name) as conn:
        row = await conn.fetchrow(
            """
            SELECT COUNT(*) AS cnt FROM chain_events
            WHERE chain_name = $1
              AND event_type = 'PRISM_VALIDATION_SUBMITTED'
              AND LOWER(to_addr) = LOWER($2)
            """,
            chain_name, agent_id,
        )
        cnt = row["cnt"] if row else 0
        logger.debug("tx_count agent=%s count=%d", agent_id, cnt)
        return cnt


# ── Routes ───────────────────────────────────────────────────────────────────


@router.get("/agents", response_model=AgentListResponse)
async def list_agents(
    page: int = Query(1, ge=1),
    size: int = Query(20, ge=1, le=100),
    chain_name: str = Query("monad_testnet"),
):
    """Return paginated agent list, enriched with latest score and tx count."""
    logger.info("list_agents page=%d size=%d chain=%s", page, size, chain_name)
    pool = await get_pool()

    # ── Count total ──────────────────────────────────────────────────────────
    async with TimedQuery(pool, "agent_count", chain_name=chain_name) as conn:
        total_row = await conn.fetchrow(
            "SELECT COUNT(*) AS cnt FROM agent_registry WHERE chain_name = $1",
            chain_name,
        )
    total = total_row["cnt"] if total_row else 0
    logger.info("agent_count total=%d", total)

    if total == 0:
        return AgentListResponse(data=[], total=0, page=page, size=size)

    # ── Fetch page ───────────────────────────────────────────────────────────
    offset = (page - 1) * size
    async with TimedQuery(pool, "agent_list", chain_name=chain_name, page=page, size=size, offset=offset) as conn:
        rows = await conn.fetch(
            """
            SELECT agent_id, owner, metadata, endpoint, registered_at
            FROM agent_registry
            WHERE chain_name = $1
            ORDER BY registered_at DESC
            LIMIT $2 OFFSET $3
            """,
            chain_name, size, offset,
        )
    logger.info("agent_list rows=%d page=%d", len(rows), page)

    # ── Enrich each agent ────────────────────────────────────────────────────
    data: list[AgentVO] = []
    for i, r in enumerate(rows):
        meta = _parse_metadata(r["metadata"])
        score = await _latest_score(r["agent_id"], chain_name)
        tx_count = await _tx_count(r["agent_id"], chain_name)

        vo = AgentVO(
            agent_id=r["agent_id"],
            owner=r["owner"] or "",
            name=meta.get("name", r["agent_id"]),
            description=meta.get("description", ""),
            capabilities=meta.get("capabilities", []),
            endpoint=r["endpoint"] or "",
            score=score,
            score_display=score_display(score),
            registered_at=r["registered_at"],
            tx_count=tx_count,
            shard=shard_of(r["agent_id"]),
        )
        data.append(vo)
        logger.debug("agent[%d] id=%s name=%s score=%s tx=%d",
                     i, vo.agent_id, vo.name, vo.score_display, vo.tx_count)

    return AgentListResponse(data=data, total=total, page=page, size=size)


@router.get("/agents/{agent_id}", response_model=AgentVO)
async def get_agent(
    agent_id: str,
    chain_name: str = Query("monad_testnet"),
):
    """Return a single agent by ID, enriched with score and tx count."""
    logger.info("get_agent agent_id=%s chain=%s", agent_id, chain_name)
    pool = await get_pool()

    # ── Fetch agent row ──────────────────────────────────────────────────────
    async with TimedQuery(pool, "agent_by_id", agent_id=agent_id, chain_name=chain_name) as conn:
        row = await conn.fetchrow(
            """
            SELECT agent_id, owner, metadata, endpoint, registered_at
            FROM agent_registry
            WHERE chain_name = $1 AND LOWER(agent_id) = LOWER($2)
            """,
            chain_name, agent_id,
        )
    if not row:
        logger.warning("agent_not_found agent_id=%s", agent_id)
        raise HTTPException(status_code=404, detail="agent not found")

    # ── Enrich ───────────────────────────────────────────────────────────────
    meta = _parse_metadata(row["metadata"])
    score = await _latest_score(row["agent_id"], chain_name)
    tx_count = await _tx_count(row["agent_id"], chain_name)

    logger.info("agent_found id=%s name=%s score=%s tx=%d",
                row["agent_id"], meta.get("name", row["agent_id"]), score_display(score), tx_count)

    return AgentVO(
        agent_id=row["agent_id"],
        owner=row["owner"] or "",
        name=meta.get("name", row["agent_id"]),
        description=meta.get("description", ""),
        capabilities=meta.get("capabilities", []),
        endpoint=row["endpoint"] or "",
        score=score,
        score_display=score_display(score),
        registered_at=row["registered_at"],
        tx_count=tx_count,
        shard=shard_of(row["agent_id"]),
    )