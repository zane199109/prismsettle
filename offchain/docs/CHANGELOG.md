# Changelog

All notable changes to this project will be documented in this file.

## [v0.1.0] - 2026-06-23

### Initial Release

#### Backend - EVM Off-Chain Event Listener
- Go 1.22 + Gin + GORM + PostgreSQL + Redis architecture
- Plugin-based EVM event parser (ERC20, ERC721, custom ABIs)
- Multi-chain RPC polling with round-robin load balancing
- Batch DB writes with Redis distributed locking
- Graceful shutdown with context cancellation
- Health check endpoint (/health)
- ERC20 transfer query API with pagination
- Token balance query (native + ERC20)
- LLM-powered transaction explanation (test endpoint)
- Structured logging with zap (log rotation)
- Docker deployment ready

## [v0.2.0] - 2026-06-23

### Documentation

#### Added
- `docs/GO-INTERVIEW-QUESTIONS.md`: Complete Go backend interview Q&A (50 questions, bilingual CN/EN)
  - Part 1: Go Language Basics (Q1-Q8)
  - Part 2: Concurrency (Q9-Q13)
  - Part 3: Database/PostgreSQL (Q14-Q17)
  - Part 4: Redis (Q18-Q21)
  - Part 5: Blockchain/Web3 (Q22-Q27)
  - Part 6: System Design (Q28-Q31)
  - Part 7: Project Deep Dive (Q32-Q36)
  - Part 8: Algorithm Questions (Q37-Q43)
  - Part 9: English Interview Scripts (Q44-Q50)

#### Documentation
- Project README with architecture overview
- CHANGELOG tracking all changes
- API documentation for endpoints
