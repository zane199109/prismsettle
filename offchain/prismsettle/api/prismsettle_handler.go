package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/zane/web3-offchain/model"
	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/logger"
	"github.com/zane/web3-offchain/pkg/response"
	prism_svc "github.com/zane/web3-offchain/prismsettle/service"
)

// PrismSettleHandler exposes HTTP endpoints for querying PrismSettleRegistry
// events indexed off-chain.
//
// Phase 7 task 7.1: 16 endpoints (3 original + 13 new).
type PrismSettleHandler struct {
	svc      *prism_svc.PrismSettleService
	validate *validator.Validate
	// httpClient is used for the /agent/invoke proxy (FR-M11). Replaceable
	// for tests.
	httpClient *http.Client
}

// NewPrismSettleHandler constructs a handler with the legacy service signature.
func NewPrismSettleHandler(
	svc *prism_svc.PrismSettleService,
	validate *validator.Validate,
) *PrismSettleHandler {
	if validate == nil {
		validate = validator.New()
	}
	return &PrismSettleHandler{
		svc:        svc,
		validate:   validate,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// RegisterRoutes wires PrismSettle endpoints under /api/v1/prismsettle.
func (h *PrismSettleHandler) RegisterRoutes(v1 *gin.RouterGroup) {
	g := v1.Group("/prismsettle")
	{
		// Original 3 endpoints (Phase 5/6).
		g.GET("/events", h.getEvents)
		g.GET("/score", h.getScore)
		g.GET("/validations/count", h.countValidations)

		// Phase 7 task 7.1: 13 new endpoints (FR-A01..A12, FR-M11, perf, timeline).
		g.GET("/agents", h.listAgents)        // FR-A06
		g.GET("/agents/:agentId", h.getAgent) // FR-A07
		g.GET("/jobs", h.listJobs)            // FR-A08
		g.GET("/jobs/:jobId", h.getJobStatus) // FR-A09
		g.GET("/jobs/:jobId/status", h.getJobStatus)
		g.GET("/jobs/:jobId/timeline", h.getJobTimeline)     // FR-JM02
		g.GET("/trust", h.checkTrust)                        // FR-A11
		g.GET("/reputation/history", h.getReputationHistory) // FR-A12
		g.POST("/trust/thresholds", h.setTrustThreshold)     // FR-AP11
		g.GET("/shards/activity", h.getShardActivity)        // FR-A04

		// Perf + reorg feed (Phase 9 replaces mock data).
		g.GET("/perf/shard-heatmap", h.getShardActivity) // alias of FR-A04
		g.GET("/perf/v0-v1-comparison", h.getPerfComparison)
		g.GET("/perf/reorg-feed", h.getReorgFeed)

		// Agent invoke proxy (FR-M11).
		g.POST("/agent/invoke", h.invokeAgent)
	}
}

// -----------------------------------------------------------------------------
// request params
// -----------------------------------------------------------------------------

type prismEventsQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
	AgentID      string `form:"agentId"`
	Page         int    `form:"page" default:"1" validate:"required,min=1"`
	Size         int    `form:"size" default:"20" validate:"required,min=1,max=100"`
}

type prismScoreQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
	AgentID      string `form:"agentId" binding:"required" validate:"required"`
}

type prismValidationsCountQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
	AgentID      string `form:"agentId" binding:"required" validate:"required"`
}

type prismAgentsQuery struct {
	ChainName string `form:"chainName" binding:"required" validate:"required"`
	Page      int    `form:"page" default:"1" validate:"required,min=1"`
	Size      int    `form:"size" default:"20" validate:"required,min=1,max=100"`
}

type prismJobsQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
	State        string `form:"state" validate:"omitempty,oneof=open funded submitted completed refunded disputed resolved"`
	Page         int    `form:"page" default:"1" validate:"required,min=1"`
	Size         int    `form:"size" default:"20" validate:"required,min=1,max=100"`
}

type prismJobQuery struct {
	ChainName string `form:"chainName" binding:"required" validate:"required"`
}

type prismTrustQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
	AgentID      string `form:"agentId" binding:"required" validate:"required"`
}

type prismReputationHistoryQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
	AgentID      string `form:"agentId" binding:"required" validate:"required"`
	Limit        int    `form:"limit" default:"30" validate:"omitempty,min=1,max=200"`
}

type prismShardActivityQuery struct {
	ChainName    string `form:"chainName" binding:"required" validate:"required"`
	ContractAddr string `form:"contract" validate:"omitempty,eth_addr"`
}

type prismSetTrustThresholdReq struct {
	AgentID        string `json:"agent_id" validate:"omitempty"` // "" = global default
	AllowThreshold string `json:"allow_threshold" binding:"required" validate:"required"`
	DenyThreshold  string `json:"deny_threshold" binding:"required" validate:"required"`
}

