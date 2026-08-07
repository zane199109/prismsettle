# Reputation history endpoint.
# GET /api/v1/prismsettle/reputation/history?agentId=0x3333&limit=30

import logging
from typing import Optional

from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel

from ..database import get_pool, score_display, TimedQuery

logger = logging.getLogger("prismsettle.api.reputation")

router = APIRouter(tags=["reputation"])


# ── Response models ──────────────────────────────────────────────────────────


class ReputationPoint(BaseModel):
    score: str
    score_display: str
    event_type: str
    block_time: int
    tx_hash: str


class ReputationHistoryResponse(BaseModel):
    agent_id: str
    history: list[ReputationPoint]


# ── Routes ───────────────────────────────────────────────────────────────────


@router.get("/reputation/history", response_model=ReputationHistoryResponse)
async def get_reputation_history(
    agent_id: str = Query(..., description="Agent ID in hex, e.g. 0x3333"),
    limit: int = Query(30, ge=1, le=200),
    chain_name: str = Query("monad_testnet"),
    contract_addr: Optional[str] = Query(None),
):
    """
    Return reputation history time series for an agent.

    Queries Aggregated, ValidationSubmitted, and Slashed events from
    chain_events, ordered by block time descending. Frontend uses the
    result to plot a reputation curve.
    """
    logger.info("reputation_history agent=%s limit=%d chain=%s contract=%s",
                agent_id, limit, chain_name, contract_addr or "none")

    pool = await get_pool()

    event_types = [
        "PRISM_AGGREGATED",
        "PRISM_VALIDATION_SUBMITTED",
        "PRISM_SLASHED",
    ]

    # ── Build query with optional contract filter ────────────────────────────
    params = dict(agent_id=agent_id, chain_name=chain_name, limit=limit)
    if contract_addr:
        params["contract_addr"] = contract_addr

    async with TimedQuery(pool, "reputation_history", **params) as conn:
        if contract_addr:
            rows = await conn.fetch(
                """
                SELECT value, event_type, block_time, tx_hash
                FROM chain_events
                WHERE chain_name = $1
                  AND event_type = ANY($2::text[])
                  AND LOWER(to_addr) = LOWER($3)
                  AND LOWER(contract) = LOWER($4)
                ORDER BY block_time DESC, block_number DESC
                LIMIT $5
                """,
                chain_name, event_types, agent_id, contract_addr, limit,
            )
        else:
            rows = await conn.fetch(
                """
                SELECT value, event_type, block_time, tx_hash
                FROM chain_events
                WHERE chain_name = $1
                  AND event_type = ANY($2::text[])
                  AND LOWER(to_addr) = LOWER($3)
                ORDER BY block_time DESC, block_number DESC
                LIMIT $4
                """,
                chain_name, event_types, agent_id, limit,
            )

    logger.info("reputation_history agent=%s rows=%d", agent_id, len(rows))

    history = [
        ReputationPoint(
            score=r["value"] or "0",
            score_display=score_display(r["value"] or "0"),
            event_type=r["event_type"],
            block_time=r["block_time"],
            tx_hash=r["tx_hash"],
        )
        for r in rows
    ]

    # Log score range for quick diagnostics
    if history:
        scores = [int(p.score) for p in history if p.score]
        if scores:
            logger.debug("reputation_history agent=%s min=%s max=%s points=%d",
                         agent_id, score_display(str(min(scores))),
                         score_display(str(max(scores))), len(history))

    return ReputationHistoryResponse(agent_id=agent_id, history=history)