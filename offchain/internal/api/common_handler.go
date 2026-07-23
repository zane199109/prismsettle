package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zane/web3-offchain/pkg/response"
)

type CommonHandler struct{}

func NewCommonHandler() *CommonHandler {
	return &CommonHandler{}
}

// RegisterRoutes registers global common routes
func (h *CommonHandler) RegisterRoutes(v1 *gin.RouterGroup) {
	// Health check registered directly on root route, not under /v1
	v1.GET("/health", h.Health)
	v1.GET("/ping", h.Ping)
}

// Health service health check (production standard)
func (h *CommonHandler) Health(c *gin.Context) {
	response.Success(c, gin.H{
		"status":    "ok",
		"service":   "web3-offchain",
		"timestamp": time.Now().Unix(),
	})
}

// Ping liveness check
func (h *CommonHandler) Ping(c *gin.Context) {
	response.Success(c, gin.H{
		"message": "pong",
	})
}