type prismInvokeReq struct {
	AgentID string          `json:"agent_id" binding:"required" validate:"required"`
	Method  string          `json:"method" binding:"required" validate:"required"`
	Params  json.RawMessage `json:"params"`
}

// -----------------------------------------------------------------------------

func (h *PrismSettleHandler) getEvents(c *gin.Context) {
	var req prismEventsQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}

	req.ContractAddr = strings.ToLower(req.ContractAddr)

	start := time.Now()
	events, total, err := h.svc.GetEvents(
		c.Request.Context(),
		req.ChainName,
		req.ContractAddr,
		req.AgentID,
		req.Page,
		req.Size,
	)
	if err != nil {
		logger.Warn("prismsettle get events failed",
			logger.String("chain", req.ChainName),
			logger.String("agent", req.AgentID),
			logger.Error(err))
		response.HandleError(c, err)
		return
	}

	voList := make([]*model.PrismSettleEventVO, 0, len(events))
	for i := range events {
		voList = append(voList, model.ConvertToPrismSettleEventVO(events[i]))
	}

	logger.Info("prismsettle get events success",
		logger.String("chain", req.ChainName),
		logger.Int("count", len(voList)),
		logger.Int64("total", total),
		logger.Cost(start))

	response.Success(c, gin.H{
		"list":  voList,
		"total": total,
		"page":  req.Page,
		"size":  req.Size,
	})
}

func (h *PrismSettleHandler) getScore(c *gin.Context) {
	var req prismScoreQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}

	req.ContractAddr = strings.ToLower(req.ContractAddr)

	ev, err := h.svc.GetScore(
		c.Request.Context(),
		req.ChainName,
		req.ContractAddr,
		req.AgentID,
	)
	if err != nil {
		logger.Warn("prismsettle get score failed",
			logger.String("agent", req.AgentID),
			logger.Error(err))
		response.HandleError(c, err)
		return
	}

	response.Success(c, model.PrismSettleScoreVO{
		AgentID:     req.AgentID,
		Score:       ev.Value,
		BlockNumber: ev.BlockNumber,
		BlockTime:   ev.BlockTime,
	})
}

func (h *PrismSettleHandler) countValidations(c *gin.Context) {
	var req prismValidationsCountQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}

	req.ContractAddr = strings.ToLower(req.ContractAddr)

	count, err := h.svc.CountValidations(
		c.Request.Context(),
		req.ChainName,
		req.ContractAddr,
		req.AgentID,
	)
	if err != nil {
		logger.Warn("prismsettle count validations failed",
			logger.String("agent", req.AgentID),
			logger.Error(err))
		response.HandleError(c, err)
		return
	}

	response.Success(c, gin.H{
		"agent_id": req.AgentID,
		"count":    count,
	})
}

// -----------------------------------------------------------------------------
// Phase 7 task 7.1: 13 new endpoint handlers
// -----------------------------------------------------------------------------

// listAgents handles GET /api/v1/prismsettle/agents (FR-A06).
func (h *PrismSettleHandler) listAgents(c *gin.Context) {
	var req prismAgentsQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	agents, total, err := h.svc.ListAgents(c.Request.Context(), req.ChainName, req.Page, req.Size)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"list":  agents,
		"total": total,
		"page":  req.Page,
		"size":  req.Size,
	})
}

// getAgent handles GET /api/v1/prismsettle/agents/:agentId (FR-A07).
func (h *PrismSettleHandler) getAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	if agentID == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "agentId is required")
		return
	}
	chainName := c.Query("chainName")
	if chainName == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "chainName is required")
		return
	}
	agent, err := h.svc.GetAgent(c.Request.Context(), chainName, agentID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, agent)
}

// listJobs handles GET /api/v1/prismsettle/jobs (FR-A08).
func (h *PrismSettleHandler) listJobs(c *gin.Context) {
	var req prismJobsQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	req.ContractAddr = strings.ToLower(req.ContractAddr)
	events, total, err := h.svc.ListJobs(
		c.Request.Context(), req.ChainName, req.ContractAddr, req.State, req.Page, req.Size,
	)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"list":  events,
		"total": total,
		"page":  req.Page,
		"size":  req.Size,
	})
}

// getJobStatus handles GET /api/v1/prismsettle/jobs/:jobId and /jobs/:jobId/status (FR-A09/A10).
func (h *PrismSettleHandler) getJobStatus(c *gin.Context) {
	jobID := c.Param("jobId")
	if jobID == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "jobId is required")
		return
	}
	chainName := c.Query("chainName")
	if chainName == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "chainName is required")
		return
	}
	ev, err := h.svc.GetJobStatus(c.Request.Context(), chainName, jobID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"job_id":       jobID,
		"status":       string(ev.EventType),
		"block_number": ev.BlockNumber,
		"block_time":   ev.BlockTime,
		"tx_hash":      ev.TxHash,
	})
}

