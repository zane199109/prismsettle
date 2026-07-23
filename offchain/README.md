# web3-offchain

EVM blockchain event indexer service built with Go. Listens to on-chain contract events (ERC20, FundMe, NFT), parses them, persists to PostgreSQL with Redis caching, and exposes REST APIs for querying.

Designed for production use with reorg handling, batch writes, and graceful shutdown.

## Architecture

```
                          ┌─────────────┐
                          │   Blockchain│
                          │   (EVM)       │
                          └──────┬──────┘
                                 │ RPC/WebSocket
                    ┌────────────▼────────────┐
                    │     EVM Listener        │
                    │  (Ordered block sync +  │
                    │   Reorg rollback)       │
                    └────────────┬────────────┘
                                 │ Raw Logs
              ┌──────────────────▼──────────────────┐
              │         Event Parser Registry        │
              │  ┌────────┐ ┌────────┐ ┌──────────┐ │
              │  │ ERC20  │ │ FundMe │ │  Custom  │ │
              │  └────────┘ └────────┘ └──────────┘ │
              └──────────────────┬──────────────────┘
                                 │ Parsed Events
                    ┌────────────▼────────────┐
                    │   Event Ingest Service   │
                    │  (Batch save + Idempotency│
                    │   + Reorg rollback)      │
                    └────────────┬────────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                   │
    ┌─────────▼────────┐ ┌──────▼──────┐  ┌─────────▼────────┐
    │   PostgreSQL     │ │   Redis     │  │  Telegram Notify │
    │  (Chain events,  │ │ (Stats      │  │  (Alerts)        │
    │   block states)  │ │  cache)     │  │                  │
    └──────────────────┘ └─────────────┘  └──────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │    REST API (Gin)        │
                    │  GET /api/v1/erc20/*     │
                    │  GET /api/v1/fundme/*    │
                    │  (Rate limit + Auth)     │
                    └──────────────────────────┘
```

## Features

- **EVM event listening**: EVM chain event subscription with unified `ChainListener` interface
- **Reorg handling**: Ordered block processing with chain reorganization rollback
- **Plugin-based parsing**: Global parser registry — add new contract types via `init()` registration
- **Production-grade storage**: Batch inserts (200/batch) with ON CONFLICT idempotency, connection pooling
- **Caching strategy**: Cache-aside pattern for stats aggregation; direct DB for paginated lists
- **Distributed locking**: Redis-based lock for multi-instance sync safety
- **Graceful shutdown**: Signal handling for clean stop of listeners and HTTP server
- **Multi-env config**: Viper-based YAML config (dev/prod) with hot-reload support
- **Security**: Token-based auth, per-token/IP rate limiting, non-root Docker user
- **Observability**: zap structured logging, pprof profiling, JSON file rotation

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.22 |
| Web Framework | Gin |
| ORM | GORM (PostgreSQL) |
| Database | PostgreSQL 14 |
| Cache | Redis 7 |
| Blockchain | go-ethereum |
| Config | Viper |
| Logging | Uber Zap |
| Container | Docker + docker-compose (multi-stage build) |

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & docker-compose
- PostgreSQL 14+
- Redis 7+

### Run with Docker (Recommended)

```bash
# Clone and enter project
git clone <your-repo-url>
cd web3-offchain

# Edit config (optional)
# cp config/dev.yaml config/local.yaml
# Edit config/local.yaml with your credentials

# Start all services
docker-compose up -d

# API runs on http://localhost:8080
# Health check: curl http://localhost:8080/health
```

### Run Locally (Development)

```bash
# Start dependencies
docker-compose up -d postgres redis

# Configure dev.yaml with your RPC endpoint
# Edit config/dev.yaml with your credentials

# Run the service
go run ./cmd/main.go -config config/dev.yaml

# Run tests
go test ./...
```

### Build Docker Image

```bash
docker build -t web3-offchain:latest .
docker run -p 9527:9527 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/logs:/app/logs \
  web3-offchain:latest
```

## Project Structure

