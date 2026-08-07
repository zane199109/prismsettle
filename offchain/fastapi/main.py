# PrismSettle FastAPI Query Service
#
# Read-only REST API over the prismsettle PostgreSQL database.
# Designed to co-exist with the Go offchain service:
#   - Go offchain: indexer, evaluator, keeper, event ingestion
#   - FastAPI:     read queries (agents, reputation history, jobs)
#
# Usage:
#   pip install -r requirements.txt
#   python main.py
#
# Docker:
#   docker build -t prismsettle/fastapi -f Dockerfile .

import logging
import time

import uvicorn
from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware

from .routes import agents, reputation

# ── Root logger ──────────────────────────────────────────────────────────────

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s level=%(levelname)s module=%(name)s %(message)s",
    datefmt="%Y-%m-%dT%H:%M:%S",
)
logger = logging.getLogger("prismsettle.api")

# ── App ──────────────────────────────────────────────────────────────────────

app = FastAPI(
    title="PrismSettle Query API",
    version="1.0.0",
    description="Read-only REST API for PrismSettle agent data",
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["GET"],
    allow_headers=["*"],
)

app.include_router(agents.router, prefix="/api/v1/prismsettle")
app.include_router(reputation.router, prefix="/api/v1/prismsettle")


# ── Request timing middleware ─────────────────────────────────────────────────

@app.middleware("http")
async def request_timing(request: Request, call_next):
    start = time.monotonic()
    response = await call_next(request)
    elapsed = (time.monotonic() - start) * 1000
    logger.info(
        "request method=%s path=%s status=%d duration_ms=%.1f",
        request.method, request.url.path, response.status_code, elapsed,
    )
    return response


# ── Health ────────────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    return {"status": "ok"}


# ── Entry ─────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    with uvicorn.run("offchain.fastapi.main:app", host="0.0.0.0", port=9200, reload=True)  # type: ignore[attr-defined]
        pass