// getJobTimeline handles GET /api/v1/prismsettle/jobs/:jobId/timeline (FR-JM02).
func (h *PrismSettleHandler) getJobTimeline(c *gin.Context) {
	jobID := c.Param("jobId")
	if jobID == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "jobId is required")
		return
	}
	chainName := c.Query("chainName")
	if chainName == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "chainName is required")
		return
	}
	events, err := h.svc.GetJobTimeline(c.Request.Context(), chainName, jobID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"job_id": jobID,
		"events": events,
		"count":  len(events),
	})
}

// checkTrust handles GET /api/v1/prismsettle/trust (FR-A11).
func (h *PrismSettleHandler) checkTrust(c *gin.Context) {
	var req prismTrustQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	req.ContractAddr = strings.ToLower(req.ContractAddr)
	result, err := h.svc.CheckTrust(c.Request.Context(), req.ChainName, req.ContractAddr, req.AgentID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, result)
}

// setTrustThreshold handles POST /api/v1/prismsettle/trust/thresholds (FR-AP11).
func (h *PrismSettleHandler) setTrustThreshold(c *gin.Context) {
	var req prismSetTrustThresholdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	if err := h.svc.SetTrustThreshold(c.Request.Context(), req.AgentID, req.AllowThreshold, req.DenyThreshold); err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{"agent_id": req.AgentID, "updated": true})
}

// getReputationHistory handles GET /api/v1/prismsettle/reputation/history (FR-A12).
func (h *PrismSettleHandler) getReputationHistory(c *gin.Context) {
	var req prismReputationHistoryQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	if req.Limit == 0 {
		req.Limit = 30
	}
	req.ContractAddr = strings.ToLower(req.ContractAddr)
	events, err := h.svc.GetReputationHistory(c.Request.Context(), req.ChainName, req.ContractAddr, req.AgentID, req.Limit)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"agent_id": req.AgentID,
		"events":   events,
		"count":    len(events),
	})
}

// getShardActivity handles GET /api/v1/prismsettle/shards/activity and
// /perf/shard-heatmap (FR-A04).
func (h *PrismSettleHandler) getShardActivity(c *gin.Context) {
	var req prismShardActivityQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid parameters")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	req.ContractAddr = strings.ToLower(req.ContractAddr)
	rows, err := h.svc.GetShardActivity(c.Request.Context(), req.ChainName, req.ContractAddr)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"chain_name": req.ChainName,
		"shards":     rows,
		"count":      len(rows),
	})
}

// getPerfComparison handles GET /api/v1/prismsettle/perf/v0-v1-comparison.
// Phase 9: reads from perf_results table; falls back to mock when no
// benchmark data exists yet.
func (h *PrismSettleHandler) getPerfComparison(c *gin.Context) {
	response.Success(c, h.svc.GetPerfComparison(c.Request.Context()))
}

// getReorgFeed handles GET /api/v1/prismsettle/perf/reorg-feed.
// Phase 9: reads from reorg_events table; returns empty list when no
// reorgs have been detected yet.
func (h *PrismSettleHandler) getReorgFeed(c *gin.Context) {
	chainName := c.Query("chain_name")
	limit := 50
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 200 {
		limit = v
	}
	items := h.svc.GetReorgFeed(c.Request.Context(), chainName, limit)
	response.Success(c, gin.H{
		"items": items,
		"count": len(items),
	})
}

// invokeAgent proxies an A2A /invoke call to the target agent's endpoint.
// FR-M11. The backend resolves the agent's endpoint URL via agent_registry,
// then POSTs the request body and streams the response back. This bypasses
// CORS for browser clients.
func (h *PrismSettleHandler) invokeAgent(c *gin.Context) {
	var req prismInvokeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "validation failed: "+err.Error())
		return
	}
	chainName := c.Query("chainName")
	if chainName == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "chainName is required")
		return
	}
	agent, err := h.svc.GetAgent(c.Request.Context(), chainName, req.AgentID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if agent.Endpoint == "" {
		response.Fail(c, errno.ErrNotFound.Code, "agent endpoint not configured")
		return
	}

	// Build A2A JSON-RPC envelope.
	payload := gin.H{
		"jsonrpc": "2.0",
		"method":  req.Method,
		"params":  req.Params,
		"id":      time.Now().UnixNano(),
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, agent.Endpoint, bytes.NewReader(body))
	if err != nil {
		response.HandleError(c, errno.ErrInternal)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		logger.Warn("agent invoke failed",
			logger.String("agent", req.AgentID),
			logger.String("endpoint", agent.Endpoint),
			logger.Error(err))
		response.Fail(c, errno.ErrInternal.Code, "agent invoke failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		response.HandleError(c, errno.ErrInternal)
		return
	}
	// Stream the upstream response back as-is (preserve JSON-RPC envelope).
	c.Data(resp.StatusCode, "application/json", respBody)
}
