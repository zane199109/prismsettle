package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zane/web3-offchain/internal/middleware"
	"github.com/zane/web3-offchain/pkg/config"
)

// HealthChecker is implemented by any service that can report runtime
// health dimensions (sync_lag, reorg_count, evaluator_state, keeper_last_run).
// The router calls Snapshot to build the /health response. nil means the
// dimensions are unknown — the endpoint still returns 200 but with zeroed
// fields, preserving backward compatibility with Phase 7's basic check.
type HealthChecker interface {
	HealthSnapshot() (syncLag, reorgCount, keeperLastRun int64, evaluatorState string)
}

// NewRouter initializes and configures the Gin engine with global middleware,
// route groups, and registers all API route modules.
// It sets up CORS, rate limiting, recovery, authentication, and timeout policies.
//
// health is an optional HealthChecker; pass nil to use the legacy
// {status: "ok"} response (Phase 7 behavior).
func NewRouter(cfg config.Config, routables []Routable, health HealthChecker) http.Handler {
	// Set Gin running mode (release mode for production by default)
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = gin.ReleaseMode
	}
	gin.SetMode(cfg.Server.Mode)

	// Create a Gin engine instance with default middleware (logger + recovery)
	r := gin.Default()

	// Register global middleware applied to all routes
	r.Use(
		middleware.CorsMiddleware(),      // Handle cross-origin requests
		middleware.RateLimitMiddleware(), // Global API rate limiting
		gin.Recovery(),                   // Recover from panic to avoid service crash
	)

	// Health check endpoint (public, no auth required).
	// Phase 9 (NFR-OBS02): when a HealthChecker is provided, the response
	// includes sync_lag / reorg_count / evaluator_state / keeper_last_run.
	// Without one, it falls back to the Phase 7 simple {status: "ok"} body
	// so the Docker HEALTHCHECK still passes during early boot.
	r.GET("/health", func(c *gin.Context) {
		if health == nil {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
			return
		}
		syncLag, reorgCount, keeperLastRun, evaluatorState := health.HealthSnapshot()
		status := "ok"
		// Degraded if any sub-system is stopped or sync lag is large.
		if evaluatorState == "stopped" || syncLag > 64 {
			status = "degraded"
		}
		c.JSON(http.StatusOK, gin.H{
			"status":          status,
			"sync_lag":        syncLag,
			"reorg_count":     reorgCount,
			"evaluator_state": evaluatorState,
			"keeper_last_run": keeperLastRun,
		})
	})

	// API v1 group: protected by authentication and timeout middleware
	apiV1 := r.Group("/api/v1",
		middleware.AuthMiddleware(),    // API token authentication
		middleware.TimeoutMiddleware(), // Request timeout control
	)

	// Register all modular routes (e.g., ERC20, FundMe) into the v1 group
	for _, route := range routables {
		route.RegisterRoutes(apiV1)
	}
	return r
}
