package api

import (
	"math/big"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/zane/web3-offchain/pkg/errno"
	"github.com/zane/web3-offchain/pkg/response"
	"github.com/zane/web3-offchain/prismsettle/demo"
)

// DemoHandler exposes the chat-style collaboration demo endpoints.
type DemoHandler struct {
	orch *demo.Orchestrator
}

func NewDemoHandler(orch *demo.Orchestrator) *DemoHandler {
	return &DemoHandler{orch: orch}
}

// RegisterRoutes wires /prismsettle/demo/* under the API v1 group.
func (h *DemoHandler) RegisterRoutes(g *gin.RouterGroup) {
	pg := g.Group("/prismsettle")
	pg.POST("/demo/sessions", h.createSession)
	pg.GET("/demo/sessions", h.listByJob)
	pg.GET("/demo/sessions/:id", h.getSession)
	pg.GET("/demo/sessions/:id/messages", h.listMessages)
}

// listByJob returns the most recent demo session for a job (history view).
func (h *DemoHandler) listByJob(c *gin.Context) {
	jobID := normalizeJobID(c.Query("job_id"))
	if jobID == "" {
		response.Fail(c, errno.ErrInvalidParam.Code, "missing job_id")
		return
	}
	sess, err := h.orch.GetByJobID(c.Request.Context(), jobID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	if sess == nil {
		response.Success(c, gin.H{"session": nil})
		return
	}
	response.Success(c, gin.H{"session": gin.H{
		"session_id":     sess.ID,
		"job_id":         sess.JobID,
		"title":          sess.Title,
		"description":    sess.Description,
		"amount":         sess.Amount,
		"token":          sess.Token,
		"provider_agent": sess.ProviderAgent,
		"scenario":       sess.Scenario,
		"state":          sess.State,
		"result":         sess.Result,
		"created_at":     sess.CreatedAt.Unix(),
	}})
}

type createSessionReq struct {
	Title         string `json:"title" binding:"required"`
	Description   string `json:"description"`
	Amount        string `json:"amount" binding:"required"` // token units (6 decimals USDC)
	Token         string `json:"token"`
	ProviderAgent string `json:"provider_agent"` // senior/junior/rookie
	Scenario      string `json:"scenario"`       // arbitration (default) | direct
	MaxRejects    int    `json:"max_rejects"`
	JobID         string `json:"job_id"` // resume an existing job instead of creating one
}

func (h *DemoHandler) createSession(c *gin.Context) {
	var req createSessionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, errno.ErrInvalidParam.Code, "invalid body: "+err.Error())
		return
	}
	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		response.Fail(c, errno.ErrInvalidParam.Code, "amount must be a positive integer")
		return
	}
	if req.Token == "" {
		req.Token = strings.ToLower(req.Token)
	}
	sess, err := h.orch.Start(c.Request.Context(), demo.SessionParams{
		Title:         req.Title,
		Description:   req.Description,
		Amount:        amount,
		Token:         req.Token,
		ProviderAgent: req.ProviderAgent,
		Scenario:      req.Scenario,
		MaxRejects:    req.MaxRejects,
		JobID:         req.JobID,
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"session_id": sess.ID,
		"state":      sess.State,
		"title":      sess.Title,
	})
}

func (h *DemoHandler) getSession(c *gin.Context) {
	sess, err := h.orch.GetSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.Success(c, gin.H{
		"session_id":     sess.ID,
		"job_id":         sess.JobID,
		"title":          sess.Title,
		"description":    sess.Description,
		"amount":         sess.Amount,
		"token":          sess.Token,
		"provider_agent": sess.ProviderAgent,
		"state":          sess.State,
		"result":         sess.Result,
		"created_at":     sess.CreatedAt.Unix(),
	})
}

func (h *DemoHandler) listMessages(c *gin.Context) {
	msgs, err := h.orch.ListMessages(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.HandleError(c, err)
		return
	}
	out := make([]gin.H, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, gin.H{
			"id":         m.ID,
			"step":       m.Step,
			"role":       m.Role,
			"content":    m.Content,
			"report":     m.Report,
			"deliverable_hash": m.DeliverableHash,
			"action":     m.Action,
			"tx_hash":    m.TxHash,
			"state":      m.State,
			"created_at": m.CreatedAt.Unix(),
		})
	}
	response.Success(c, gin.H{
		"session_id": c.Param("id"),
		"count":      len(out),
		"messages":   out,
	})
}