```
├── cmd/main.go              # Entry point, DI wiring
├── api/                     # External API handlers
├── fundme/                  # FundMe contract module
│   ├── api/                 # Handler
│   ├── service/             # Business logic
│   └── parser/              # Event parser (ABI decoding)
├── internal/
│   ├── api/                 # Core API handlers (ERC20, common)
│   ├── listener/            # Chain listeners (EVM)
│   ├── middleware/          # Auth, rate limit, timeout
│   ├── notify/              # Notification (Telegram)
│   ├── parser/              # Event parser interface + registry
│   ├── repository/          # Data access layer
│   ├── router/              # Route registration
│   ├── service/             # Core business services
│   └── storage/             # DB/Redis connection management
├── model/                   # Domain models
├── pkg/                     # Shared utilities
│   ├── config/              # Config loading
│   ├── constant/            # Constants
│   ├── errno/               # Error codes
│   ├── logger/              # Logger wrapper
│   ├── response/            # API response formatter
│   ├── rpc/                 # RPC client wrapper
│   └── utils/               # Helpers
├── config/                  # Environment configs
│   ├── dev.yaml
│   └── prod.yaml
└── docker-compose.yml       # Dev environment orchestration
```

## API Endpoints

All endpoints under `/api/v1/*` require `X-API-TOKEN` header.

### ERC20

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/erc20/transfers` | Query ERC20 transfer records (paginated) |
| GET | `/api/v1/erc20/balance` | Query token balance for an address |
| GET | `/api/v1/erc20/token/info` | Get token metadata (symbol, decimals) |

**Query params:**
- `chainName` (required) — chain identifier (e.g. "sepolia")
- `address` — wallet address
- `tokenAddr` — token contract address
- `page` / `size` — pagination
- `startTime` / `endTime` — Unix timestamp range

### FundMe

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/fundme/events` | Query FundMe contract events |
| GET | `/api/v1/fundme/stats` | Get FundMe aggregated statistics |

**Query params:**
- `chainName` (required)
- `contract` (required) — FundMe contract address
- `type` — event type filter (comma-separated: FUNDME_FUNDED,FUNDME_REFUND,...)
- `page` / `size` — pagination

### Health Check

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Public health check, no auth required |

## Event Types

| Type | Signature | Description |
|------|-----------|-------------|
| ERC20_TRANSFER | `Transfer(address,address,uint256)` | Standard ERC20 transfer |
| FUNDME_FUNDED | `Funded(address,uint256)` | FundMe contribution |
| FUNDME_REFUND | `RefundByFunder(address,uint256)` | FundMe refund |
| FUNDME_WITHDRAW | `FundWithdrawByOwner(uint256)` | FundMe owner withdrawal |
| NFT_TRANSFER | `Transfer(address,address,uint256)` | NFT transfer (same ABI as ERC20) |

## Adding a New Contract Parser

1. Create a new parser struct implementing `EventParser` interface:

```go
type MyParser struct{}

var MyEventSig = crypto.Keccak256Hash([]byte("MyEvent(address,uint256)"))

func init() {
    parser.RegisterParser("MyContract", &MyParser{})
}

func (p *MyParser) Name() string { return "MyContract" }

func (p *MyParser) Match(log types.Log) bool {
    return len(log.Topics) > 0 && log.Topics[0] == MyEventSig
}

func (p *MyParser) Parse(log types.Log) (any, error) {
    // Decode log topics/data into model.ChainEvent
    return &model.ChainEvent{...}, nil
}
```

2. Import the parser package in `main.go` to trigger `init()` registration.

3. Add API endpoints in a new handler/service module.

## Design Decisions

### Why PostgreSQL over MongoDB?
- Blockchain events are highly structured — relational schema fits naturally
- Need ACID transactions for batch inserts with idempotency
- Complex queries (address-based filtering, time range joins) are easier in SQL
- Better indexing support for time-series data

### Why cache-aside for stats but not for paginated lists?
- Stats (aggregations) are expensive to compute and rarely change — high cache hit ratio
- Paginated lists have low cache hit ratio (different page/size combos) and high invalidation cost
- Direct DB query for paginated lists is the standard production practice

### Reorg handling
- Blocks are processed in strict order via `completedTasks` map
- When a reorg is detected (new block's parent != last finalized), rollback to fork point
- `RollbackEvents` deletes events from the reorg point onward, then re-syncs

## Development

### Run Tests

```bash
go test -v ./...
```

### Profiling

In debug mode, pprof is available at `http://localhost:6060/debug/pprof/`

```bash
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Logging

Logs are written to `./logs/web3-offchain.log` with rotation (100MB per file, keep 3 files).

## Security Notes

- Config files (`dev.yaml`, `prod.yaml`) contain secrets — they are in `.gitignore` and should never be committed
- Use `.env.example` as a template — copy to `.env.local` and fill in your credentials
- Change default API tokens before deployment
- Use HTTPS in production (reverse proxy with nginx/Traefik)
- PostgreSQL and Redis passwords should be managed via environment variables or secrets manager

## License

MIT
